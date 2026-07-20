package metrics

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/biairmal/go-sdk/errorz"
)

// namespacePattern matches valid Prometheus metric-name segments: a leading
// letter or underscore followed by letters, digits, or underscores.
var namespacePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Config holds the YAML-able metrics settings.
type Config struct {
	// Namespace prefixes every metric name, e.g. "orders" -> "orders_http_requests_total".
	// Empty means no prefix. When set, must match Prometheus metric-name rules.
	Namespace string `mapstructure:"namespace"`
	// HTTPBuckets are the histogram bucket boundaries (seconds) used for
	// http_request_duration_seconds. Default prometheus.DefBuckets.
	HTTPBuckets []float64 `mapstructure:"http_buckets"`
}

// DefaultConfig returns a Config with sane defaults: no namespace and
// Prometheus's default latency buckets.
func DefaultConfig() Config {
	return Config{HTTPBuckets: prometheus.DefBuckets}
}

// Validate checks that Config is well-formed.
func (c Config) Validate() error {
	if c.Namespace != "" && !namespacePattern.MatchString(c.Namespace) {
		return errorz.BadRequest().WithMessage(
			"metrics: namespace must match ^[a-zA-Z_][a-zA-Z0-9_]*$")
	}
	for _, b := range c.HTTPBuckets {
		if b <= 0 {
			return errorz.BadRequest().WithMessage("metrics: http_buckets values must be positive")
		}
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig() so
// callers (and YAML) only need to override what differs.
func withDefaults(cfg Config) Config {
	if len(cfg.HTTPBuckets) == 0 {
		cfg.HTTPBuckets = DefaultConfig().HTTPBuckets
	}
	return cfg
}
