// Package httpkit provides router-agnostic HTTP handler, client, middlewares,
// and response structures for use with net/http or go-chi.
package httpkit

import (
	"github.com/biairmal/go-sdk/httpkit/handler"
)

// StatusCodeFromError returns the HTTP status code for the given error.
// If the error is a *errorz.Error, its Code is looked up in the default map.
// Otherwise it returns http.StatusInternalServerError.
// This is the same as handler.StatusCodeFromError; it is re-exported here for convenience.
func StatusCodeFromError(err error) int {
	return handler.StatusCodeFromError(err)
}
