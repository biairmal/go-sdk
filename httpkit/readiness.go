package httpkit

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/biairmal/go-sdk/httpkit/handler"
)

// Readiness returns a handler that runs the given check.
// If check returns nil, the handler responds with 200 OK.
// If check returns a non-nil error, the handler responds with 503 Service Unavailable
// and writes the same error envelope format as the rest of httpkit.
func Readiness(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(readinessPayload{Status: "ready"}); err != nil {
				return
			}
			return
		}
		err := check(r.Context())
		if err != nil {
			handler.WriteErrorResponse(w, http.StatusServiceUnavailable, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(readinessPayload{Status: "ready"}); err != nil {
			return
		}
	}
}

type readinessPayload struct {
	Status string `json:"status"`
}
