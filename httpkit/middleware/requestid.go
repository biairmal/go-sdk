package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/biairmal/go-sdk/ctxkit"
)

// requestIDKey is the context key for the request ID.
// Use it with logger.ContextExtractor to include request_id in logs.
type requestIDKey struct{}

// RequestIDKey is the context key for the request ID value.
// Handlers or logger extractors can use it: ctx.Value(RequestIDKey).
var RequestIDKey = requestIDKey{}

// RequestIDHeader is the HTTP header name for the request ID (incoming and outgoing).
const RequestIDHeader = "X-Request-Id"

// RequestID returns a middleware that injects a request ID into the context
// and response header. It reads X-Request-Id from the request if present;
// otherwise it generates a new random hex string.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if id == "" {
				id = generateRequestID()
			}
			// Store under both the exported RequestIDKey (back-compat for existing
			// readers) and the canonical ctxkit key (picked up by ctxkit.LoggerExtractor).
			ctx := context.WithValue(r.Context(), RequestIDKey, id)
			ctx = ctxkit.WithRequestID(ctx, id)
			w.Header().Set(RequestIDHeader, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b)
}
