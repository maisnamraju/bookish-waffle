package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Coupon represents a coupon in the system
type Coupon struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name            string             `bson:"name" json:"name"`
	Amount          int32              `bson:"amount" json:"amount"`           // in cents
	RemainingAmount int32              `bson:"remaining_amount" json:"remaining_amount"` // in cents
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	ExpiresAt       time.Time          `bson:"expired_at" json:"expired_at"`
	IsActive        bool               `bson:"is_active" json:"is_active"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}

// Claim represents a coupon claim by a user
type Claim struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID     string             `bson:"user_id" json:"user_id"`
	CouponID   primitive.ObjectID `bson:"coupon_id" json:"coupon_id"`     // Used for unique index
	CouponName string             `bson:"coupon_name" json:"coupon_name"` // Denormalized for querying
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}

// ClaimCouponRequest represents the request to claim a coupon
type ClaimCouponRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	CouponName string `json:"coupon_name" binding:"required"`
}

// CouponDetailsResponse represents the response for coupon details
type CouponDetailsResponse struct {
	Name           string   `json:"name"`
	Amount         int32    `json:"amount"`          // in cents
	RemainingAmount int32   `json:"remaining_amount"` // in cents
	ClaimedBy      []string `json:"claimed_by"`
}

