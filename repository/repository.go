package repository

import (
	"context"
	"database/sql"
)

// Repository is a generic repository interface for CRUD operations.
// Uses Go generics for type safety.
// TEntity is the entity type.
// TID is the ID type.
// All methods accept context for cancellation and timeout.
// Entity pointer is used to allow nil returns and modification.
type Repository[TEntity any, TID comparable] interface {
	// Create inserts a new entity
	Create(ctx context.Context, entity *TEntity) error

	// GetByID retrieves an entity by its ID
	GetByID(ctx context.Context, id TID) (*TEntity, error)

	// Update updates an existing entity
	Update(ctx context.Context, id TID, entity *TEntity) error

	// Delete removes an entity by its ID
	Delete(ctx context.Context, id TID) error

	// List retrieves entities with filtering and pagination, and returns total count.
	// Runs both list and count.
	List(ctx context.Context, opts *ListOptions) ([]*TEntity, int64, error)

	// Count returns the total number of entities matching the filter (for use when only total is needed).
	Count(ctx context.Context, filter Filter) (int64, error)

	// Exists checks if an entity with given ID exists
	Exists(ctx context.Context, id TID) (bool, error)
}

// ReadRepository is a read-only repository interface.
// Use case: When repository should only allow reads,
// for follower-only database access, or
// Read-only microservices.
type ReadRepository[TEntity any, TID comparable] interface {
	GetByID(ctx context.Context, id TID) (*TEntity, error)
	List(ctx context.Context, opts *ListOptions) ([]*TEntity, int64, error)
	Count(ctx context.Context, filter Filter) (int64, error)
	Exists(ctx context.Context, id TID) (bool, error)
}

// WriteRepository is a write-only repository interface.
// Use case: Command-side in CQRS architecture,
// write-only microservices, or
// event sourcing systems.
type WriteRepository[TEntity any, TID comparable] interface {
	Create(ctx context.Context, entity *TEntity) error
	Update(ctx context.Context, id TID, entity *TEntity) error
	Delete(ctx context.Context, id TID) error
}

// TransactionalRepository is a repository with transaction support.
// Use case: When repository needs to participate in external transaction.
// Integration with sqlc (which has WithTx method).
type TransactionalRepository[TEntity any, TID comparable] interface {
	Repository[TEntity, TID]

	// WithTx returns a new repository instance bound to the transaction.
	// Use this when you need the repository to participate in an existing transaction.
	WithTx(tx *sql.Tx) Repository[TEntity, TID]
}
