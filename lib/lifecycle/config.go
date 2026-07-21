package lifecycle

import (
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Config holds the YAML-able graceful-shutdown timing settings.
type Config struct {
	// DrainDelay is how long Run waits, after flipping readiness but before
	// calling srv.Shutdown, so a load balancer / k8s endpoint controller has
	// time to observe the readiness flip and stop routing new traffic before
	// the listener actually stops accepting connections. Interruptible by a
	// second shutdown signal.
	DrainDelay time.Duration `mapstructure:"drain_delay"`
	// ShutdownTimeout bounds how long srv.Shutdown may take to drain
	// in-flight HTTP requests before Run gives up on a graceful HTTP close.
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	// CloserTimeout bounds the entire closer phase (every registered Closer,
	// run sequentially in registration order) as a whole — independent of
	// how long ShutdownTimeout took, so a slow HTTP drain can't starve
	// resource cleanup down to zero.
	CloserTimeout time.Duration `mapstructure:"closer_timeout"`
}

// DefaultConfig returns a Config with sane defaults: a 5s drain delay, 15s to
// drain in-flight HTTP requests, and 15s for the closer phase — a ~35s
// worst-case wall time from signal to return. Tune per-service: a service
// behind a slow-propagating LB or with a slow DB pool drain should raise the
// relevant field independently rather than one global timeout.
func DefaultConfig() Config {
	return Config{
		DrainDelay:      5 * time.Second,
		ShutdownTimeout: 15 * time.Second,
		CloserTimeout:   15 * time.Second,
	}
}

// Validate checks that Config is well-formed. Run does not call this itself
// (it fills zero-valued fields from DefaultConfig() instead, so a zero
// Config is always usable) — call Validate yourself after loading a Config
// from YAML via config.Load, before rejecting a malformed one.
func (c Config) Validate() error {
	if c.DrainDelay < 0 {
		return errorz.BadRequest().WithMessage("lifecycle: drain_delay must not be negative")
	}
	if c.ShutdownTimeout <= 0 {
		return errorz.BadRequest().WithMessage("lifecycle: shutdown_timeout must be positive")
	}
	if c.CloserTimeout <= 0 {
		return errorz.BadRequest().WithMessage("lifecycle: closer_timeout must be positive")
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig() so
// callers (and YAML) only need to override what differs. A DrainDelay of
// exactly zero can't be distinguished from "unset" — callers who want no
// drain delay at all should set a small non-zero value (e.g. 1ms) instead.
func withDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.DrainDelay == 0 {
		cfg.DrainDelay = def.DrainDelay
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = def.ShutdownTimeout
	}
	if cfg.CloserTimeout == 0 {
		cfg.CloserTimeout = def.CloserTimeout
	}
	return cfg
}
