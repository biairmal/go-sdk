package circuitbreaker

import (
	"time"

	"github.com/biairmal/go-sdk/errorz"
)

// Config holds the YAML-able circuit breaker settings.
type Config struct {
	// FailureThreshold trips the breaker to Open once this many consecutive
	// failures are recorded in the Closed state. It also doubles as the
	// minimum number of requests observed before FailureRatio is evaluated,
	// so the ratio rule never trips on a handful of unlucky calls.
	FailureThreshold int `mapstructure:"failure_threshold"`
	// FailureRatio trips the breaker to Open when the failure rate over all
	// requests recorded since the breaker last closed reaches this value
	// (0, 1], once at least FailureThreshold requests have been recorded.
	FailureRatio float64 `mapstructure:"failure_ratio"`
	// OpenTimeout is how long the breaker stays Open before allowing a
	// single Half-Open probe call through.
	OpenTimeout time.Duration `mapstructure:"open_timeout"`
	// HalfOpenMaxCalls is both the number of concurrent probe calls allowed
	// while Half-Open and the number of consecutive probe successes required
	// to close the breaker again. Any single probe failure reopens it.
	HalfOpenMaxCalls int `mapstructure:"half_open_max_calls"`
}

// DefaultConfig returns a Config with sane defaults: 5 consecutive failures
// or a 50% failure rate (over at least 5 requests) trips the breaker; it
// stays Open for 30s, then allows 1 Half-Open probe at a time.
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		FailureRatio:     0.5,
		OpenTimeout:      30 * time.Second,
		HalfOpenMaxCalls: 1,
	}
}

// Validate checks that Config is well-formed. NewBreaker does not call this
// itself (it fills zero-valued fields from DefaultConfig() instead, so a
// zero Config is always usable) — call Validate yourself after loading a
// Config from YAML via config.Load, before rejecting a malformed one.
func (c Config) Validate() error {
	if c.FailureThreshold <= 0 {
		return errorz.BadRequest().WithMessage("circuitbreaker: failure_threshold must be positive")
	}
	if c.FailureRatio <= 0 || c.FailureRatio > 1 {
		return errorz.BadRequest().WithMessage("circuitbreaker: failure_ratio must be in (0, 1]")
	}
	if c.OpenTimeout <= 0 {
		return errorz.BadRequest().WithMessage("circuitbreaker: open_timeout must be positive")
	}
	if c.HalfOpenMaxCalls <= 0 {
		return errorz.BadRequest().WithMessage("circuitbreaker: half_open_max_calls must be positive")
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig() so
// callers (and YAML) only need to override what differs.
func withDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = def.FailureThreshold
	}
	if cfg.FailureRatio == 0 {
		cfg.FailureRatio = def.FailureRatio
	}
	if cfg.OpenTimeout == 0 {
		cfg.OpenTimeout = def.OpenTimeout
	}
	if cfg.HalfOpenMaxCalls == 0 {
		cfg.HalfOpenMaxCalls = def.HalfOpenMaxCalls
	}
	return cfg
}
