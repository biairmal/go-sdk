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

func TestSanitizeColumnName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "email", want: "email"},
		{name: "underscore and digits", in: "created_at2", want: "created_at2"},
		{name: "table qualified", in: "n.owner_id", want: "n.owner_id"},
		{name: "trimmed", in: "  email ", want: "email"},
		{name: "boolean bypass", in: "1=1 OR id", want: ""},
		{name: "or clause", in: "id OR name", want: ""},
		{name: "union", in: "x UNION SELECT *", want: ""},
		{name: "comment", in: "id--", want: ""},
		{name: "block comment", in: "id/*", want: ""},
		{name: "semicolon", in: "id;DROP TABLE t", want: ""},
		{name: "quote", in: `id"`, want: ""},
		{name: "parens", in: "lower(email)", want: ""},
		{name: "leading dot", in: ".id", want: ""},
		{name: "two dots", in: "a.b.c", want: ""},
		{name: "starts with digit", in: "1id", want: ""},
		{name: "empty", in: "", want: ""},
		{name: "star", in: "*", want: ""},
		{name: "newline", in: "id\nOR 1=1", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeColumnName(tt.in); got != tt.want {
				t.Errorf("SanitizeColumnName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A tenant filter ANDed with a hostile field must never let the hostile text into the SQL.
func TestBuildWhereAndOrderBy_DropInjectedFields(t *testing.T) {
	filter := repository.Filter{Conditions: []repository.FilterCondition{
		{Field: "tenant_id", Operator: repository.FilterOperatorEq, Value: "t1"},
		{Field: "1=1 OR id", Operator: repository.FilterOperatorEq, Value: "x"},
	}}
	where, args := BuildWhereClause(Postgres{}, filter)
	if strings.Contains(where, "1=1") || strings.Contains(where, " OR ") {
		t.Errorf("where = %q, want the hostile field left out", where)
	}
	if len(args) != 1 {
		t.Errorf("len(args) = %d, want 1 (only the tenant condition)", len(args))
	}

	order := BuildOrderByClause([]repository.Sort{{Field: "name, (select 1)"}, {Field: "x UNION SELECT *"}})
	if order != "" {
		t.Errorf("order = %q, want empty", order)
	}
}
