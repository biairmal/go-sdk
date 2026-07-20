package auth

import (
	"context"
	"reflect"
	"testing"

	"github.com/biairmal/go-sdk/ctxkit"
)

func TestGetPath(t *testing.T) {
	m := map[string]any{
		"sub": "user-1",
		"org": map[string]any{
			"id":   "org-9",
			"team": map[string]any{"name": "core"},
		},
	}
	tests := []struct {
		name string
		path string
		want any
		ok   bool
	}{
		{"top-level hit", "sub", "user-1", true},
		{"nested hit", "org.id", "org-9", true},
		{"deep nested hit", "org.team.name", "core", true},
		{"missing key", "org.missing", nil, false},
		{"non-map intermediate", "sub.foo", nil, false},
		{"empty path", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := getPath(m, tt.path)
			if ok != tt.ok || got != tt.want {
				t.Errorf("getPath(%q) = (%v, %v), want (%v, %v)", tt.path, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNewClaims(t *testing.T) {
	tests := []struct {
		name      string
		raw       map[string]any
		wantSub   string
		wantRoles []string
	}{
		{"string roles slice", map[string]any{"sub": "u1", "roles": []string{"admin"}}, "u1", []string{"admin"}},
		{"any roles slice from json", map[string]any{"sub": "u2", "roles": []any{"a", "b"}}, "u2", []string{"a", "b"}},
		{"missing subject", map[string]any{}, "", nil},
		{"non-string subject ignored", map[string]any{"sub": 42}, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClaims(tt.raw)
			if c.Subject() != tt.wantSub {
				t.Errorf("Subject() = %q, want %q", c.Subject(), tt.wantSub)
			}
			if !reflect.DeepEqual(c.Roles(), tt.wantRoles) {
				t.Errorf("Roles() = %v, want %v", c.Roles(), tt.wantRoles)
			}
		})
	}
}

func TestContextWithClaims(t *testing.T) {
	claims := NewClaims(map[string]any{"sub": "user-42", "roles": []string{"admin"}})
	ctx := ContextWithClaims(context.Background(), claims)

	got, ok := ClaimsFromContext(ctx)
	if !ok || got.Subject() != "user-42" {
		t.Fatalf("ClaimsFromContext() = (%v, %v), want subject user-42", got, ok)
	}
	if SubjectFromContext(ctx) != "user-42" {
		t.Errorf("SubjectFromContext() = %q, want user-42", SubjectFromContext(ctx))
	}
	if ctxkit.UserID(ctx) != "user-42" {
		t.Errorf("ctxkit.UserID() = %q, want user-42 (subject must be published)", ctxkit.UserID(ctx))
	}
}

func TestContextWithClaimsNil(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), nil)
	if _, ok := ClaimsFromContext(ctx); ok {
		t.Error("expected no claims in context for nil input")
	}
	if SubjectFromContext(ctx) != "" {
		t.Error("expected empty subject for nil claims")
	}
}
