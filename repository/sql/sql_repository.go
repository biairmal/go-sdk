package sql

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/sqlkit"
)

// SQLRepositoryOption configures SQLRepository.
type SQLRepositoryOption[TEntity any, TID comparable] func(*SQLRepository[TEntity, TID])

// SQLRepository is a generic CRUD repository implementation using reflection (struct tag db).
type SQLRepository[TEntity any, TID comparable] struct {
	*BaseRepository
	log           logger.Logger
	dialect       Dialect
	selectColumns []string
	entityType    reflect.Type
}

// NewSQLRepository creates a new SQL repository.
// Logger may be nil (no query logging). Opts are optional (e.g. WithDialect, WithSelectColumns, WithIDColumn).
func NewSQLRepository[TEntity any, TID comparable](
	log logger.Logger,
	db *sqlkit.DB,
	tableName string,
	opts ...SQLRepositoryOption[TEntity, TID],
) repository.Repository[TEntity, TID] {
	var zero TEntity
	typ := reflect.TypeOf(&zero).Elem()
	if typ.Kind() != reflect.Struct {
		panic("repository: TEntity must be a struct type")
	}
	repo := &SQLRepository[TEntity, TID]{
		BaseRepository: NewBaseRepository(db, tableName),
		log:            log,
		dialect:        DefaultDialect,
		entityType:     typ,
	}
	for _, opt := range opts {
		opt(repo)
	}
	return repo
}

// WithDialect sets the SQL dialect (Postgres, MySQL, Oracle) for placeholders and pagination.
func WithDialect[TEntity any, TID comparable](d Dialect) SQLRepositoryOption[TEntity, TID] {
	return func(r *SQLRepository[TEntity, TID]) {
		if d != nil {
			r.dialect = d
		}
	}
}

// WithSelectColumns sets columns to SELECT for read operations (List, GetByID).
func WithSelectColumns[TEntity any, TID comparable](columns []string) SQLRepositoryOption[TEntity, TID] {
	return func(r *SQLRepository[TEntity, TID]) {
		r.selectColumns = columns
	}
}

// WithIDColumn sets the ID column name (default "id").
func WithIDColumn[TEntity any, TID comparable](column string) SQLRepositoryOption[TEntity, TID] {
	return func(r *SQLRepository[TEntity, TID]) {
		r.BaseRepository = r.BaseRepository.WithIDColumn(column)
	}
}

func (r *SQLRepository[TEntity, TID]) logQuery(ctx context.Context, query string, args []any) {
	if r.log == nil {
		return
	}
	if ctx != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", query, args)
	} else {
		r.log.Debugf("query: %s args: %v", query, args)
	}
}

func (r *SQLRepository[TEntity, TID]) getDialect() Dialect {
	d := r.dialect
	if d == nil {
		d = DefaultDialect
	}
	return d
}

// Create inserts a new entity using reflection (db tags).
// If the entity's ID is zero/nil, the ID column is omitted from INSERT so the DB can set it via DEFAULT;
// the generated ID is then written back to the entity (int64 via LastInsertId, UUID/string via RETURNING).
// If the entity's ID is non-zero, the row is inserted with that ID.
func (r *SQLRepository[TEntity, TID]) Create(ctx context.Context, entity *TEntity) error {
	conn := r.GetConnection(ctx)
	d := r.getDialect()
	idColumn := r.IDColumn()
	excludeID := IsEntityIDZero(entity, idColumn)
	query := BuildInsertQuery(r.TableName(), idColumn, d, r.entityType, excludeID)
	args := ExtractInsertValues(entity, idColumn, excludeID)
	r.logQuery(ctx, query, args)

	if excludeID && IsEntityIDFieldInt64(entity, idColumn) {
		result, err := conn.ExecContext(ctx, query, args...)
		if err != nil {
			return ConvertSQLError(err)
		}
		if id, err := result.LastInsertId(); err == nil && id != 0 {
			_ = SetEntityID(entity, id, idColumn)
		}
		return nil
	}
	if excludeID {
		queryReturning := query + " RETURNING " + idColumn
		r.logQuery(ctx, queryReturning, args)
		row := conn.QueryRowContext(ctx, queryReturning, args...)
		if err := ScanReturnedIDAndSetEntity(entity, idColumn, row); err != nil {
			return ConvertSQLError(err)
		}
		return nil
	}
	_, err := conn.ExecContext(ctx, query, args...)
	return ConvertSQLError(err)
}

// GetByID retrieves an entity by its ID.
func (r *SQLRepository[TEntity, TID]) GetByID(ctx context.Context, id TID) (*TEntity, error) {
	conn := r.GetReadConnection(ctx)
	sel := "*"
	if len(r.selectColumns) > 0 {
		sel = strings.Join(r.selectColumns, ", ")
	}
	d := r.getDialect()
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s", sel, r.TableName(), r.IDColumn(), d.Placeholder(1))
	args := []any{id}
	r.logQuery(ctx, query, args)
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, ConvertSQLError(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, repository.ErrNotFound
	}
	entity, err := ScanRow[TEntity](rows)
	if err != nil {
		return nil, ConvertSQLError(err)
	}
	return entity, nil
}

// Update updates an existing entity using reflection (db tags).
func (r *SQLRepository[TEntity, TID]) Update(ctx context.Context, id TID, entity *TEntity) error {
	conn := r.GetConnection(ctx)
	d := r.getDialect()
	query := BuildUpdateQuery(r.TableName(), r.IDColumn(), d, r.entityType)
	if query == "" {
		return fmt.Errorf("repository: no fields to update")
	}
	args := ExtractUpdateValues(entity, any(id), r.IDColumn())
	r.logQuery(ctx, query, args)
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
func (r *SQLRepository[TEntity, TID]) Delete(ctx context.Context, id TID) error {
	conn := r.GetConnection(ctx)
	d := r.getDialect()
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = %s", r.TableName(), r.IDColumn(), d.Placeholder(1))
	args := []any{id}
	r.logQuery(ctx, query, args)
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

// List retrieves entities with filtering and pagination and returns total count.
func (r *SQLRepository[TEntity, TID]) List(ctx context.Context, opts *repository.ListOptions) ([]*TEntity, int64, error) {
	conn := r.GetReadConnection(ctx)
	query, args := r.buildListQuery(opts)
	r.logQuery(ctx, query, args)
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, ConvertSQLError(err)
	}
	defer rows.Close()
	var entities []*TEntity
	for rows.Next() {
		entity, err := ScanRow[TEntity](rows)
		if err != nil {
			return nil, 0, ConvertSQLError(err)
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, ConvertSQLError(err)
	}
	var total int64 = 0
	if !opts.SkipCount {
		total, err = r.Count(ctx, opts.Filter)
		if err != nil {
			return nil, 0, ConvertSQLError(err)
		}
	}
	return entities, total, nil
}

// Count returns the total number of entities matching the filter.
func (r *SQLRepository[TEntity, TID]) Count(ctx context.Context, filter repository.Filter) (int64, error) {
	conn := r.GetReadConnection(ctx)
	query, args := r.buildCountQuery(filter)
	r.logQuery(ctx, query, args)
	var count int64
	err := conn.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, ConvertSQLError(err)
	}
	return count, nil
}

// Exists checks if an entity with given ID exists.
func (r *SQLRepository[TEntity, TID]) Exists(ctx context.Context, id TID) (bool, error) {
	conn := r.GetReadConnection(ctx)
	d := r.getDialect()
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = %s)", r.TableName(), r.IDColumn(), d.Placeholder(1))
	args := []any{id}
	r.logQuery(ctx, query, args)
	var exists bool
	err := conn.QueryRowContext(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, ConvertSQLError(err)
	}
	return exists, nil
}

func (r *SQLRepository[TEntity, TID]) buildListQuery(opts *repository.ListOptions) (listQuery string, listArgs []any) {
	sel := "*"
	if len(r.selectColumns) > 0 {
		sel = strings.Join(r.selectColumns, ", ")
	}
	query := fmt.Sprintf("SELECT %s FROM %s", sel, r.TableName())
	var args []any
	d := r.getDialect()
	if opts == nil {
		opts = &repository.ListOptions{}
	}
	whereClause, whereArgs := BuildWhereClause(d, opts.Filter)
	if whereClause != "" {
		query += " " + whereClause
		args = append(args, whereArgs...)
	}
	orderByClause := BuildOrderByClause(opts.Sorts)
	if orderByClause != "" {
		query += " " + orderByClause
	}
	paginationClause, paginationArgs := BuildPaginationClause(d, opts.Pagination)
	if paginationClause != "" {
		query += " " + paginationClause
		args = append(args, paginationArgs...)
	}
	return query, args
}

func (r *SQLRepository[TEntity, TID]) buildCountQuery(filter repository.Filter) (countQuery string, countArgs []any) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.TableName())
	d := r.getDialect()
	whereClause, args := BuildWhereClause(d, filter)
	if whereClause != "" {
		query += " " + whereClause
	}
	return query, args
}
