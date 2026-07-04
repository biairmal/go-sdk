package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/ctxkit"
)

func TestCorrelation(t *testing.T) {
	tests := []struct {
		name         string
		incomingID   string
		wantEchoedID string // exact value expected in response header; "" means "generated, non-empty"
	}{
		{name: "preserves incoming correlation id", incomingID: "corr-abc", wantEchoedID: "corr-abc"},
		{name: "generates when header absent", incomingID: "", wantEchoedID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctxID string
			handler := Correlation()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ctxID = ctxkit.CorrelationID(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.incomingID != "" {
				req.Header.Set(CorrelationIDHeader, tt.incomingID)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			echoed := w.Header().Get(CorrelationIDHeader)
			if echoed == "" {
				t.Fatal("response is missing the correlation id header")
			}
			if ctxID != echoed {
				t.Errorf("context id %q does not match echoed header %q", ctxID, echoed)
			}
			if tt.wantEchoedID != "" && echoed != tt.wantEchoedID {
				t.Errorf("echoed id = %q, want %q", echoed, tt.wantEchoedID)
			}
			if tt.wantEchoedID == "" && echoed == "corr-fallback" {
				t.Error("expected a generated id, got the fallback value")
			}
		})
	}
}
