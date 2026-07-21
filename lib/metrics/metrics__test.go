package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"github.com/biairmal/go-sdk/lib/logger"
)

// recordingLogger wraps a no-op logger.Logger and counts Errorf calls, so
// tests can assert on internal diagnostics without depending on log output.
type recordingLogger struct {
	logger.Logger
	errCount int
}

func (l *recordingLogger) Errorf(_ string, _ ...any) {
	l.errCount++
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "empty config is valid", cfg: Config{}},
		{name: "valid namespace", cfg: Config{Namespace: "orders"}},
		{name: "namespace starting with digit", cfg: Config{Namespace: "1bad"}, wantErr: true},
		{name: "namespace with dash", cfg: Config{Namespace: "bad-ns"}, wantErr: true},
		{name: "valid buckets", cfg: Config{HTTPBuckets: []float64{0.1, 0.5, 1}}},
		{name: "zero bucket", cfg: Config{HTTPBuckets: []float64{0}}, wantErr: true},
		{name: "negative bucket", cfg: Config{HTTPBuckets: []float64{0.1, -1}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestWithDefaults(t *testing.T) {
	cfg := withDefaults(Config{})
	if len(cfg.HTTPBuckets) != len(prometheus.DefBuckets) {
		t.Errorf("withDefaults() HTTPBuckets = %v, want %v", cfg.HTTPBuckets, prometheus.DefBuckets)
	}

	custom := []float64{1, 2, 3}
	cfg = withDefaults(Config{HTTPBuckets: custom})
	if len(cfg.HTTPBuckets) != len(custom) {
		t.Errorf("withDefaults() overrode explicit HTTPBuckets = %v, want %v", cfg.HTTPBuckets, custom)
	}
}

func TestSortedKeys(t *testing.T) {
	if got := sortedKeys(nil); got != nil {
		t.Errorf("sortedKeys(nil) = %v, want nil", got)
	}

	got := sortedKeys(Labels{"b": "2", "a": "1", "c": "3"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("sortedKeys() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sortedKeys()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNoOpRecorder(_ *testing.T) {
	rec := NewNoOp()
	rec.CounterInc("x", Labels{"a": "b"})
	rec.HistogramObserve("x", 1.5, Labels{"a": "b"})
	rec.GaugeAdd("x", -1, nil)
}

func TestWithRegisterer_NilIsNoop(t *testing.T) {
	orig := prometheus.NewRegistry()
	rec := &prometheusRecorder{registerer: orig}
	WithRegisterer(nil)(rec)
	if rec.registerer != orig {
		t.Error("WithRegisterer(nil) changed the registerer")
	}
}

func TestWithLogger(t *testing.T) {
	rec := &prometheusRecorder{}
	log := logger.NewNoOp()
	WithLogger(log)(rec)
	if rec.log != log {
		t.Error("WithLogger() did not set log")
	}
}

func TestNewPrometheus_InvalidConfig(t *testing.T) {
	if _, err := NewPrometheus(Config{Namespace: "bad-ns"}); err == nil {
		t.Fatal("NewPrometheus() with invalid config = nil error, want error")
	}
}

func TestNewPrometheus_RegistrationConflict(t *testing.T) {
	reg := prometheus.NewRegistry()
	conflict := prometheus.NewCounterVec(prometheus.CounterOpts{Name: HTTPRequestsTotal}, httpLabelKeys)
	if err := reg.Register(conflict); err != nil {
		t.Fatalf("setup Register() = %v, want nil", err)
	}

	if _, err := NewPrometheus(Config{}, WithRegisterer(reg)); err == nil {
		t.Fatal("NewPrometheus() with a name conflict = nil error, want error")
	}
}

func TestNewPrometheus_RegistersHTTPMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	rec, err := NewPrometheus(Config{Namespace: "svc"}, WithRegisterer(reg))
	if err != nil {
		t.Fatalf("NewPrometheus() = %v, want nil", err)
	}

	pr, ok := rec.(*prometheusRecorder)
	if !ok {
		t.Fatalf("NewPrometheus() returned %T, want *prometheusRecorder", rec)
	}
	if _, ok := pr.counters[HTTPRequestsTotal]; !ok {
		t.Error("http_requests_total counter was not pre-registered")
	}
	if _, ok := pr.histograms[HTTPRequestDuration]; !ok {
		t.Error("http_request_duration_seconds histogram was not pre-registered")
	}
	if _, ok := pr.gauges[HTTPRequestsInFlight]; !ok {
		t.Error("http_requests_in_flight gauge was not pre-registered")
	}
}

func TestPrometheusRecorder_CounterInc(t *testing.T) {
	reg := prometheus.NewRegistry()
	rec, err := NewPrometheus(Config{}, WithRegisterer(reg))
	if err != nil {
		t.Fatalf("NewPrometheus() = %v, want nil", err)
	}

	labels := Labels{"method": "GET", "path": "/x", "status_code": "200"}
	rec.CounterInc(HTTPRequestsTotal, labels)
	rec.CounterInc(HTTPRequestsTotal, labels)

	pr := rec.(*prometheusRecorder)
	metric, err := pr.counters[HTTPRequestsTotal].GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		t.Fatalf("GetMetricWith() = %v, want nil", err)
	}
	if got := testutil.ToFloat64(metric); got != 2 {
		t.Errorf("counter value = %v, want 2", got)
	}
}

func TestPrometheusRecorder_HistogramObserve(t *testing.T) {
	reg := prometheus.NewRegistry()
	rec, err := NewPrometheus(Config{}, WithRegisterer(reg))
	if err != nil {
		t.Fatalf("NewPrometheus() = %v, want nil", err)
	}

	labels := Labels{"method": "GET", "path": "/x", "status_code": "200"}
	rec.HistogramObserve(HTTPRequestDuration, 0.2, labels)
	rec.HistogramObserve(HTTPRequestDuration, 0.4, labels)

	pr := rec.(*prometheusRecorder)
	observer, err := pr.histograms[HTTPRequestDuration].GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		t.Fatalf("GetMetricWith() = %v, want nil", err)
	}

	var m dto.Metric
	metric, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatalf("observer is %T, want prometheus.Metric", observer)
	}
	if err := metric.Write(&m); err != nil {
		t.Fatalf("Write() = %v, want nil", err)
	}
	if got := m.GetHistogram().GetSampleCount(); got != 2 {
		t.Errorf("sample count = %d, want 2", got)
	}
}

func TestPrometheusRecorder_GaugeAdd(t *testing.T) {
	reg := prometheus.NewRegistry()
	rec, err := NewPrometheus(Config{}, WithRegisterer(reg))
	if err != nil {
		t.Fatalf("NewPrometheus() = %v, want nil", err)
	}

	rec.GaugeAdd(HTTPRequestsInFlight, 3, nil)
	rec.GaugeAdd(HTTPRequestsInFlight, -1, nil)

	pr := rec.(*prometheusRecorder)
	metric, err := pr.gauges[HTTPRequestsInFlight].GetMetricWith(nil)
	if err != nil {
		t.Fatalf("GetMetricWith() = %v, want nil", err)
	}
	if got := testutil.ToFloat64(metric); got != 2 {
		t.Errorf("gauge value = %v, want 2", got)
	}
}

func TestPrometheusRecorder_LabelMismatchLogsError(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeLog := &recordingLogger{Logger: logger.NewNoOp()}
	rec, err := NewPrometheus(Config{}, WithRegisterer(reg), WithLogger(fakeLog))
	if err != nil {
		t.Fatalf("NewPrometheus() = %v, want nil", err)
	}

	rec.CounterInc("custom_total", Labels{"a": "1"})
	rec.CounterInc("custom_total", Labels{"b": "1"}) // different label keys -> GetMetricWith error

	if fakeLog.errCount == 0 {
		t.Error("expected the recorder to log a label-mismatch error")
	}
}

func TestPrometheusRecorder_DynamicRegistrationConflictLogsError(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeLog := &recordingLogger{Logger: logger.NewNoOp()}
	rec, err := NewPrometheus(Config{}, WithRegisterer(reg), WithLogger(fakeLog))
	if err != nil {
		t.Fatalf("NewPrometheus() = %v, want nil", err)
	}

	// Pre-register a raw collector under the same name so the recorder's own
	// lazy registration of "custom_total" collides with it.
	conflict := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "custom_total"}, []string{"a"})
	if err := reg.Register(conflict); err != nil {
		t.Fatalf("setup Register() = %v, want nil", err)
	}

	rec.CounterInc("custom_total", Labels{"a": "1"})

	if fakeLog.errCount == 0 {
		t.Error("expected the recorder to log a registration error for a name conflict")
	}
}
