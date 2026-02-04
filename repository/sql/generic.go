package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/sqlkit"
)

// ScanFunc scans database rows into entity.
type ScanFunc[T any] func(rows *sql.Rows) (*T, error)

// InsertQueryBuilder builds INSERT query and extracts values from entity.
type InsertQueryBuilder[T any] interface {
	BuildQuery(tableName string, idColumn string) string
	ExtractValues(entity *T) []any
	SetID(entity *T, id int64) error
}

// UpdateQueryBuilder builds UPDATE query and extracts values from entity.
type UpdateQueryBuilder[T any] interface {
	BuildQuery(tableName string, idColumn string) []string // Returns list of SET clauses
	ExtractValues(entity *T) []any
}

// GenericRepoOption configures GenericRepository.
type GenericRepoOption[T any] func(*GenericRepository[T])

// GenericRepository is a generic CRUD repository implementation.
// Mapper functions (can be auto-generated or provided).
type GenericRepository[T any] struct {
	*BaseRepository

	scanFunc       ScanFunc[T]
	insertBuilder  InsertQueryBuilder[T]
	updateBuilder  UpdateQueryBuilder[T]
	allowedColumns []string // For column name validation
}

// NewGenericRepository creates a generic repository.
// Requires scanFunc to scan rows into entity.
// Insert and update builders are optional - if not provided, Create and Update will return errors.
func NewGenericRepository[T any](
	db *sqlkit.DB,
	tableName string,
	scanFunc ScanFunc[T],
	opts ...GenericRepoOption[T],
) repository.Repository[T] {
	repo := &GenericRepository[T]{
		BaseRepository: NewBaseRepository(db, tableName),
		scanFunc:       scanFunc,
	}

	for _, opt := range opts {
		opt(repo)
	}

	return repo
}

// WithInsertBuilder sets the insert query builder.
func WithInsertBuilder[T any](builder InsertQueryBuilder[T]) GenericRepoOption[T] {
	return func(r *GenericRepository[T]) {
		r.insertBuilder = builder
	}
}

// WithUpdateBuilder sets the update query builder.
func WithUpdateBuilder[T any](builder UpdateQueryBuilder[T]) GenericRepoOption[T] {
	return func(r *GenericRepository[T]) {
		r.updateBuilder = builder
	}
}

// WithAllowedColumns sets allowed column names for validation.
func WithAllowedColumns[T any](columns []string) GenericRepoOption[T] {
	return func(r *GenericRepository[T]) {
		r.allowedColumns = columns
	}
}

// Create inserts a new entity.
// Error handling:
// - Duplicate key → ErrAlreadyExists
// - Constraint violation → ErrInvalidEntity
// - Other errors → wrap with context
func (r *GenericRepository[T]) Create(ctx context.Context, entity *T) error {
	if r.insertBuilder == nil {
		return fmt.Errorf("repository: insert builder not configured")
	}

	conn := r.GetConnection(ctx)

	// Build INSERT query
	query := r.insertBuilder.BuildQuery(r.tableName, r.idColumn)
	args := r.insertBuilder.ExtractValues(entity)

	// Execute
	result, err := conn.ExecContext(ctx, query, args...)
	if err != nil {
		return ConvertSQLError(err)
	}

	// Get generated ID if applicable
	if id, err := result.LastInsertId(); err == nil {
		if err := r.insertBuilder.SetID(entity, id); err != nil {
			// Log error but don't fail - ID might not be settable
			_ = err
		}
	}

	return nil
}

// GetByID retrieves an entity by its ID.
func (r *GenericRepository[T]) GetByID(ctx context.Context, id any) (*T, error) {
	conn := r.GetReadConnection(ctx)

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", r.tableName, r.idColumn)

	rows, err := conn.QueryContext(ctx, query, id)
	if err != nil {
		return nil, ConvertSQLError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, repository.ErrNotFound
	}

	entity, err := r.scanFunc(rows)
	if err != nil {
		return nil, ConvertSQLError(err)
	}

	return entity, nil
}

// Update updates an existing entity.
func (r *GenericRepository[T]) Update(ctx context.Context, id any, entity *T) error {
	if r.updateBuilder == nil {
		return fmt.Errorf("repository: update builder not configured")
	}

	conn := r.GetConnection(ctx)

	// Build UPDATE query
	setClauses := r.updateBuilder.BuildQuery(r.tableName, r.idColumn)
	if len(setClauses) == 0 {
		return fmt.Errorf("repository: no fields to update")
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
		r.tableName,
		strings.Join(setClauses, ", "),
		r.idColumn,
		len(setClauses)+1,
	)

	args := r.updateBuilder.ExtractValues(entity)
	args = append(args, id)

	result, err := conn.ExecContext(ctx, query, args...)
	if err != nil {
		return ConvertSQLError(err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return repository.ErrNotFound
	}

	return nil
}

// Delete removes an entity by its ID.
func (r *GenericRepository[T]) Delete(ctx context.Context, id any) error {
	conn := r.GetConnection(ctx)

	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", r.tableName, r.idColumn)

	result, err := conn.ExecContext(ctx, query, id)
	if err != nil {
		return ConvertSQLError(err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return repository.ErrNotFound
	}

	return nil
}

// List retrieves entities with filtering and pagination.
func (r *GenericRepository[T]) List(ctx context.Context, opts *repository.ListOptions) ([]*T, error) {
	conn := r.GetReadConnection(ctx)

	// Build query with filters, pagination, sorting
	query, args := r.buildListQuery(opts)

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, ConvertSQLError(err)
	}
	defer rows.Close()

	var entities []*T
	for rows.Next() {
		entity, err := r.scanFunc(rows)
		if err != nil {
			return nil, ConvertSQLError(err)
		}
		entities = append(entities, entity)
	}

	return entities, rows.Err()
}

// Count returns the total number of entities matching the filter.
func (r *GenericRepository[T]) Count(ctx context.Context, filter repository.Filter) (int64, error) {
	conn := r.GetReadConnection(ctx)

	query, args := r.buildCountQuery(filter)

	var count int64
	err := conn.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, ConvertSQLError(err)
	}

	return count, nil
}

// Exists checks if an entity with given ID exists.
func (r *GenericRepository[T]) Exists(ctx context.Context, id any) (bool, error) {
	conn := r.GetReadConnection(ctx)

	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = $1)", r.tableName, r.idColumn)

	var exists bool
	err := conn.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, ConvertSQLError(err)
	}

	return exists, nil
}

// buildListQuery builds SELECT query with filters, pagination, and sorting.
func (r *GenericRepository[T]) buildListQuery(opts *repository.ListOptions) (listQuery string, listArgs []any) {
	query := fmt.Sprintf("SELECT * FROM %s", r.tableName)
	var args []any

	if opts == nil {
		return query, args
	}

	// Build WHERE clause
	whereClause, whereArgs := BuildWhereClause(opts.Filter)
	if whereClause != "" {
		query += " " + whereClause
		args = append(args, whereArgs...)
	}

	// Build ORDER BY clause
	orderByClause := BuildOrderByClause(opts.Sort)
	if orderByClause != "" {
		query += " " + orderByClause
	}

	// Build pagination clause
	// Adjust argument indices for pagination
	paginationClause, paginationArgs := BuildPaginationClause(opts.Pagination)
	if paginationClause != "" {
		// Replace $1, $2 in pagination clause with correct indices
		argIdx := len(args) + 1
		paginationClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		query += " " + paginationClause
		args = append(args, paginationArgs...)
	}

	return query, args
}

// buildCountQuery builds COUNT query with filters.
func (r *GenericRepository[T]) buildCountQuery(filter repository.Filter) (countQuery string, countArgs []any) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.tableName)

	// Build WHERE clause
	whereClause, args := BuildWhereClause(filter)
	if whereClause != "" {
		query += " " + whereClause
	}

	return query, args
}
