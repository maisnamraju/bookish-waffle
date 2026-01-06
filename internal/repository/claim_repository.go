package repository

import (
	"context"
	"coupon-system/internal/model"
)

// ClaimRepository defines the interface for claim data operations
// All methods accept a context which can be a mongo.SessionContext when used in transactions
type ClaimRepository interface {
	// CreateClaim creates a new claim record
	CreateClaim(ctx context.Context, claim *model.Claim) error

	// CreateClaimIfNotExists atomically creates a claim only if it doesn't exist
	// Uses MongoDB upsert with $setOnInsert for atomic idempotent operation
	// Returns (true, nil) if created, (false, ErrAlreadyClaimed) if already exists
	CreateClaimIfNotExists(ctx context.Context, claim *model.Claim) (bool, error)

	// DeleteClaim removes a claim record (used for compensating transactions)
	DeleteClaim(ctx context.Context, userID string, couponID interface{}) error

	// GetClaimsByCouponName retrieves all claims for a specific coupon
	GetClaimsByCouponName(ctx context.Context, couponName string) ([]*model.Claim, error)

	// HasUserClaimed checks if a user has already claimed a specific coupon
	// The context can be a mongo.SessionContext when used in transactions
	HasUserClaimed(ctx context.Context, userID string, couponID interface{}) (bool, error)
}

