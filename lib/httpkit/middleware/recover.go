package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/biairmal/go-sdk/lib/logger"
)

// Recover returns a middleware that recovers from panics, logs the panic value
// and stack at error level, and writes a generic 500 error envelope. The panic
// value never reaches the client. A nil log disables the logging.
func Recover(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if log != nil {
						log.ErrorWithContext(r.Context(), "http panic recovered",
							logger.F("panic", v),
							logger.F("method", r.Method),
							logger.F("path", r.URL.Path),
							logger.F("stack", string(debug.Stack())),
						)
					}
					handler.WriteErrorResponse(w, http.StatusInternalServerError, v)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
