package sql

import (
	"strings"

	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
)

// Supported filter operators (whitelist for safety).
var supportedOps = map[string]bool{
	"eq": true, "ne": true, "gt": true, "gte": true, "lt": true, "lte": true,
	"like": true, "in": true, "is_null": true, "is_not_null": true,
}

// whereCondFunc builds one condition and returns condition SQL, args to append, and next arg index.
type whereCondFunc func(
	field string, c repository.FilterCondition, d Dialect, argIdx int,
) (cond string, addArgs []any, nextIdx int)

var whereCondBuilders = map[string]whereCondFunc{
	"eq": func(f string, c repository.FilterCondition, d Dialect, i int) (string, []any, int) {
		return f + " = " + d.Placeholder(i), []any{c.Value}, i + 1
	},
	"ne": func(f string, c repository.FilterCondition, d Dialect, i int) (string, []any, int) {
		return f + " <> " + d.Placeholder(i), []any{c.Value}, i + 1
	},
	"gt": func(f string, c repository.FilterCondition, d Dialect, i int) (string, []any, int) {
		return f + " > " + d.Placeholder(i), []any{c.Value}, i + 1
	},
	"gte": func(f string, c repository.FilterCondition, d Dialect, i int) (string, []any, int) {
		return f + " >= " + d.Placeholder(i), []any{c.Value}, i + 1
	},
	"lt": func(f string, c repository.FilterCondition, d Dialect, i int) (string, []any, int) {
		return f + " < " + d.Placeholder(i), []any{c.Value}, i + 1
	},
	"lte": func(f string, c repository.FilterCondition, d Dialect, i int) (string, []any, int) {
		return f + " <= " + d.Placeholder(i), []any{c.Value}, i + 1
	},
	"like": func(f string, c repository.FilterCondition, d Dialect, i int) (string, []any, int) {
		return f + " LIKE " + d.Placeholder(i), []any{c.Value}, i + 1
	},
	"in": buildWhereInCond,
	"is_null": func(f string, _ repository.FilterCondition, _ Dialect, i int) (string, []any, int) {
		return f + " IS NULL", nil, i
	},
	"is_not_null": func(f string, _ repository.FilterCondition, _ Dialect, i int) (string, []any, int) {
		return f + " IS NOT NULL", nil, i
	},
}

func buildWhereInCond(
	field string, c repository.FilterCondition, d Dialect, argIdx int,
) (cond string, addArgs []any, nextIdx int) {
	if len(c.Values) == 0 {
		return "", nil, argIdx
	}
	placeholders := make([]string, len(c.Values))
	for i := range c.Values {
		placeholders[i] = d.Placeholder(argIdx)
		argIdx++
	}
	cond = field + " IN (" + strings.Join(placeholders, ", ") + ")"
	addArgs = c.Values
	nextIdx = argIdx
	return cond, addArgs, nextIdx
}

// BuildWhereClause builds WHERE clause from filter using the given dialect for placeholders.
func BuildWhereClause(dialect Dialect, filter repository.Filter) (whereClause string, whereArgs []any) {
	if dialect == nil {
		dialect = DefaultDialect
	}
	var conditions []string
	var args []any
	argIdx := 1

	for _, c := range filter.Conditions {
		field := SanitizeColumnName(c.Field)
		if field == "" {
			continue
		}
		op := strings.ToLower(string(c.Operator))
		if !supportedOps[op] {
			continue
		}
		build := whereCondBuilders[op]
		if build == nil {
			continue
		}
		cond, addArgs, nextIdx := build(field, c, dialect, argIdx)
		if cond != "" {
			conditions = append(conditions, cond)
			args = append(args, addArgs...)
			argIdx = nextIdx
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// BuildOrderByClause builds ORDER BY clause from multiple sorts.
func BuildOrderByClause(sorts []repository.Sort) string {
	if len(sorts) == 0 {
		return ""
	}
	var parts []string
	for _, s := range sorts {
		if s.Field == "" {
			continue
		}
		field := SanitizeColumnName(s.Field)
		if field == "" {
			continue
		}
		dir := string(s.Direction)
		if dir != string(repository.SortAsc) && dir != string(repository.SortDesc) {
			dir = string(repository.SortAsc)
		}
		parts = append(parts, field+" "+dir)
	}
	if len(parts) == 0 {
		return ""
	}
	return "ORDER BY " + strings.Join(parts, ", ")
}

// BuildPaginationClause returns the pagination SQL fragment and args [limit, offset] using dialect.
func BuildPaginationClause(dialect Dialect, pagination repository.Pagination) (clause string, args []any) {
	if dialect == nil {
		dialect = DefaultDialect
	}
	if pagination.Limit <= 0 {
		pagination.Limit = 20
	}
	if pagination.Limit > 100 {
		pagination.Limit = 100
	}
	if pagination.Offset < 0 {
		pagination.Offset = 0
	}
	clause = dialect.PaginationClause(1, 2)
	args = []any{pagination.Limit, pagination.Offset}
	return clause, args
}

// SanitizeColumnName validates and sanitizes column names.
func SanitizeColumnName(column string) string {
	column = strings.TrimSpace(column)
	if strings.ContainsAny(column, "';\"\\()[]{}") {
		return ""
	}
	return strings.Trim(column, ".")
}

// ConvertSQLError converts database-specific errors to repository errors.
func ConvertSQLError(err error) error {
	if err == nil {
		return nil
	}
	if sqlkit.IsNoRows(err) {
		return repository.ErrNotFound
	}
	// TODO: map MySQL 1062, Oracle ORA-00001 to ErrAlreadyExists
	return err
}
