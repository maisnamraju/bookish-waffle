package service

import (
	"context"
	"coupon-system/internal/model"
	"coupon-system/internal/repository"
	"errors"
	"time"
)

var (
	ErrCouponNotFound    = errors.New("coupon not found")
	ErrCouponAlreadyExists = errors.New("coupon already exists")
	ErrAlreadyClaimed    = errors.New("coupon already claimed by this user")
	ErrNoStock           = errors.New("no stock available")
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
func (s *CouponService) ClaimCoupon(ctx context.Context, req *model.ClaimCouponRequest) error {
	coupon, err := s.couponRepo.GetCouponByName(ctx, req.CouponName)
	if err != nil {
		return err
	}

	hasClaimed, err := s.claimRepo.HasUserClaimed(ctx, req.UserID, coupon.ID)
	if err != nil {
		return err
	}
	if hasClaimed {
		return ErrAlreadyClaimed
	}

	// Atomically decrement stock
	if err := s.couponRepo.DecrementStock(ctx, coupon.ID, 1); err != nil {
		return err
	}

	// Create claim record
	claim := &model.Claim{
		UserID:     req.UserID,
		CouponID:   coupon.ID,
		CouponName: req.CouponName,
		CreatedAt:  time.Now(),
	}

	if err := s.claimRepo.CreateClaim(ctx, claim); err != nil {
		return err
	}

	return nil
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

