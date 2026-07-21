package metrics

import (
	"fmt"
	"sort"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
)

// httpLabelKeys are the label keys pre-registered for the standard HTTP
// counter and histogram. Already alphabetically sorted, matching how
// dynamically-created vecs order their label keys.
var httpLabelKeys = []string{"method", "path", "status_code"}

// prometheusRecorder implements Recorder using the Prometheus client. Metric
// vectors are created lazily on first use (except the standard HTTP metrics,
// pre-registered at construction) and cached by name for reuse.
type prometheusRecorder struct {
	registerer prometheus.Registerer
	namespace  string
	buckets    []float64
	log        logger.Logger

	mu         sync.RWMutex
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec
}

// NewPrometheus builds a Recorder backed by the Prometheus client. Cfg
// carries the YAML-able knobs; zero-valued fields fall back to
// DefaultConfig(). Opts inject an optional prometheus.Registerer (default
// prometheus.DefaultRegisterer) and logger.Logger.
//
// NewPrometheus eagerly registers the standard HTTP metrics (HTTPRequestsTotal,
// HTTPRequestDuration, HTTPRequestsInFlight) so httpkit/middleware.Metrics
// never pays a lazy-registration cost on the first request.
func NewPrometheus(cfg Config, opts ...Option) (Recorder, error) {
	cfg = withDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	rec := &prometheusRecorder{
		registerer: prometheus.DefaultRegisterer,
		namespace:  cfg.Namespace,
		buckets:    cfg.HTTPBuckets,
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(rec)
		}
	}

	if err := rec.registerHTTPMetrics(); err != nil {
		return nil, err
	}
	return rec, nil
}

// registerHTTPMetrics pre-registers the standard HTTP metrics.
func (r *prometheusRecorder) registerHTTPMetrics() error {
	if _, err := r.counterVec(HTTPRequestsTotal, httpLabelKeys); err != nil {
		return err
	}
	if _, err := r.histogramVec(HTTPRequestDuration, httpLabelKeys); err != nil {
		return err
	}
	if _, err := r.gaugeVec(HTTPRequestsInFlight, nil); err != nil {
		return err
	}
	return nil
}

// CounterInc increments the named counter, creating it on first use.
func (r *prometheusRecorder) CounterInc(name string, labels Labels) {
	vec, err := r.counterVec(name, sortedKeys(labels))
	if err != nil {
		r.logError(err)
		return
	}
	metric, err := vec.GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		r.logError(fmt.Errorf("metrics: counter %s: %w", name, err))
		return
	}
	metric.Inc()
}

// HistogramObserve records value for the named histogram, creating it on first use.
func (r *prometheusRecorder) HistogramObserve(name string, value float64, labels Labels) {
	vec, err := r.histogramVec(name, sortedKeys(labels))
	if err != nil {
		r.logError(err)
		return
	}
	metric, err := vec.GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		r.logError(fmt.Errorf("metrics: histogram %s: %w", name, err))
		return
	}
	metric.Observe(value)
}

// GaugeAdd adds delta to the named gauge, creating it on first use.
func (r *prometheusRecorder) GaugeAdd(name string, delta float64, labels Labels) {
	vec, err := r.gaugeVec(name, sortedKeys(labels))
	if err != nil {
		r.logError(err)
		return
	}
	metric, err := vec.GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		r.logError(fmt.Errorf("metrics: gauge %s: %w", name, err))
		return
	}
	metric.Add(delta)
}

// counterVec returns the cached CounterVec for name, registering a new one
// with labelKeys if this is the first use.
func (r *prometheusRecorder) counterVec(name string, labelKeys []string) (*prometheus.CounterVec, error) {
	return getOrCreateVec(&r.mu, r.counters, name, r.registerer, func() *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: r.namespace, Name: name}, labelKeys)
	})
}

// histogramVec returns the cached HistogramVec for name, registering a new
// one with labelKeys and the configured buckets if this is the first use.
func (r *prometheusRecorder) histogramVec(name string, labelKeys []string) (*prometheus.HistogramVec, error) {
	return getOrCreateVec(&r.mu, r.histograms, name, r.registerer, func() *prometheus.HistogramVec {
		return prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: r.namespace, Name: name, Buckets: r.buckets,
		}, labelKeys)
	})
}

// gaugeVec returns the cached GaugeVec for name, registering a new one with
// labelKeys if this is the first use.
func (r *prometheusRecorder) gaugeVec(name string, labelKeys []string) (*prometheus.GaugeVec, error) {
	return getOrCreateVec(&r.mu, r.gauges, name, r.registerer, func() *prometheus.GaugeVec {
		return prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: r.namespace, Name: name}, labelKeys)
	})
}

// getOrCreateVec returns the cached collector for name, or builds one via
// build, registers it against registerer, and caches it under mu. Shared by
// counterVec/histogramVec/gaugeVec so the register-on-first-use logic is
// written once regardless of the concrete vec type.
func getOrCreateVec[T prometheus.Collector](
	mu *sync.RWMutex, cache map[string]T, name string, registerer prometheus.Registerer, build func() T,
) (T, error) {
	mu.RLock()
	vec, ok := cache[name]
	mu.RUnlock()
	if ok {
		return vec, nil
	}

	mu.Lock()
	defer mu.Unlock()
	if vec, ok := cache[name]; ok {
		return vec, nil
	}

	vec = build()
	if err := registerer.Register(vec); err != nil {
		var zero T
		return zero, errorz.Wrap(err).WithCode(errorz.CodeInternal).
			WithMessage(fmt.Sprintf("metrics: register %s", name))
	}
	cache[name] = vec
	return vec, nil
}

// logError logs err via the optional logger. Nil-safe.
func (r *prometheusRecorder) logError(err error) {
	if r.log == nil {
		return
	}
	r.log.Errorf("%v", err)
}

// sortedKeys returns labels' keys in sorted order, so metric vecs created
// for the same name always declare label names in a deterministic order.
func sortedKeys(labels Labels) []string {
	if len(labels) == 0 {
		return nil
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
