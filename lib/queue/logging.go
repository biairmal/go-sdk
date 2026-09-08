package queue

import (
	"context"

	"github.com/biairmal/go-sdk/lib/logger"
)

// logging logs every publish instead of sending it anywhere. A stand-in for
// a real broker in dev/tests.
type logging struct {
	log logger.Logger
}

// NewLogging builds a Publisher that logs topic + message length (never the
// payload itself — it may carry PII) and returns nil. log must not be nil.
func NewLogging(log logger.Logger) Publisher {
	return logging{log: log}
}

// Publish logs topic and len(message) and returns nil.
func (p logging) Publish(_ context.Context, topic string, message []byte, _ ...PublishOption) error {
	p.log.Info("queue: publish",
		logger.F("topic", topic),
		logger.F("message_bytes", len(message)),
	)
	return nil
}
