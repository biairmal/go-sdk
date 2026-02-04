package repository

import (
	"context"
	"database/sql"
)

// Repository is a generic repository interface for CRUD operations.
// Uses Go generics for type safety.
// ID is any to support different ID types (int64, string, UUID).
// All methods accept context for cancellation and timeout.
// Entity pointer is used to allow nil returns and modification.
type Repository[T any] interface {
	// Create inserts a new entity
	Create(ctx context.Context, entity *T) error

	// GetByID retrieves an entity by its ID
	GetByID(ctx context.Context, id any) (*T, error)

	// Update updates an existing entity
	Update(ctx context.Context, id any, entity *T) error

	// Delete removes an entity by its ID
	Delete(ctx context.Context, id any) error

	// List retrieves entities with filtering and pagination
	List(ctx context.Context, opts *ListOptions) ([]*T, error)

	// Count returns the total number of entities matching the filter
	Count(ctx context.Context, filter Filter) (int64, error)

	// Exists checks if an entity with given ID exists
	Exists(ctx context.Context, id any) (bool, error)
}

// ReadRepository is a read-only repository interface.
// Use case: When repository should only allow reads.
// For follower-only database access.
// Read-only microservices.
type ReadRepository[T any] interface {
	GetByID(ctx context.Context, id any) (*T, error)
	List(ctx context.Context, opts *ListOptions) ([]*T, error)
	Count(ctx context.Context, filter Filter) (int64, error)
	Exists(ctx context.Context, id any) (bool, error)
}

// WriteRepository is a write-only repository interface.
// Use case: Command-side in CQRS architecture.
// Write-only microservices.
// Event sourcing systems.
type WriteRepository[T any] interface {
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, id any, entity *T) error
	Delete(ctx context.Context, id any) error
}

// TransactionalRepository is a repository with transaction support.
// Use case: When repository needs to participate in external transaction.
// Integration with sqlc (which has WithTx method).
type TransactionalRepository[T any] interface {
	Repository[T]

	// WithTx returns a new repository instance bound to the transaction.
	// Use this when you need the repository to participate in an existing transaction.
	WithTx(tx *sql.Tx) Repository[T]
}
