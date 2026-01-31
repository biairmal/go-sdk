package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain(t *testing.T) {
	var order []string
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}
	h := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		order = append(order, "handler")
	})
	chained := Chain(h, m1, m2)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	chained.ServeHTTP(w, req)
	want := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(want) {
		t.Errorf("order = %v, want %v", order, want)
	} else {
		for i := range want {
			if order[i] != want[i] {
				t.Errorf("order[%d] = %v, want %v", i, order[i], want[i])
			}
		}
	}
}
