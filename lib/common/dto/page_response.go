package dto

import "math"

// PageResponse is the single type for paginated API responses.
// Built by the service from repository list + count; repository does not return this type.
type PageResponse[T any] struct {
	Items      []*T  `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Size       int   `json:"size"`
	TotalPages int   `json:"total_pages"`
	HasPrev    bool  `json:"has_prev"`
	HasNext    bool  `json:"has_next"`
}

// NewPageResponse builds a PageResponse from items, total count, and page/size.
// Computes TotalPages, HasPrev, and HasNext. Use this in the service layer after repo.List.
func NewPageResponse[T any](items []*T, total int64, page, size int) *PageResponse[T] {
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	totalPages := 1
	if size > 0 && total > 0 {
		totalPages = max(int(math.Ceil(float64(total)/float64(size))), 1)
	}
	return &PageResponse[T]{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
		HasPrev:    page > 1,
		HasNext:    page*size < int(total),
	}
}
