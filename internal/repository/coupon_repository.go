package repository

import (
	"context"
	"coupon-system/internal/model"
)

// CouponRepository defines the interface for coupon data operations
// All methods accept a context which can be a mongo.SessionContext when used in transactions
type CouponRepository interface {
	// CreateCoupon creates a new coupon
	CreateCoupon(ctx context.Context, coupon *model.Coupon) error

	// GetCouponByName retrieves a coupon by its name
	GetCouponByName(ctx context.Context, name string) (*model.Coupon, error)

	// DecrementStock atomically decrements the remaining stock of a coupon
	// Returns error if stock is exhausted or coupon not found
	// The context can be a mongo.SessionContext when used in transactions
	DecrementStock(ctx context.Context, couponID interface{}, amount int32) error
}

