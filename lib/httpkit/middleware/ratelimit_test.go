package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/lib/ctxkit"
	"github.com/biairmal/go-sdk/lib/ratelimit"
)

// fakeLimiter is a hand-rolled ratelimit.Limiter test double that returns a
// canned result/error and records the key it was called with.
type fakeLimiter struct {
	result  ratelimit.Result
	err     error
	lastKey string
	calls   int
}

func (f *fakeLimiter) Allow(_ context.Context, key string) (ratelimit.Result, error) {
	f.calls++
	f.lastKey = key
	return f.result, f.err
}

func TestRateLimit_AllowedRequestPassesThroughWithHeaders(t *testing.T) {
	l := &fakeLimiter{result: ratelimit.Result{Allowed: true, Limit: 10, Remaining: 7}}
	mw := RateLimit(l, KeyByIP)

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get(RateLimitLimitHeader); got != "10" {
		t.Errorf("%s = %q, want %q", RateLimitLimitHeader, got, "10")
	}
	if got := rec.Header().Get(RateLimitRemainingHeader); got != "7" {
		t.Errorf("%s = %q, want %q", RateLimitRemainingHeader, got, "7")
	}
}

func TestRateLimit_DeniedRequestWrites429WithRetryAfter(t *testing.T) {
	l := &fakeLimiter{result: ratelimit.Result{Allowed: false, Limit: 10, RetryAfter: 3 * time.Second}}
	mw := RateLimit(l, KeyByIP)

	rec := httptest.NewRecorder()
	handlerCalled := false
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { handlerCalled = true })
	mw(inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", http.NoBody))

	if handlerCalled {
		t.Error("next handler must not run when the request is denied")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get(RetryAfterHeader); got != "3" {
		t.Errorf("%s = %q, want %q", RetryAfterHeader, got, "3")
	}
}

func TestRateLimit_BackendErrorFailsOpenByDefault(t *testing.T) {
	l := &fakeLimiter{err: errors.New("redis unreachable")}
	mw := RateLimit(l, KeyByIP)

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (fail-open)", rec.Code)
	}
}

func TestRateLimit_BackendErrorRejectsWithFailClosed(t *testing.T) {
	l := &fakeLimiter{err: errors.New("redis unreachable")}
	mw := RateLimit(l, KeyByIP, WithFailClosed())

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", http.NoBody))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (fail-closed)", rec.Code)
	}
}

func TestRateLimit_NilLimiterPassesThrough(t *testing.T) {
	mw := RateLimit(nil, KeyByIP)

	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRateLimit_NilKeyFuncDefaultsToKeyByIP(t *testing.T) {
	l := &fakeLimiter{result: ratelimit.Result{Allowed: true}}
	mw := RateLimit(l, nil)

	req := httptest.NewRequest(http.MethodGet, "/orders", http.NoBody)
	req.RemoteAddr = "203.0.113.1:1234"
	mw(okHandler()).ServeHTTP(httptest.NewRecorder(), req)

	if l.lastKey != "203.0.113.1:1234" {
		t.Errorf("key = %q, want the client IP", l.lastKey)
	}
}

func TestKeyByUser_FallsBackToIPWhenUnauthenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orders", http.NoBody)
	req.RemoteAddr = "203.0.113.1:1234"

	if got := KeyByUser(req); got != "203.0.113.1:1234" {
		t.Errorf("KeyByUser() = %q, want the client IP fallback", got)
	}
}

func TestKeyByUser_UsesAuthenticatedSubject(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orders", http.NoBody)
	req = req.WithContext(ctxkit.WithUserID(req.Context(), "user-42"))

	if got := KeyByUser(req); got != "user-42" {
		t.Errorf("KeyByUser() = %q, want %q", got, "user-42")
	}
}

func TestKeyByHeader_FallsBackToIPWhenHeaderAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orders", http.NoBody)
	req.RemoteAddr = "203.0.113.1:1234"

	if got := KeyByHeader("X-Api-Key")(req); got != "203.0.113.1:1234" {
		t.Errorf("KeyByHeader() = %q, want the client IP fallback", got)
	}
}

func TestKeyByHeader_UsesHeaderValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/orders", http.NoBody)
	req.Header.Set("X-Api-Key", "abc123")

	if got := KeyByHeader("X-Api-Key")(req); got != "abc123" {
		t.Errorf("KeyByHeader() = %q, want %q", got, "abc123")
	}
}
