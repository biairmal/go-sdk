package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biairmal/go-sdk/lib/metrics"
)

// recordedMetric captures a single Recorder call for test assertions.
type recordedMetric struct {
	kind   string // "counter", "histogram", or "gauge"
	name   string
	value  float64
	labels metrics.Labels
}

// fakeRecorder is a hand-rolled metrics.Recorder test double that records
// every call so tests can assert on them without a real Prometheus registry.
type fakeRecorder struct {
	calls []recordedMetric
}

func (f *fakeRecorder) CounterInc(name string, labels metrics.Labels) {
	f.calls = append(f.calls, recordedMetric{kind: "counter", name: name, value: 1, labels: labels})
}

func (f *fakeRecorder) HistogramObserve(name string, value float64, labels metrics.Labels) {
	f.calls = append(f.calls, recordedMetric{kind: "histogram", name: name, value: value, labels: labels})
}

func (f *fakeRecorder) GaugeAdd(name string, delta float64, labels metrics.Labels) {
	f.calls = append(f.calls, recordedMetric{kind: "gauge", name: name, value: delta, labels: labels})
}

func (f *fakeRecorder) findByKind(kind string) []recordedMetric {
	var out []recordedMetric
	for _, c := range f.calls {
		if c.kind == kind {
			out = append(out, c)
		}
	}
	return out
}

func TestMetrics_RecordsInFlightGauge(t *testing.T) {
	rec := &fakeRecorder{}
	handler := Metrics(rec, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	gauges := rec.findByKind("gauge")
	if len(gauges) != 2 {
		t.Fatalf("gauge calls = %d, want 2 (entry + exit)", len(gauges))
	}
	if gauges[0].value != 1 || gauges[1].value != -1 {
		t.Errorf("gauge deltas = %v, %v, want 1, -1", gauges[0].value, gauges[1].value)
	}
	for _, g := range gauges {
		if g.name != metrics.HTTPRequestsInFlight {
			t.Errorf("gauge name = %q, want %q", g.name, metrics.HTTPRequestsInFlight)
		}
	}
}

func TestMetrics_RecordsCounterAndHistogram(t *testing.T) {
	rec := &fakeRecorder{}
	handler := Metrics(rec, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/orders", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	counters := rec.findByKind("counter")
	if len(counters) != 1 {
		t.Fatalf("counter calls = %d, want 1", len(counters))
	}
	wantLabels := metrics.Labels{"method": http.MethodPost, "path": "/orders", "status_code": "201"}
	if !labelsEqual(counters[0].labels, wantLabels) {
		t.Errorf("counter labels = %v, want %v", counters[0].labels, wantLabels)
	}

	histograms := rec.findByKind("histogram")
	if len(histograms) != 1 {
		t.Fatalf("histogram calls = %d, want 1", len(histograms))
	}
	if !labelsEqual(histograms[0].labels, wantLabels) {
		t.Errorf("histogram labels = %v, want %v", histograms[0].labels, wantLabels)
	}
	if histograms[0].value < 0 {
		t.Errorf("histogram duration = %v, want >= 0", histograms[0].value)
	}
}

func TestMetrics_DefaultsToOK_WhenHandlerDoesNotWriteHeader(t *testing.T) {
	rec := &fakeRecorder{}
	handler := Metrics(rec, nil)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	counters := rec.findByKind("counter")
	if len(counters) != 1 {
		t.Fatalf("counter calls = %d, want 1", len(counters))
	}
	if counters[0].labels["status_code"] != "200" {
		t.Errorf("status_code label = %q, want %q", counters[0].labels["status_code"], "200")
	}
}

func TestMetrics_UsesCustomPathNormalizer(t *testing.T) {
	rec := &fakeRecorder{}
	opts := &MetricsOptions{
		PathNormalizer: func(_ *http.Request) string { return "/orders/{id}" },
	}
	handler := Metrics(rec, opts)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/orders/42", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	counters := rec.findByKind("counter")
	if len(counters) != 1 {
		t.Fatalf("counter calls = %d, want 1", len(counters))
	}
	if got := counters[0].labels["path"]; got != "/orders/{id}" {
		t.Errorf("path label = %q, want %q", got, "/orders/{id}")
	}
}

// labelsEqual compares two metrics.Labels maps for equality.
func labelsEqual(a, b metrics.Labels) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
