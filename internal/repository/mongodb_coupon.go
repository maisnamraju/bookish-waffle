package repository

import (
	"context"
	"coupon-system/internal/model"
	"coupon-system/internal/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mongodbCouponRepository implements CouponRepository using MongoDB
type mongodbCouponRepository struct {
	collection *mongo.Collection
}

// NewCouponRepository creates a new MongoDB-based coupon repository
func NewCouponRepository(db *mongo.Database) CouponRepository {
	return &mongodbCouponRepository{
		collection: db.Collection("coupons"),
	}
}

// CreateCoupon creates a new coupon
func (r *mongodbCouponRepository) CreateCoupon(ctx context.Context, coupon *model.Coupon) error {
	_, err := r.collection.InsertOne(ctx, coupon)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return service.ErrCouponAlreadyExists
		}
		return err
	}

	return nil
}

// GetCouponByName retrieves a coupon by its name
func (r *mongodbCouponRepository) GetCouponByName(ctx context.Context, name string) (*model.Coupon, error) {
	var coupon model.Coupon
	err := r.collection.FindOne(ctx, bson.M{"name": name}).Decode(&coupon)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, service.ErrCouponNotFound
		}
		return nil, err
	}

	return &coupon, nil
}

// DecrementStock atomically decrements the remaining stock of a coupon
func (r *mongodbCouponRepository) DecrementStock(ctx context.Context, couponID interface{}, amount int32) error {
	updateResult := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":             couponID,
			"remaining_amount": bson.M{"$gte": amount}, // Only update if stock >= amount
		},
		bson.M{"$inc": bson.M{"remaining_amount": -amount}}, // Atomic decrement
		options.FindOneAndUpdate().
			SetReturnDocument(options.After).
			SetUpsert(false),
	)

	if updateResult.Err() != nil {
		if updateResult.Err() == mongo.ErrNoDocuments {
			return service.ErrNoStock
		}
		return updateResult.Err()
	}

	return nil
}

