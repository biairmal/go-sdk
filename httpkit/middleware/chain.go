// Package middleware provides router-agnostic HTTP middlewares that implement
// func(http.Handler) http.Handler for use with net/http or go-chi.
package middleware

import "net/http"

// Chain returns a handler that runs the given middlewares in order.
// The first element of m is the outermost layer (runs first on request, last on response).
func Chain(h http.Handler, m ...func(http.Handler) http.Handler) http.Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}
