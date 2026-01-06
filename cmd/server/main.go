package main

import (
	"context"
	"coupon-system/internal/model"
	"coupon-system/internal/repository"
	"coupon-system/internal/service"
	"coupon-system/pkg/config"
	"coupon-system/pkg/database"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Get configuration from environment variables
	mongoURI := config.GetEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := config.GetEnv("MONGO_DB", "coupon_system")
	port := config.GetEnv("PORT", "8080")

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoDB, err := database.Connect(ctx, mongoURI, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := mongoDB.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	log.Println("✅ Connected to MongoDB successfully")

	// Initialize repositories
	couponRepo := repository.NewCouponRepository(mongoDB.Database)
	claimRepo := repository.NewClaimRepository(mongoDB.Database)

	// Initialize service (no transaction dependency - uses atomic upsert pattern)
	svc := service.NewCouponService(couponRepo, claimRepo)

	// Setup Gin router
	router := setupRouter(svc)

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupRouter(svc *service.CouponService) *gin.Engine {
	// Set Gin mode
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API routes
	api := router.Group("/api")
	{
		api.POST("/coupons", createCouponHandler(svc))
		api.POST("/coupons/claim", claimCouponHandler(svc))
		api.GET("/coupons/:name", getCouponDetailsHandler(svc))
	}

	return router
}

// createCouponHandler handles POST /coupons
func createCouponHandler(svc *service.CouponService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req model.CreateCouponRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		coupon, err := svc.CreateCoupon(c.Request.Context(), &req)
		if err != nil {
			switch err {
			case service.ErrCouponAlreadyExists:
				c.JSON(http.StatusConflict, gin.H{"error": "coupon already exists"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create coupon"})
			}
			return
		}

		c.JSON(http.StatusCreated, coupon)
	}
}

// claimCouponHandler handles POST /api/coupons/claim
// in real use cases, I would put the claim coupon behind a cache layer to prevent duplicate claims
func claimCouponHandler(svc *service.CouponService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req model.ClaimCouponRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		err := svc.ClaimCoupon(c.Request.Context(), &req)
		if err != nil {
			switch err {
			case service.ErrAlreadyClaimed:
				c.JSON(http.StatusConflict, gin.H{"error": "coupon already claimed by this user"})
			case service.ErrNoStock:
				c.JSON(http.StatusBadRequest, gin.H{"error": "no stock available"})
			case service.ErrCouponNotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "coupon not found"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to claim coupon"})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "coupon claimed successfully"})
	}
}

// getCouponDetailsHandler handles GET /api/coupons/:name
func getCouponDetailsHandler(svc *service.CouponService) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "coupon name is required"})
			return
		}

		details, err := svc.GetCouponDetails(c.Request.Context(), name)
		if err != nil {
			switch err {
			case service.ErrCouponNotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "coupon not found"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get coupon details"})
			}
			return
		}

		c.JSON(http.StatusOK, details)
	}
}

