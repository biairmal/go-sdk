package httpkit

import (
	"encoding/json"
	"net/http"
)

// Health returns a handler that always responds with 200 OK (liveness).
// The response body is optional; a minimal JSON body like {"status":"ok"} is written
// for compatibility with probes that expect a body.
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(healthPayload{Status: "ok"}); err != nil {
			// Header already sent; cannot return error to client.
			return
		}
	}
}

type healthPayload struct {
	Status string `json:"status"`
}
