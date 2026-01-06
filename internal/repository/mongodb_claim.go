package repository

import (
	"context"
	"coupon-system/internal/model"
	apperrors "coupon-system/pkg/errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
			return apperrors.ErrAlreadyClaimed
		}
		return err
	}

	return nil
}

// CreateClaimIfNotExists atomically creates a claim only if it doesn't exist
// Uses MongoDB upsert with $setOnInsert for atomic idempotent operation
func (r *mongodbClaimRepository) CreateClaimIfNotExists(ctx context.Context, claim *model.Claim) (bool, error) {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"user_id":   claim.UserID,
			"coupon_id": claim.CouponID,
		},
		bson.M{
			"$setOnInsert": bson.M{
				"user_id":     claim.UserID,
				"coupon_id":   claim.CouponID,
				"coupon_name": claim.CouponName,
				"created_at":  claim.CreatedAt,
			},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return false, err
	}

	// UpsertedCount > 0 means a new document was created
	// If UpsertedCount == 0, the document already existed
	if result.UpsertedCount == 0 {
		return false, apperrors.ErrAlreadyClaimed
	}

	return true, nil
}

// DeleteClaim removes a claim record (used for compensating transactions)
func (r *mongodbClaimRepository) DeleteClaim(ctx context.Context, userID string, couponID interface{}) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{
		"user_id":   userID,
		"coupon_id": couponID,
	})
	return err
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

