package errors

import "errors"

// Domain errors for the coupon system
var (
	ErrCouponNotFound      = errors.New("coupon not found")
	ErrCouponAlreadyExists = errors.New("coupon already exists")
	ErrAlreadyClaimed      = errors.New("coupon already claimed by this user")
	ErrNoStock             = errors.New("no stock available")
)
