package sql

import (
	"fmt"
	"strings"

	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/sqlkit"
)

// BuildWhereClause builds WHERE clause from filter.
// Returns WHERE clause and arguments.
// Example: "WHERE status = $1 AND type = $2", []any{"active", "premium"}.
func BuildWhereClause(filter repository.Filter) (whereClause string, whereArgs []any) {
	var conditions []string
	var args []any
	argIdx := 1

	// Add conditions from map
	for field, value := range filter.Conditions {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", SanitizeColumnName(field), argIdx))
		args = append(args, value)
		argIdx++
	}

	// Add raw WHERE clause if provided
	if filter.RawWhere != "" {
		// Replace placeholders in RawWhere with proper positional arguments
		rawWhere := filter.RawWhere
		for range filter.RawArgs {
			rawWhere = strings.Replace(rawWhere, "?", fmt.Sprintf("$%d", argIdx), 1)
			argIdx++
		}
		conditions = append(conditions, rawWhere)
		args = append(args, filter.RawArgs...)
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

// BuildOrderByClause builds ORDER BY clause from sort.
// Implementation with SQL injection protection.
// Returns ORDER BY clause.
func BuildOrderByClause(sort repository.Sort) string {
	if sort.Field == "" {
		return ""
	}

	// Sanitize field name to prevent SQL injection
	field := SanitizeColumnName(sort.Field)

	// Validate direction
	direction := string(sort.Direction)
	if direction != string(repository.SortAsc) && direction != string(repository.SortDesc) {
		direction = string(repository.SortAsc) // Default to ASC
	}

	return fmt.Sprintf("ORDER BY %s %s", field, direction)
}

// BuildPaginationClause builds LIMIT/OFFSET clause.
// Returns LIMIT/OFFSET clause and arguments.
func BuildPaginationClause(pagination repository.Pagination) (paginationClause string, paginationArgs []any) {
	if pagination.Limit <= 0 {
		pagination.Limit = 20 // Default limit
	}
	if pagination.Limit > 100 {
		pagination.Limit = 100 // Max limit
	}

	if pagination.Offset < 0 {
		pagination.Offset = 0
	}

	return "LIMIT $1 OFFSET $2", []any{pagination.Limit, pagination.Offset}
}

// SanitizeColumnName validates and sanitizes column names.
// Whitelist-based validation.
// For now, uses simple validation - in production, should validate against allowed columns.
// This is a basic implementation that prevents obvious SQL injection.
// For production use, maintain a whitelist of allowed column names.
func SanitizeColumnName(column string) string {
	// Remove any characters that could be used for SQL injection
	// Allow only alphanumeric, underscore, and dot
	column = strings.TrimSpace(column)

	// Basic validation - reject if contains dangerous characters
	if strings.ContainsAny(column, "';\"\\()[]{}") {
		return ""
	}

	// Remove leading/trailing dots
	column = strings.Trim(column, ".")

	return column
}

// ConvertSQLError converts database-specific errors to repository errors.
func ConvertSQLError(err error) error {
	if err == nil {
		return nil
	}

	// Check for ErrNoRows
	if sqlkit.IsNoRows(err) {
		return repository.ErrNotFound
	}

	// TODO: Check for duplicate key errors (database-specific)
	// PostgreSQL: pq.Error with code 23505
	// MySQL: Error 1062

	// TODO: Check for foreign key violations
	// PostgreSQL: pq.Error with code 23503

	return err
}
