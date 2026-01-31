package handler

import (
	"net/http"

	"github.com/biairmal/go-sdk/httpkit/response"
)

// Func is a function that handles a request and returns a response payload and an optional error.
type Func func(r *http.Request) (any, error)

// Handle converts a Func into an http.HandlerFunc.
// On error it uses StatusCodeFromError to set the status and writes the error envelope.
// On success it uses *response.Success HTTPStatusCode when present, otherwise 200.
func Handle(h Func) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := h(r)
		if err != nil {
			statusCode := StatusCodeFromError(err)
			WriteErrorResponse(w, statusCode, err)
			return
		}

		var statusCode int
		var payload any
		if succ, ok := data.(*response.Success); ok {
			statusCode = succ.HTTPStatusCode
			payload = succ.Data
		} else {
			statusCode = http.StatusOK
			payload = data
		}

		WriteSuccessResponse(w, statusCode, payload)
	}
}
