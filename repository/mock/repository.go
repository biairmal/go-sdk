package mock

import (
	"context"
	"sync"

	"github.com/biairmal/go-sdk/repository"
)

// Repository is an in-memory repository for testing.
// Use cases:
// - Unit testing without database
// - Integration testing setup/teardown
// - Testing error scenarios
type Repository[T any] struct {
	data      map[any]*T
	mu        sync.RWMutex
	idCounter int64

	// For testing specific scenarios
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
	countErr  error
	existsErr error
}

// NewMockRepository creates a new mock repository.
func NewMockRepository[T any]() *Repository[T] {
	return &Repository[T]{
		data: make(map[any]*T),
	}
}

// SetCreateError configures the repository to return an error on Create.
func (r *Repository[T]) SetCreateError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.createErr = err
}

// SetGetError configures the repository to return an error on GetByID.
func (r *Repository[T]) SetGetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getErr = err
}

// SetUpdateError configures the repository to return an error on Update.
func (r *Repository[T]) SetUpdateError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateErr = err
}

// SetDeleteError configures the repository to return an error on Delete.
func (r *Repository[T]) SetDeleteError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleteErr = err
}

// SetListError configures the repository to return an error on List.
func (r *Repository[T]) SetListError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listErr = err
}

// SetCountError configures the repository to return an error on Count.
func (r *Repository[T]) SetCountError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.countErr = err
}

// SetExistsError configures the repository to return an error on Exists.
func (r *Repository[T]) SetExistsError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.existsErr = err
}

// Reset clears all data and errors.
func (r *Repository[T]) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = make(map[any]*T)
	r.idCounter = 0
	r.createErr = nil
	r.getErr = nil
	r.updateErr = nil
	r.deleteErr = nil
	r.listErr = nil
	r.countErr = nil
	r.existsErr = nil
}

// Create inserts a new entity.
func (r *Repository[T]) Create(_ context.Context, entity *T) error {
	if r.createErr != nil {
		return r.createErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Simple ID generation - in real implementation, would use reflection
	// or require entity to have ID field
	r.idCounter++
	// For now, use pointer as key (not ideal, but works for testing)
	r.data[r.idCounter] = entity

	return nil
}

// GetByID retrieves an entity by its ID.
func (r *Repository[T]) GetByID(_ context.Context, id any) (*T, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, ok := r.data[id]
	if !ok {
		return nil, repository.ErrNotFound
	}

	return entity, nil
}

// Update updates an existing entity.
func (r *Repository[T]) Update(_ context.Context, id any, entity *T) error {
	if r.updateErr != nil {
		return r.updateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data[id]; !ok {
		return repository.ErrNotFound
	}

	r.data[id] = entity
	return nil
}

// Delete removes an entity by its ID.
func (r *Repository[T]) Delete(_ context.Context, id any) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data[id]; !ok {
		return repository.ErrNotFound
	}

	delete(r.data, id)
	return nil
}

// List retrieves entities with filtering and pagination.
// Note: This is a simplified implementation that ignores filters and pagination.
// For more complex testing, extend this implementation.
func (r *Repository[T]) List(_ context.Context, opts *repository.ListOptions) ([]*T, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var entities []*T
	for _, entity := range r.data {
		entities = append(entities, entity)
	}

	// Simple pagination (ignores filters and sorting)
	if opts.Pagination.Limit > 0 {
		start := opts.Pagination.Offset
		end := start + opts.Pagination.Limit
		if start > len(entities) {
			return []*T{}, nil
		}
		if end > len(entities) {
			end = len(entities)
		}
		entities = entities[start:end]
	}

	return entities, nil
}

// Count returns the total number of entities matching the filter.
// Note: This is a simplified implementation that ignores filters.
func (r *Repository[T]) Count(_ context.Context, _ repository.Filter) (int64, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return int64(len(r.data)), nil
}

// Exists checks if an entity with given ID exists.
func (r *Repository[T]) Exists(_ context.Context, id any) (bool, error) {
	if r.existsErr != nil {
		return false, r.existsErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.data[id]
	return ok, nil
}
