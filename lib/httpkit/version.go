package httpkit

import (
	"encoding/json"
	"net/http"
)

// Version returns a handler that always responds with 200 OK and the given
// version string as JSON body {"version":"<version>"}.
func Version(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(versionPayload{Version: version}); err != nil {
			// Header already sent; cannot return error to client.
			return
		}
	}
}

type versionPayload struct {
	Version string `json:"version"`
}
