package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/authz"
)

func TestRequirePermission(t *testing.T) {
	az, err := authz.New(&authz.Config{Source: authz.SourceToken})
	if err != nil {
		t.Fatalf("authz.New: %v", err)
	}
	withPerms := func(perms ...any) context.Context {
		return auth.ContextWithClaims(context.Background(), auth.NewClaims(map[string]any{"sub": "u", "perms": perms}))
	}

	tests := []struct {
		name string
		mw   func(http.Handler) http.Handler
		ctx  context.Context
		want int
	}{
		{"no claims is 401", RequirePermission(az, "notes:read"), context.Background(), http.StatusUnauthorized},
		{"missing permission is 403", RequirePermission(az, "notes:write"), withPerms("notes:read"), http.StatusForbidden},
		{"granted passes", RequirePermission(az, "notes:read"), withPerms("notes:read"), http.StatusOK},
		{"any: second perm passes", RequireAny(az, "notes:write", "notes:read"), withPerms("notes:read"), http.StatusOK},
		{"any: none held is 403", RequireAny(az, "a:b", "c:d"), withPerms("notes:read"), http.StatusForbidden},
		{"any: no claims is 401", RequireAny(az, "a:b", "c:d"), context.Background(), http.StatusUnauthorized},
		{"any: empty list is 403", RequireAny(az), withPerms("notes:read"), http.StatusForbidden},
		{
			"nil authorizer is 500", RequirePermission(nil, "notes:read"), withPerms("notes:read"),
			http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/notes", http.NoBody).WithContext(tt.ctx)
			tt.mw(okHandler()).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}
