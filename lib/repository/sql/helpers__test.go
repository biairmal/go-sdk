package sql

import (
	"strings"
	"testing"

	"github.com/biairmal/go-sdk/lib/repository"
)

// TestBuildPaginationClause_ArgOffset guards against a regression where
// LIMIT/OFFSET placeholders were hardcoded to $1/$2 regardless of how many
// WHERE args preceded them: with a numbered-placeholder dialect (Postgres,
// Oracle), a preceding WHERE clause using $1 caused LIMIT to reuse $1 too,
// so the driver bound LIMIT to the WHERE value instead of the limit —
// surfacing as "argument of LIMIT must be type bigint, not type <whatever>".
func TestBuildPaginationClause_ArgOffset(t *testing.T) {
	tests := []struct {
		name      string
		argOffset int
		wantLimit string
		wantOff   string
	}{
		{name: "no preceding args", argOffset: 0, wantLimit: "$1", wantOff: "$2"},
		{name: "one preceding WHERE arg", argOffset: 1, wantLimit: "$2", wantOff: "$3"},
		{name: "three preceding WHERE args", argOffset: 3, wantLimit: "$4", wantOff: "$5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause, args := BuildPaginationClause(Postgres{}, repository.Pagination{Limit: 10, Offset: 0}, tt.argOffset)
			if !strings.Contains(clause, "LIMIT "+tt.wantLimit) {
				t.Errorf("clause = %q, want it to contain %q", clause, "LIMIT "+tt.wantLimit)
			}
			if !strings.Contains(clause, "OFFSET "+tt.wantOff) {
				t.Errorf("clause = %q, want it to contain %q", clause, "OFFSET "+tt.wantOff)
			}
			if len(args) != 2 {
				t.Errorf("len(args) = %d, want 2", len(args))
			}
		})
	}
}

// TestPaginationClause_NoPlaceholderCollisionWithWhere builds a WHERE clause
// then a pagination clause chained onto it (mirroring SQLRepository.
// buildListQuery) and asserts every numbered placeholder in the combined SQL
// is unique — a collision is exactly the bug TestBuildPaginationClause_
// ArgOffset guards against, verified here on the composed query text.
func TestPaginationClause_NoPlaceholderCollisionWithWhere(t *testing.T) {
	filter := repository.Filter{Conditions: []repository.FilterCondition{
		{Field: "email", Operator: repository.FilterOperatorEq, Value: "a@acme.com"},
	}}
	whereClause, whereArgs := BuildWhereClause(Postgres{}, filter)
	paginationClause, paginationArgs := BuildPaginationClause(
		Postgres{}, repository.Pagination{Limit: 1}, len(whereArgs),
	)

	query := whereClause + " " + paginationClause
	seen := map[string]bool{}
	for _, ph := range []string{"$1", "$2", "$3"} {
		count := strings.Count(query, ph)
		if count > 1 {
			t.Errorf(
				"placeholder %s appears %d times in %q, want at most 1 (collision binds two values to one arg)",
				ph, count, query,
			)
		}
		if count == 1 {
			seen[ph] = true
		}
	}
	wantArgs := len(whereArgs) + len(paginationArgs)
	if len(seen) != wantArgs {
		t.Errorf("query %q references %d distinct placeholders, want %d (matching total args)", query, len(seen), wantArgs)
	}
}
