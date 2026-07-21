package tracer

import (
	"strings"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Config holds the YAML-able tracer settings.
type Config struct {
	// ServiceName identifies this service in traces. Required.
	ServiceName string `mapstructure:"service_name"`
	// ServiceVersion identifies the running build. Default "0.0.0".
	ServiceVersion string `mapstructure:"service_version"`
	// Endpoint is the OTLP gRPC collector address, e.g. "localhost:4317". Required.
	Endpoint string `mapstructure:"endpoint"`
	// Insecure disables TLS on the OTLP gRPC connection (e.g. for a local collector).
	Insecure bool `mapstructure:"insecure"`
	// SampleRate is the fraction of traces sampled, 0..1. Default 1.0 (sample everything).
	SampleRate float64 `mapstructure:"sample_rate"`
}

// DefaultConfig returns a Config with sane defaults for everything except the
// required ServiceName and Endpoint, which have no safe default.
func DefaultConfig() Config {
	return Config{ServiceVersion: "0.0.0", SampleRate: 1.0}
}

// Validate checks that Config is well-formed.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ServiceName) == "" {
		return errorz.BadRequest().WithMessage("tracer: service_name is required")
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return errorz.BadRequest().WithMessage("tracer: endpoint is required")
	}
	if c.SampleRate < 0 || c.SampleRate > 1 {
		return errorz.BadRequest().WithMessage("tracer: sample_rate must be between 0 and 1")
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig() so
// callers (and YAML) only need to override what differs.
func withDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = def.ServiceVersion
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = def.SampleRate
	}
	return cfg
}
