package ratelimit

import (
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/redis"
)

// Rate-limit backend selectors for Config.Backend.
const (
	BackendMemory = "memory"
	BackendRedis  = "redis"
)

// Config holds the YAML-able rate-limit settings, shared by both backends.
// Which fields apply depends on Backend: "memory" is a continuous
// per-process token bucket using Rate/Burst/MaxKeys; "redis" is a sliding
// window shared across every process, using Burst (the window's request
// cap) and Window (the window length).
type Config struct {
	// Backend selects the Limiter implementation: "memory" or "redis".
	Backend string `mapstructure:"backend"`
	// Rate is the sustained permits per second. Used by the memory backend's
	// token-bucket refill rate; ignored by the redis backend.
	Rate float64 `mapstructure:"rate"`
	// Burst is the maximum permits available at once. Memory backend: the
	// token bucket's capacity. Redis backend: the request cap within Window.
	Burst int `mapstructure:"burst"`
	// Window is the sliding-window length used by the redis backend.
	// Ignored by the memory backend.
	Window time.Duration `mapstructure:"window"`
	// MaxKeys bounds the memory backend's per-key bucket map; once reached,
	// idle keys are evicted to make room. Ignored by the redis backend.
	MaxKeys int `mapstructure:"max_keys"`
}

// DefaultConfig returns a Config with sane defaults: in-memory backend, 10
// permits/sec with a burst of 20, a 1-second redis window, capped at 10k
// tracked keys.
func DefaultConfig() Config {
	return Config{
		Backend: BackendMemory,
		Rate:    10,
		Burst:   20,
		Window:  time.Second,
		MaxKeys: 10_000,
	}
}

// Validate checks that Config is well-formed for its selected backend.
func (c Config) Validate() error {
	switch c.Backend {
	case BackendMemory, BackendRedis:
	default:
		return errorz.BadRequest().WithMessage("ratelimit: backend must be memory or redis")
	}
	if c.Burst <= 0 {
		return errorz.BadRequest().WithMessage("ratelimit: burst must be positive")
	}
	if c.Backend == BackendMemory {
		return validateMemory(c)
	}
	return validateRedis(c)
}

// validateMemory checks the memory backend's settings.
func validateMemory(c Config) error {
	if c.Rate <= 0 {
		return errorz.BadRequest().WithMessage("ratelimit: rate must be positive for the memory backend")
	}
	if c.MaxKeys <= 0 {
		return errorz.BadRequest().WithMessage("ratelimit: max_keys must be positive for the memory backend")
	}
	return nil
}

// validateRedis checks the redis backend's settings.
func validateRedis(c Config) error {
	if c.Window <= 0 {
		return errorz.BadRequest().WithMessage("ratelimit: window must be positive for the redis backend")
	}
	return nil
}

// withDefaults fills zero-valued fields of cfg from DefaultConfig() so
// callers (and YAML) only need to override what differs.
func withDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.Backend == "" {
		cfg.Backend = def.Backend
	}
	if cfg.Rate == 0 {
		cfg.Rate = def.Rate
	}
	if cfg.Burst == 0 {
		cfg.Burst = def.Burst
	}
	if cfg.Window == 0 {
		cfg.Window = def.Window
	}
	if cfg.MaxKeys == 0 {
		cfg.MaxKeys = def.MaxKeys
	}
	return cfg
}

// FromConfig builds a Limiter from cfg, selecting the backend by
// cfg.Backend. redisClient is required (non-nil) when cfg.Backend is
// "redis"; it's ignored for "memory".
func FromConfig(cfg Config, redisClient redis.Client) (Limiter, error) {
	cfg = withDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if cfg.Backend == BackendRedis {
		if redisClient == nil {
			return nil, errorz.BadRequest().WithMessage("ratelimit: redis backend requires a non-nil redis.Client")
		}
		return NewRedis(redisClient, cfg), nil
	}
	return NewInMemory(cfg), nil
}
