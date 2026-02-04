package sql

import (
	"context"
	"database/sql"

	"github.com/biairmal/go-sdk/sqlkit"
)

// BaseRepository provides common database access logic for SQL repositories.
type BaseRepository struct {
	db        *sqlkit.DB
	tableName string
	idColumn  string // Usually "id"
}

// NewBaseRepository creates a new base repository.
func NewBaseRepository(db *sqlkit.DB, tableName string) *BaseRepository {
	return &BaseRepository{
		db:        db,
		tableName: tableName,
		idColumn:  "id",
	}
}

// WithIDColumn sets a custom ID column name.
func (r *BaseRepository) WithIDColumn(column string) *BaseRepository {
	r.idColumn = column
	return r
}

// Connection is an interface for database operations.
type Connection interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// ReadConnection is an interface for read-only database operations.
type ReadConnection interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// GetConnection returns appropriate database connection for write operations.
// Behavior:
// 1. Check if transaction exists in context (sqlkit.ExtractTx).
// 2. If yes, return transaction.
// 3. If no, return db.Leader().
// Thread-safe: Yes.
// Use: All write operations (CREATE, UPDATE, DELETE).
func (r *BaseRepository) GetConnection(ctx context.Context) Connection {
	if tx, ok := sqlkit.ExtractTx(ctx); ok {
		return tx
	}
	return r.db.Leader()
}

// GetReadConnection returns appropriate database connection for read operations.
// Behavior:
// 1. Check if transaction exists in context.
// 2. If yes, return transaction (for read consistency).
// 3. If no, return db.Follower().
// Thread-safe: Yes.
// Use: All read operations (SELECT).
func (r *BaseRepository) GetReadConnection(ctx context.Context) ReadConnection {
	if tx, ok := sqlkit.ExtractTx(ctx); ok {
		return tx
	}
	return r.db.Follower()
}

// TableName returns the table name.
func (r *BaseRepository) TableName() string {
	return r.tableName
}

// IDColumn returns the ID column name.
func (r *BaseRepository) IDColumn() string {
	return r.idColumn
}
