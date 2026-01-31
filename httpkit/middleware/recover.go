package middleware

import (
	"net/http"

	"github.com/biairmal/go-sdk/httpkit/handler"
)

// Recover returns a middleware that recovers from panics and writes
// a 500 error response using the httpkit error envelope.
func Recover() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					handler.WriteErrorResponse(w, http.StatusInternalServerError, v)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
