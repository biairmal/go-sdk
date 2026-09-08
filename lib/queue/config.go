package queue

import (
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/kafka"
	"github.com/biairmal/go-sdk/lib/logger"
)

// Publisher backend selectors for Config.Backend.
const (
	BackendNoOp    = "noop"
	BackendLogging = "logging"
	BackendKafka   = "kafka"
)

// Config holds the YAML-able queue settings.
type Config struct {
	// Backend selects the Publisher implementation: "noop", "logging", or
	// "kafka".
	Backend string `mapstructure:"backend"`
	// Kafka is read only when Backend is "kafka".
	Kafka kafka.Config `mapstructure:"kafka"`
}

// DefaultConfig returns a Config with the noop backend, the safe default
// until a real broker is wired up.
func DefaultConfig() Config {
	return Config{Backend: BackendNoOp}
}

// Validate checks that Config is well-formed for its selected backend.
func (c *Config) Validate() error {
	switch c.Backend {
	case BackendNoOp, BackendLogging:
		return nil
	case BackendKafka:
		return c.Kafka.Validate()
	default:
		return errorz.BadRequest().WithMessage("queue: backend must be noop, logging or kafka")
	}
}

// FromConfig builds a Publisher from cfg, selecting the backend by
// cfg.Backend. log is required for "logging"; kafkaClient is required for
// "kafka" (mirrors ratelimit.FromConfig(cfg, redisClient)); both are ignored
// otherwise.
func FromConfig(cfg *Config, log logger.Logger, kafkaClient *kafka.Client) (Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	switch cfg.Backend {
	case BackendLogging:
		if log == nil {
			return nil, errorz.BadRequest().WithMessage("queue: logging backend requires a non-nil logger.Logger")
		}
		return NewLogging(log), nil
	case BackendKafka:
		if kafkaClient == nil {
			return nil, errorz.BadRequest().WithMessage("queue: kafka backend requires a non-nil *kafka.Client")
		}
		return NewKafka(kafkaClient), nil
	default:
		return NewNoOp(), nil
	}
}
