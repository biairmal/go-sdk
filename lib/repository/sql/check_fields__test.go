package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/repository"
)

type fieldsEntity struct {
	ID       string `db:"id"`
	TenantID string `db:"tenant_id"`
	Name     string `db:"name"`
	secret   string //nolint:unused // unexported: never a column
	Skipped  string `db:"-"`
}

func newFieldsRepo() *SQLRepository[fieldsEntity, string] {
	repo, ok := NewSQLRepository[fieldsEntity, string](nil, nil, "t").(*SQLRepository[fieldsEntity, string])
	if !ok {
		panic("unexpected repository type")
	}
	return repo
}

func TestCheckFields(t *testing.T) {
	tests := []struct {
		name    string
		filter  repository.Filter
		sorts   []repository.Sort
		wantErr bool
	}{
		{name: "none"},
		{name: "known filter", filter: filterOn("name")},
		{name: "case insensitive", filter: filterOn("Tenant_ID")},
		{name: "known sort", sorts: []repository.Sort{{Field: "name"}}},
		{
			name:    "unknown filter",
			filter:  filterOn("password_hash"),
			wantErr: true,
		},
		{
			name:    "injected filter",
			filter:  filterOn("1=1 OR id"),
			wantErr: true,
		},
		{name: "unknown sort", sorts: []repository.Sort{{Field: "password_hash"}}, wantErr: true},
		{name: "dash tag is not a column", sorts: []repository.Sort{{Field: "skipped"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newFieldsRepo().checkFields(tt.filter, tt.sorts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errorsIsBadRequest(err) {
				t.Errorf("err = %v, want a bad request error", err)
			}
		})
	}
}

// List and Count must refuse before touching the database (the repo has no db here, so reaching it would panic).
func TestListAndCount_RejectUnknownFieldsBeforeQuerying(t *testing.T) {
	bad := repository.Filter{Conditions: []repository.FilterCondition{
		{Field: "1=1 OR id", Operator: repository.FilterOperatorEq, Value: "x"},
	}}
	repo := newFieldsRepo()
	if _, _, err := repo.List(context.Background(), &repository.ListOptions{Filter: bad}); !errorsIsBadRequest(err) {
		t.Errorf("List err = %v, want bad request", err)
	}
	if _, err := repo.Count(context.Background(), bad); !errorsIsBadRequest(err) {
		t.Errorf("Count err = %v, want bad request", err)
	}
}

func errorsIsBadRequest(err error) bool {
	return err != nil && errors.Is(err, errorz.ErrBadRequest)
}

func filterOn(field string) repository.Filter {
	return repository.Filter{Conditions: []repository.FilterCondition{
		{Field: field, Operator: repository.FilterOperatorEq, Value: "x"},
	}}
}
