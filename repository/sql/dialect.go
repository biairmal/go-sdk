package sql

import "fmt"

// Dialect abstracts SQL dialect differences (placeholders, pagination, optional quoting).
type Dialect interface {
	// Placeholder returns the placeholder string for the given 1-based argument index.
	// e.g. Postgres "$1", MySQL "?", Oracle ":1"
	Placeholder(index int) string

	// PaginationClause returns the SQL fragment for LIMIT/OFFSET and the two args (limit, offset).
	// Postgres/MySQL: "LIMIT ? OFFSET ?"; Oracle: "OFFSET ? ROWS FETCH NEXT ? ROWS ONLY"
	PaginationClause(limitArgIndex, offsetArgIndex int) string
}

// Postgres dialect (placeholder $1, $2, ...).
type Postgres struct{}

func (Postgres) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (Postgres) PaginationClause(limitArgIndex, offsetArgIndex int) string {
	return fmt.Sprintf("LIMIT %s OFFSET %s", fmt.Sprintf("$%d", limitArgIndex), fmt.Sprintf("$%d", offsetArgIndex))
}

// MySQL dialect (placeholder ?).
type MySQL struct{}

func (MySQL) Placeholder(index int) string {
	return "?"
}

func (MySQL) PaginationClause(limitArgIndex, offsetArgIndex int) string {
	return "LIMIT ? OFFSET ?"
}

// Oracle dialect (placeholder :1, :2, ...). Pagination uses OFFSET/FETCH (12c+).
type Oracle struct{}

func (Oracle) Placeholder(index int) string {
	return fmt.Sprintf(":%d", index)
}

func (Oracle) PaginationClause(limitArgIndex, offsetArgIndex int) string {
	// Oracle 12c+: OFFSET n ROWS FETCH NEXT m ROWS ONLY (args: offset, limit in that order in standard)
	// We pass args as [limit, offset] so use placeholders accordingly
	return fmt.Sprintf("OFFSET %s ROWS FETCH NEXT %s ROWS ONLY", fmt.Sprintf(":%d", offsetArgIndex), fmt.Sprintf(":%d", limitArgIndex))
}

// DefaultDialect is used when no dialect is set (Postgres for backward compatibility).
var DefaultDialect Dialect = Postgres{}
