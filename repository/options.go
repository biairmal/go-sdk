package repository

import "math"

// ListOptions are options for listing entities.
type ListOptions struct {
	Filter     Filter     // Filtering criteria
	Pagination Pagination // Pagination settings
	Sort       Sort       // Sorting settings
}

// Filter provides generic filtering options.
// Conditions map is safer than raw SQL.
// RawWhere is escape hatch for complex queries.
// Repository implementations should sanitize inputs.
// Consider using builder pattern for complex filters.
type Filter struct {
	// Map of field names to values for equality filtering.
	// Example: {"status": "active", "type": "premium"}
	Conditions map[string]any

	// Raw SQL WHERE clause (use with caution).
	// Example: "created_at > ? AND status = ?"
	RawWhere string

	// Arguments for RawWhere clause.
	RawArgs []any
}

// Pagination provides pagination settings.
// Offset-based: Simple but not performant for large offsets.
// Cursor-based: More performant, better for infinite scroll.
// Repository should support at least offset-based.
// Cursor-based is optional but recommended.
type Pagination struct {
	Limit  int    // Number of items per page (default: 20, max: 100)
	Offset int    // Number of items to skip
	Cursor string // Opaque cursor string (for cursor-based pagination)
}

// Sort provides sorting options.
// Field name should be validated against allowed columns.
// Prevent SQL injection via field name.
type Sort struct {
	Field     string        // Field name to sort by
	Direction SortDirection // ASC or DESC
}

// SortDirection represents sort direction.
type SortDirection string

const (
	// SortAsc represents ascending sort order.
	SortAsc SortDirection = "ASC"

	// SortDesc represents descending sort order.
	SortDesc SortDirection = "DESC"
)

// PagedResult is a result wrapper for paginated queries.
type PagedResult[T any] struct {
	Items      []*T   // Retrieved items
	Total      int64  // Total number of items (across all pages)
	Limit      int    // Items per page
	Offset     int    // Current offset
	Page       int    // Current page number (1-based, calculated from Offset/Limit)
	TotalPages int    // Total number of pages (calculated from Total/Limit)
	HasPrev    bool   // Whether there are previous pages
	HasNext    bool   // Whether there are more pages
	NextCursor string // Next page cursor (for cursor-based pagination)
}

// NewPagedResult creates a new PagedResult with calculated fields.
// Calculates Page, TotalPages, HasPrev, and HasNext from provided values.
func NewPagedResult[T any](items []*T, total int64, limit, offset int) *PagedResult[T] {
	if limit <= 0 {
		limit = 20 // Default limit
	}

	page := 1
	if limit > 0 {
		page = (offset / limit) + 1
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	hasPrev := page > 1
	hasNext := offset+limit < int(total)

	return &PagedResult[T]{
		Items:      items,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		Page:       page,
		TotalPages: totalPages,
		HasPrev:    hasPrev,
		HasNext:    hasNext,
	}
}
