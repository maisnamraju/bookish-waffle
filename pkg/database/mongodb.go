package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB wraps the MongoDB client and database
type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// Connect establishes a connection to MongoDB
func Connect(ctx context.Context, uri, dbName string) (*MongoDB, error) {
	clientOptions := options.Client().ApplyURI(uri)
	
	// Set connection timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(dbName)

	mongoDB := &MongoDB{
		Client:   client,
		Database: db,
	}

	// Create indexes
	if err := mongoDB.CreateIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return mongoDB, nil
}

// CreateIndexes creates all necessary indexes for the application
func (m *MongoDB) CreateIndexes(ctx context.Context) error {
	// Create unique index on coupons.name
	couponsCollection := m.Database.Collection("coupons")
	couponNameIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("coupon_name_unique"),
	}
	if _, err := couponsCollection.Indexes().CreateOne(ctx, couponNameIndex); err != nil {
		return fmt.Errorf("failed to create coupon name index: %w", err)
	}

	// Create unique compound index on claims(user_id, coupon_id)
	// This prevents double-dip attacks
	claimsCollection := m.Database.Collection("claims")
	userCouponIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "coupon_id", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetName("user_coupon_unique"),
	}
	if _, err := claimsCollection.Indexes().CreateOne(ctx, userCouponIndex); err != nil {
		return fmt.Errorf("failed to create user_coupon unique index: %w", err)
	}

	// Create index on coupon_id for faster lookups
	couponIDIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "coupon_id", Value: 1}},
		Options: options.Index().SetName("coupon_id_index"),
	}
	if _, err := claimsCollection.Indexes().CreateOne(ctx, couponIDIndex); err != nil {
		return fmt.Errorf("failed to create coupon_id index: %w", err)
	}

	// Create index on coupon_name for querying
	couponNameClaimIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "coupon_name", Value: 1}},
		Options: options.Index().SetName("coupon_name_index"),
	}
	if _, err := claimsCollection.Indexes().CreateOne(ctx, couponNameClaimIndex); err != nil {
		return fmt.Errorf("failed to create coupon_name index: %w", err)
	}

	return nil
}

// Disconnect closes the MongoDB connection
func (m *MongoDB) Disconnect(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}

