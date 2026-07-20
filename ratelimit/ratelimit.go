// Package ratelimit provides interface-first rate limiting: a Limiter checks
// whether a keyed request is allowed under a configured rate. Two backends
// ship: an in-memory token bucket (NewInMemory) for a single monolith
// instance, and a Redis-backed sliding window (NewRedis) that shares one
// limit across every process hitting the same Redis, for scale-out.
//
// httpkit/middleware.RateLimit is the standard HTTP integration: it derives
// a key per request (KeyByIP, KeyByUser, KeyByHeader), calls Limiter.Allow,
// sets the standard rate-limit response headers, and writes a 429 on denial.
//
// Example usage:
//
//	l := ratelimit.NewInMemory(ratelimit.Config{Rate: 10, Burst: 20})
//	result, err := l.Allow(ctx, "user:42")
//	if err == nil && !result.Allowed {
//		// deny, retry after result.RetryAfter
//	}
package ratelimit

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../mocks/ratelimit/mock_ratelimit.go -package=mockratelimit github.com/biairmal/go-sdk/ratelimit Limiter

import (
	"context"
	"time"
)

// Result carries the outcome of a rate-limit check, enough for a caller (or
// httpkit/middleware.RateLimit) to set standard rate-limit response headers.
type Result struct {
	// Allowed reports whether the request may proceed.
	Allowed bool
	// Limit is the configured capacity: the memory backend's burst size, or
	// the redis backend's per-window request cap.
	Limit int
	// Remaining is the number of permits left. 0 when denied.
	Remaining int
	// RetryAfter is how long the caller should wait before retrying. Zero
	// when Allowed is true.
	RetryAfter time.Duration
}

// Limiter checks whether a request identified by key is allowed under the
// configured rate. Implementations must be safe for concurrent use.
type Limiter interface {
	// Allow reports whether a request for key is permitted right now,
	// consuming one permit if so. A non-nil error means the backend itself
	// could not be reached (e.g. Redis unavailable) — the Result is
	// meaningless in that case; callers decide the fail-open/closed policy.
	// httpkit/middleware.RateLimit fails open by default.
	Allow(ctx context.Context, key string) (Result, error)
}
