package repository

import (
	"context"
	"coupon-system/internal/model"
)

// ClaimRepository defines the interface for claim data operations
type ClaimRepository interface {
	// CreateClaim creates a new claim record
	CreateClaim(ctx context.Context, claim *model.Claim) error

	// GetClaimsByCouponName retrieves all claims for a specific coupon
	GetClaimsByCouponName(ctx context.Context, couponName string) ([]*model.Claim, error)

	// HasUserClaimed checks if a user has already claimed a specific coupon
	HasUserClaimed(ctx context.Context, userID string, couponID interface{}) (bool, error)
}

