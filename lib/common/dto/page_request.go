package dto

// PageRequest is the interface for page request parameters.
type PageRequest interface {
	GetPage() int
	SetPage(page int)
	GetSize() int
	SetSize(size int)
	GetSorts() []SortSpec
	SetSorts(sorts []SortSpec)
}

// BasePageRequest is the base implementation of PageRequest.
type BasePageRequest struct {
	Page  int        `json:"page" validate:"required,min=1"` // 1-based page number
	Size  int        `json:"size" validate:"required,min=1"` // Items per page (default 20)
	Sorts []SortSpec `json:"sorts"`                          // Sort by multiple columns
}

// NewBasePageRequest creates a new BasePageRequest.
func NewBasePageRequest(page, size int, sorts []SortSpec) *BasePageRequest {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	return &BasePageRequest{
		Page:  page,
		Size:  size,
		Sorts: sorts,
	}
}

// SetPage sets the page number.
func (r *BasePageRequest) GetPage() int {
	return r.Page
}

// SetPage sets the page number.
func (r *BasePageRequest) SetPage(page int) {
	r.Page = page
}

// GetSize returns the size of the page.
func (r *BasePageRequest) GetSize() int {
	return r.Size
}

// SetSize sets the size of the page.
func (r *BasePageRequest) SetSize(size int) {
	r.Size = size
}

// GetSorts returns the sort specifications.
func (r *BasePageRequest) GetSorts() []SortSpec {
	return r.Sorts
}

// SetSorts sets the sort specifications.
func (r *BasePageRequest) SetSorts(sorts []SortSpec) {
	r.Sorts = sorts
}

// SortDirection represents sort direction.
type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

// SortSpec specifies one sort column and direction.
type SortSpec struct {
	Field     string        `json:"field"`     // Column name
	Direction SortDirection `json:"direction"` // "asc" or "desc"
}
