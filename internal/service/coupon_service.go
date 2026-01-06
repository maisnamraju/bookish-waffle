package service

import (
	"context"
	"coupon-system/internal/model"
	"coupon-system/internal/repository"
	apperrors "coupon-system/pkg/errors"
	"time"
)

// Re-export errors for backward compatibility with handlers
var (
	ErrCouponNotFound      = apperrors.ErrCouponNotFound
	ErrCouponAlreadyExists = apperrors.ErrCouponAlreadyExists
	ErrAlreadyClaimed      = apperrors.ErrAlreadyClaimed
	ErrNoStock             = apperrors.ErrNoStock
)

// CouponService handles business logic for coupons
type CouponService struct {
	couponRepo repository.CouponRepository
	claimRepo  repository.ClaimRepository
}

// NewCouponService creates a new coupon service
func NewCouponService(couponRepo repository.CouponRepository, claimRepo repository.ClaimRepository) *CouponService {
	return &CouponService{
		couponRepo: couponRepo,
		claimRepo:  claimRepo,
	}
}

// ClaimCoupon attempts to claim a coupon for a user
// Uses atomic upsert pattern to prevent double-dip attacks without requiring transactions
func (s *CouponService) ClaimCoupon(ctx context.Context, req *model.ClaimCouponRequest) error {
	// Get coupon (read-only operation)
	coupon, err := s.couponRepo.GetCouponByName(ctx, req.CouponName)
	if err != nil {
		return err
	}

	// Step 1: Atomically claim FIRST using upsert pattern
	// This is idempotent - 10 concurrent requests result in exactly 1 insert
	// No race window exists because MongoDB's upsert is atomic
	claim := &model.Claim{
		UserID:     req.UserID,
		CouponID:   coupon.ID,
		CouponName: req.CouponName,
		CreatedAt:  time.Now(),
	}

	created, err := s.claimRepo.CreateClaimIfNotExists(ctx, claim)
	if err != nil {
		return err // Already claimed or DB error - no stock touched
	}
	if !created {
		return ErrAlreadyClaimed
	}

	// Step 2: Decrement stock (claim is now secured)
	// If this fails, we need to rollback the claim we just created
	if err := s.couponRepo.DecrementStock(ctx, coupon.ID, 1); err != nil {
		// Compensating action: remove the claim we just created
		_ = s.claimRepo.DeleteClaim(ctx, req.UserID, coupon.ID)
		return err
	}

	return nil
}

// CreateCoupon creates a new coupon
func (s *CouponService) CreateCoupon(ctx context.Context, req *model.CreateCouponRequest) (*model.Coupon, error) {
	// Parse expiration date if provided, otherwise default to 30 days
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err == nil {
			expiresAt = parsed
		}
	}

	coupon := &model.Coupon{
		Name:            req.Name,
		Amount:          req.Amount,
		RemainingAmount: req.Amount,
		IsActive:        true,
		CreatedAt:       time.Now(),
		ExpiresAt:       expiresAt,
		UpdatedAt:       time.Now(),
	}

	if err := s.couponRepo.CreateCoupon(ctx, coupon); err != nil {
		return nil, err
	}

	return coupon, nil
}

// GetCouponDetails retrieves coupon details including claim history
func (s *CouponService) GetCouponDetails(ctx context.Context, name string) (*model.CouponDetailsResponse, error) {
	coupon, err := s.couponRepo.GetCouponByName(ctx, name)
	if err != nil {
		return nil, ErrCouponNotFound
	}

	claims, err := s.claimRepo.GetClaimsByCouponName(ctx, name)
	if err != nil {
		return nil, err
	}

	claimedBy := make([]string, 0, len(claims))
	for _, claim := range claims {
		claimedBy = append(claimedBy, claim.UserID)
	}

	return &model.CouponDetailsResponse{
		Name:            coupon.Name,
		Amount:          coupon.Amount,
		RemainingAmount: coupon.RemainingAmount,
		ClaimedBy:       claimedBy,
	}, nil
}

