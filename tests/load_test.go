package service

import (
	"bytes"
	"context"
	"coupon-system/internal/model"
	"coupon-system/pkg/config"
	"coupon-system/pkg/database"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	testMongoURI = config.GetEnv("MONGO_URI", "mongodb://localhost:27017")
	testDBName   = config.GetEnv("MONGO_DB", "coupon_system")
	baseURL      = config.GetEnv("BASE_URL", "http://localhost:8080")
)

// TestResult tracks the result of a claim request
type TestResult struct {
	StatusCode int
	Success    bool
	Error      string
}

// setupTestDatabase cleans and seeds the test database
func setupTestDatabase(t *testing.T) func() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MongoDB
	mongoDB, err := database.Connect(ctx, testMongoURI, testDBName)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Clean database - drop all collections
	collections := []string{"coupons", "claims"}
	for _, collName := range collections {
		collection := mongoDB.Database.Collection(collName)
		if err := collection.Drop(ctx); err != nil {
			t.Logf("Warning: Failed to drop collection %s: %v", collName, err)
		}
	}

	// Recreate indexes
	if err := mongoDB.CreateIndexes(ctx); err != nil {
		t.Fatalf("Failed to create indexes: %v", err)
	}

	// Seed test data
	couponsCollection := mongoDB.Database.Collection("coupons")

	// Flash Sale coupon - 5 items in stock
	flashSaleCoupon := &model.Coupon{
		ID:              primitive.NewObjectID(),
		Name:            "FLASH_SALE_2026",
		Amount:          500,
		RemainingAmount: 5,
		IsActive:        true,
		CreatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(24 * time.Hour),
		UpdatedAt:       time.Now(),
	}
	_, err = couponsCollection.InsertOne(ctx, flashSaleCoupon)
	if err != nil {
		t.Fatalf("Failed to seed flash sale coupon: %v", err)
	}

	// Promo coupon - for double dip test (plenty of stock)
	promoCoupon := &model.Coupon{
		ID:              primitive.NewObjectID(),
		Name:            "PROMO_SUPER",
		Amount:          10000,
		RemainingAmount: 100,
		IsActive:        true,
		CreatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(24 * time.Hour),
		UpdatedAt:       time.Now(),
	}
	_, err = couponsCollection.InsertOne(ctx, promoCoupon)
	if err != nil {
		t.Fatalf("Failed to seed promo coupon: %v", err)
	}

	t.Logf("✅ Database cleaned and seeded successfully")
	
	// Return cleanup function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mongoDB.Disconnect(ctx)
	}
}

// claimCoupon makes a claim request to the API
func claimCoupon(baseURL, userID, couponName string) TestResult {
	reqBody := model.ClaimCouponRequest{
		UserID:     userID,
		CouponName: couponName,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return TestResult{
			StatusCode: 0,
			Success:    false,
			Error:      fmt.Sprintf("Failed to marshal request: %v", err),
		}
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/api/coupons/claim", baseURL),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return TestResult{
			StatusCode: 0,
			Success:    false,
			Error:      fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	var errorMsg string
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			errorMsg = errorResp["error"]
		}
	}

	return TestResult{
		StatusCode: resp.StatusCode,
		Success:    resp.StatusCode == http.StatusOK,
		Error:      errorMsg,
	}
}

// getCouponDetails retrieves coupon details from the API
func getCouponDetails(baseURL, couponName string) (*model.CouponDetailsResponse, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/coupons/%s", baseURL, couponName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var details model.CouponDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}

	return &details, nil
}

// waitForServer waits for the server to be ready
func waitForServer(baseURL string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("%s/health", baseURL))
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server not ready after %v", maxWait)
}

// TestFlashSaleAttack tests the Flash Sale attack scenario
// 50 concurrent requests for a coupon with only 5 items in stock
// Expected: Exactly 5 successful claims, 0 remaining stock
func TestFlashSaleAttack(t *testing.T) {
	// Wait for server to be ready
	if err := waitForServer(baseURL, 10*time.Second); err != nil {
		t.Fatalf("Server is not ready: %v. Make sure the server is running on %s", err, baseURL)
	}

	// Setup test database
	cleanup := setupTestDatabase(t)
	defer cleanup()

	couponName := "FLASH_SALE_2026"
	concurrentRequests := 50
	expectedSuccess := 5
	expectedNoStock := 45

	// Track results
	var (
		successCount int64
		noStockCount int64
		otherErrors  int64
		mu           sync.Mutex
		wg           sync.WaitGroup
		results      []TestResult
	)

	t.Logf("Starting Flash Sale Attack Test")
	t.Logf("   Coupon: %s", couponName)
	t.Logf("   Concurrent Requests: %d", concurrentRequests)
	t.Logf("   Expected Success: %d", expectedSuccess)
	t.Logf("   Expected No Stock: %d", expectedNoStock)

	// Make concurrent requests
	start := time.Now()
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			result := claimCoupon(baseURL, fmt.Sprintf("user_%d", userID), couponName)

			mu.Lock()
			results = append(results, result)
			switch result.StatusCode {
			case http.StatusOK:
				atomic.AddInt64(&successCount, 1)
			case http.StatusBadRequest:
				if result.Error == "no stock available" {
					atomic.AddInt64(&noStockCount, 1)
				} else {
					atomic.AddInt64(&otherErrors, 1)
				}
			default:
				atomic.AddInt64(&otherErrors, 1)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	_ = time.Since(start) // duration tracked but not logged

	// Wait a bit for all operations to complete
	time.Sleep(500 * time.Millisecond)

	// Verify final state
	details, err := getCouponDetails(baseURL, couponName)
	if err != nil {
		t.Fatalf("Failed to get coupon details: %v", err)
	}

	// Assertions
	if successCount != int64(expectedSuccess) {
		t.Errorf("❌ FAILED: Expected %d successful claims, got %d", expectedSuccess, successCount)
	} else {
		t.Logf("✅ PASSED: Success count is correct (%d)", successCount)
	}

	if noStockCount != int64(expectedNoStock) {
		t.Errorf("❌ FAILED: Expected %d no stock errors, got %d", expectedNoStock, noStockCount)
	} else {
		t.Logf("✅ PASSED: No stock error count is correct (%d)", noStockCount)
	}

	if otherErrors != 0 {
		t.Errorf("❌ FAILED: Expected 0 other errors, got %d", otherErrors)
		for _, result := range results {
			if result.StatusCode != http.StatusOK && result.StatusCode != http.StatusBadRequest {
				t.Logf("   Unexpected error: Status %d, Error: %s", result.StatusCode, result.Error)
			}
		}
	} else {
		t.Logf("✅ PASSED: No unexpected errors")
	}

	if details.RemainingAmount != 0 {
		t.Errorf("❌ FAILED: Expected remaining stock to be 0, got %d", details.RemainingAmount)
	} else {
		t.Logf("✅ PASSED: Remaining stock is 0")
	}

	if len(details.ClaimedBy) != expectedSuccess {
		t.Errorf("❌ FAILED: Expected %d claims in database, got %d", expectedSuccess, len(details.ClaimedBy))
	} else {
		t.Logf("✅ PASSED: Claim count in database is correct (%d)", len(details.ClaimedBy))
	}
}

// TestDoubleDipAttack tests the Double Dip attack scenario
// 10 concurrent requests from the SAME user for the same coupon
// Expected: Exactly 1 successful claim, 9 failures (409 Conflict)
func TestDoubleDipAttack(t *testing.T) {
	// Wait for server to be ready
	if err := waitForServer(baseURL, 10*time.Second); err != nil {
		t.Fatalf("Server is not ready: %v. Make sure the server is running on %s", err, baseURL)
	}

	// Setup test database
	cleanup := setupTestDatabase(t)
	defer cleanup()

	couponName := "PROMO_SUPER"
	userID := "double_dip_user_123"
	concurrentRequests := 10
	expectedSuccess := 1
	expectedConflicts := 9

	// Track results
	var (
		successCount int64
		conflictCount int64
		otherErrors  int64
		mu           sync.Mutex
		wg           sync.WaitGroup
		results      []TestResult
	)

	t.Logf("Starting Double Dip Attack Test")
	t.Logf("   Coupon: %s", couponName)
	t.Logf("   User ID: %s", userID)
	t.Logf("   Concurrent Requests: %d", concurrentRequests)
	t.Logf("   Expected Success: %d", expectedSuccess)
	t.Logf("   Expected Conflicts: %d", expectedConflicts)

	// Make concurrent requests from the same user
	start := time.Now()
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			result := claimCoupon(baseURL, userID, couponName)

			mu.Lock()
			results = append(results, result)
			switch result.StatusCode {
			case http.StatusOK:
				atomic.AddInt64(&successCount, 1)
			case http.StatusConflict:
				atomic.AddInt64(&conflictCount, 1)
			default:
				atomic.AddInt64(&otherErrors, 1)
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	_ = time.Since(start) // duration tracked but not logged

	// Wait a bit for all operations to complete
	time.Sleep(500 * time.Millisecond)

	// Verify final state
	details, err := getCouponDetails(baseURL, couponName)
	if err != nil {
		t.Fatalf("Failed to get coupon details: %v", err)
	}

	// Count how many times the user appears in claimed_by
	userClaimCount := 0
	for _, claimedUser := range details.ClaimedBy {
		if claimedUser == userID {
			userClaimCount++
		}
	}

	// Assertions
	if successCount != int64(expectedSuccess) {
		t.Errorf("❌ FAILED: Expected %d successful claim, got %d", expectedSuccess, successCount)
	} else {
		t.Logf("✅ PASSED: Success count is correct (%d)", successCount)
	}

	if conflictCount != int64(expectedConflicts) {
		t.Errorf("❌ FAILED: Expected %d conflict errors, got %d", expectedConflicts, conflictCount)
	} else {
		t.Logf("✅ PASSED: Conflict error count is correct (%d)", conflictCount)
	}

	if otherErrors != 0 {
		t.Errorf("❌ FAILED: Expected 0 other errors, got %d", otherErrors)
		for _, result := range results {
			if result.StatusCode != http.StatusOK && result.StatusCode != http.StatusConflict {
				t.Logf("   Unexpected error: Status %d, Error: %s", result.StatusCode, result.Error)
			}
		}
	} else {
		t.Logf("✅ PASSED: No unexpected errors")
	}

	if userClaimCount != expectedSuccess {
		t.Errorf("❌ FAILED: Expected user to appear %d time(s) in claimed_by, got %d", expectedSuccess, userClaimCount)
	} else {
		t.Logf("✅ PASSED: User appears exactly once in database")
	}
}

