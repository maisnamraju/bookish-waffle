package database

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// UnitOfWork manages MongoDB transactions
type UnitOfWork struct {
	client *mongo.Client
}

// NewUnitOfWork creates a new Unit of Work instance
func NewUnitOfWork(client *mongo.Client) *UnitOfWork {
	return &UnitOfWork{
		client: client,
	}
}

// WithTransaction executes a function within a MongoDB transaction
// If the function returns an error, the transaction is aborted
func (uow *UnitOfWork) WithTransaction(ctx context.Context, fn func(mongo.SessionContext) error) error {
	session, err := uow.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	// Use session.WithTransaction which handles transaction lifecycle automatically
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		// Execute the work function within the transaction
		return nil, fn(sc)
	})

	return err
}

