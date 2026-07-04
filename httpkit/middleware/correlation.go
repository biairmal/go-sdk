package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/biairmal/go-sdk/ctxkit"
)

// CorrelationIDHeader is the HTTP header name for the correlation ID (incoming and outgoing).
const CorrelationIDHeader = "X-Correlation-Id"

// Correlation returns a middleware that injects a correlation ID into the context
// (via ctxkit.WithCorrelationID) and the response header. It reads X-Correlation-Id
// from the request if present; otherwise it generates a new random hex string.
//
// Unlike RequestID, which identifies a single hop, the correlation ID is meant to
// be propagated ACROSS service boundaries so one logical operation can be traced
// through every service it touches. httpkit/client forwards it on outbound calls.
func Correlation() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(CorrelationIDHeader)
			if id == "" {
				id = generateCorrelationID()
			}
			ctx := ctxkit.WithCorrelationID(r.Context(), id)
			w.Header().Set(CorrelationIDHeader, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func generateCorrelationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "corr-fallback"
	}
	return hex.EncodeToString(b)
}
