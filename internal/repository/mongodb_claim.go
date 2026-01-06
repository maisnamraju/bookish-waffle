package repository

import (
	"context"
	"coupon-system/internal/model"
	"coupon-system/internal/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// mongodbClaimRepository implements ClaimRepository using MongoDB
type mongodbClaimRepository struct {
	collection *mongo.Collection
}

// NewClaimRepository creates a new MongoDB-based claim repository
func NewClaimRepository(db *mongo.Database) ClaimRepository {
	return &mongodbClaimRepository{
		collection: db.Collection("claims"),
	}
}

// CreateClaim creates a new claim record
func (r *mongodbClaimRepository) CreateClaim(ctx context.Context, claim *model.Claim) error {
	_, err := r.collection.InsertOne(ctx, claim)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return service.ErrAlreadyClaimed
		}
		return err
	}

	return nil
}

// GetClaimsByCouponName retrieves all claims for a specific coupon
func (r *mongodbClaimRepository) GetClaimsByCouponName(ctx context.Context, couponName string) ([]*model.Claim, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"coupon_name": couponName})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var claims []*model.Claim
	if err := cursor.All(ctx, &claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// HasUserClaimed checks if a user has already claimed a specific coupon
func (r *mongodbClaimRepository) HasUserClaimed(ctx context.Context, userID string, couponID interface{}) (bool, error) {
	err := r.collection.FindOne(ctx, bson.M{
		"user_id":   userID,
		"coupon_id": couponID,
	}).Err()

	if err == nil {
		return true, nil
	}
	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	return false, err
}

