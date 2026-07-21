package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/ctxkit"
	"github.com/biairmal/go-sdk/lib/errorz"
)

// fakeValidator is a hand-rolled auth.Validator test double that records
// whether it was called and returns canned results.
type fakeValidator struct {
	called bool
	claims auth.Claims
	err    error
}

func (f *fakeValidator) Validate(context.Context, string) (auth.Claims, error) {
	f.called = true
	return f.claims, f.err
}

func newRequest(path, authHeader string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	if authHeader != "" {
		r.Header.Set(AuthorizationHeader, authHeader)
	}
	return r
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthPublicRouteBypassesValidator(t *testing.T) {
	v := &fakeValidator{}
	policy := auth.Policy{DefaultProtected: true, Rules: []auth.Rule{{Pattern: "/health", Public: true}}}
	mw := Auth(v, WithPolicy(policy))

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, newRequest("/health", ""))

	if v.called {
		t.Error("validator must not be called on a public route")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthMissingOrMalformedHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"absent header", ""},
		{"wrong scheme", "Basic abc"},
		{"bearer without token", "Bearer "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &fakeValidator{}
			rec := httptest.NewRecorder()
			Auth(v)(okHandler()).ServeHTTP(rec, newRequest("/api", tt.header))

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if v.called {
				t.Error("validator must not be called without a valid bearer token")
			}
		})
	}
}

func TestAuthValidatorErrorStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"unauthorized maps to 401", errorz.Unauthorized().WithMessage("bad token"), http.StatusUnauthorized},
		{"bad gateway maps to 502", errorz.BadGateway().WithMessage("identity down"), http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &fakeValidator{err: tt.err}
			rec := httptest.NewRecorder()
			Auth(v)(okHandler()).ServeHTTP(rec, newRequest("/api", "Bearer tok"))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestAuthSuccessInjectsClaims(t *testing.T) {
	v := &fakeValidator{claims: auth.NewClaims(map[string]any{"sub": "user-9", "roles": []string{"admin"}})}

	var gotSubject, gotUserID string
	downstream := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if c, ok := auth.ClaimsFromContext(r.Context()); ok {
			gotSubject = c.Subject()
		}
		gotUserID = ctxkit.UserID(r.Context())
	})

	rec := httptest.NewRecorder()
	Auth(v)(downstream).ServeHTTP(rec, newRequest("/api", "Bearer tok"))

	if gotSubject != "user-9" {
		t.Errorf("claims subject in context = %q, want user-9", gotSubject)
	}
	if gotUserID != "user-9" {
		t.Errorf("ctxkit.UserID = %q, want user-9", gotUserID)
	}
}

func TestAuthNilValidatorFailsClosed(t *testing.T) {
	rec := httptest.NewRecorder()
	Auth(nil)(okHandler()).ServeHTTP(rec, newRequest("/api", "Bearer tok"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (nil validator must fail closed)", rec.Code)
	}
}

func TestAuthDefaultPolicyProtectsEverything(t *testing.T) {
	v := &fakeValidator{}
	rec := httptest.NewRecorder()
	// No WithPolicy: every route is protected, so a tokenless request is 401.
	Auth(v)(okHandler()).ServeHTTP(rec, newRequest("/anything", ""))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
