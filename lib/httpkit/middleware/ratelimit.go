package middleware

import (
	"net/http"
	"strconv"

	"github.com/biairmal/go-sdk/lib/ctxkit"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/biairmal/go-sdk/lib/ratelimit"
)

// Standard rate-limit response headers.
const (
	RateLimitLimitHeader     = "X-RateLimit-Limit"
	RateLimitRemainingHeader = "X-RateLimit-Remaining"
	RetryAfterHeader         = "Retry-After"
)

// KeyFunc derives the rate-limit key for a request, e.g. by client IP,
// authenticated user, or a request header.
type KeyFunc func(r *http.Request) string

// KeyByIP derives the rate-limit key from the request's client IP, honoring
// X-Forwarded-For / X-Real-IP ahead of RemoteAddr (see clientIP).
func KeyByIP(r *http.Request) string {
	return clientIP(r)
}

// KeyByUser derives the rate-limit key from the authenticated subject
// (ctxkit.UserID), falling back to the client IP for unauthenticated
// requests. Place RateLimit after Auth in the chain so the user id is set.
func KeyByUser(r *http.Request) string {
	if id := ctxkit.UserID(r.Context()); id != "" {
		return id
	}
	return clientIP(r)
}

// KeyByHeader returns a KeyFunc that derives the rate-limit key from the
// named request header (e.g. an API key), falling back to the client IP when
// the header is absent.
func KeyByHeader(name string) KeyFunc {
	return func(r *http.Request) string {
		if v := r.Header.Get(name); v != "" {
			return v
		}
		return clientIP(r)
	}
}

// rateLimitOptions holds the configured RateLimit middleware settings.
type rateLimitOptions struct {
	failClosed bool
}

// RateLimitOption configures the RateLimit middleware.
type RateLimitOption func(*rateLimitOptions)

// WithFailClosed makes the middleware reject requests (503) when the
// Limiter backend itself errors (e.g. Redis unreachable), instead of the
// default fail-open behavior that lets the request through unlimited.
func WithFailClosed() RateLimitOption {
	return func(o *rateLimitOptions) { o.failClosed = true }
}

// RateLimit returns a middleware that limits requests via l, keyed by
// keyFn. Allowed requests get X-RateLimit-Limit/-Remaining response
// headers; denied requests get a 429 (errorz.TooManyRequests()) with
// Retry-After. A nil l or keyFn is safe: nil l passes every request
// through, nil keyFn defaults to KeyByIP.
//
// A non-nil error from l.Allow means the backend itself failed (e.g. Redis
// unreachable), not a normal rate-limit denial. That fails open by default
// (the request proceeds unlimited) unless WithFailClosed is set, in which
// case it's rejected with 503.
func RateLimit(l ratelimit.Limiter, keyFn KeyFunc, opts ...RateLimitOption) func(http.Handler) http.Handler {
	options := rateLimitOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if keyFn == nil {
		keyFn = KeyByIP
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if l == nil {
				next.ServeHTTP(w, r)
				return
			}
			result, err := l.Allow(r.Context(), keyFn(r))
			if err != nil {
				handleRateLimitBackendError(w, r, next, options.failClosed, err)
				return
			}
			setRateLimitHeaders(w, result)
			if !result.Allowed {
				writeTooManyRequests(w, result)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// handleRateLimitBackendError passes the request through (fail-open) or
// rejects it with 503 (fail-closed) when the Limiter backend itself errors.
func handleRateLimitBackendError(
	w http.ResponseWriter, r *http.Request, next http.Handler, failClosed bool, err error,
) {
	if !failClosed {
		next.ServeHTTP(w, r)
		return
	}
	wrapped := errorz.Wrap(err).WithCode(errorz.CodeServiceUnavailable).WithMessage("rate limit backend unavailable")
	handler.WriteErrorResponse(w, handler.StatusCodeFromError(wrapped), wrapped)
}

// setRateLimitHeaders writes the standard rate-limit visibility headers.
func setRateLimitHeaders(w http.ResponseWriter, result ratelimit.Result) {
	w.Header().Set(RateLimitLimitHeader, strconv.Itoa(result.Limit))
	w.Header().Set(RateLimitRemainingHeader, strconv.Itoa(result.Remaining))
}

// writeTooManyRequests writes the 429 response with a Retry-After header.
func writeTooManyRequests(w http.ResponseWriter, result ratelimit.Result) {
	w.Header().Set(RetryAfterHeader, strconv.Itoa(int(result.RetryAfter.Seconds())))
	err := errorz.TooManyRequests()
	handler.WriteErrorResponse(w, handler.StatusCodeFromError(err), err)
}
