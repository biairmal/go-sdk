package repository

// ListOptions are options for listing entities.
type ListOptions struct {
	Pagination Pagination // Pagination settings
	Filter     Filter     // Filtering criteria
	Sorts      []Sort     // Sort by multiple columns (order preserved)
	SkipCount  bool       // Skip count query
}

// FilterCondition specifies one filter: field, operator, and value(s).
// Use Value for single-value operators (eq, ne, gt, gte, lt, lte, like).
// Use Values for the "in" operator.
type FilterCondition struct {
	Field    string         // Column name
	Operator FilterOperator // Operator
	Value    any            // Value for single-value operators
	Values   []any          // Values for the "in" operator
}

// FilterOperator represents filter operator.
type FilterOperator string

const (
	FilterOperatorEq        FilterOperator = "eq"
	FilterOperatorNe        FilterOperator = "ne"
	FilterOperatorGt        FilterOperator = "gt"
	FilterOperatorGte       FilterOperator = "gte"
	FilterOperatorLt        FilterOperator = "lt"
	FilterOperatorLte       FilterOperator = "lte"
	FilterOperatorLike      FilterOperator = "like"
	FilterOperatorIn        FilterOperator = "in"
	FilterOperatorIsNull    FilterOperator = "is_null"
	FilterOperatorIsNotNull FilterOperator = "is_not_null"
)

// Filter provides generic filtering options.
// Conditions is a list of predicate conditions (combined with AND).
type Filter struct {
	Conditions []FilterCondition
}

// Pagination provides pagination settings.
type Pagination struct {
	Limit  int
	Offset int
	Cursor string
}

// Sort provides sorting options.
type Sort struct {
	Field     string
	Direction SortDirection
}

// SortDirection represents sort direction.
type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)
