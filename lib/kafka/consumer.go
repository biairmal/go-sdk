package kafka

import (
	"context"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// dialTimeout bounds how long a Consumer's Dialer waits to establish a
// connection (kafka-go's Dialer has no default timeout otherwise).
const dialTimeout = 10 * time.Second

// Message is one delivery handed to a Consumer's handler.
type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

// Consumer reads messages from one Kafka topic under one consumer group
// (segmentio/kafka-go's Reader model: one Reader = one group + one topic's
// partitions). Reuses buildSASLMechanism/buildTLSConfig via a kafka.Dialer —
// Reader takes a Dialer, not a Transport like Writer.
type Consumer struct {
	reader *kafkago.Reader
}

// NewConsumer validates cfg and returns a Consumer bound to groupID and
// topic.
func NewConsumer(cfg *Config, groupID, topic string) (*Consumer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if groupID == "" {
		return nil, errorz.BadRequest().WithMessage("kafka: groupID must not be empty")
	}
	if topic == "" {
		return nil, errorz.BadRequest().WithMessage("kafka: topic must not be empty")
	}

	mechanism, err := buildSASLMechanism(&cfg.Auth)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := buildTLSConfig(&cfg.Auth)
	if err != nil {
		return nil, err
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: cfg.Brokers,
		GroupID: groupID,
		Topic:   topic,
		Dialer:  &kafkago.Dialer{Timeout: dialTimeout, SASLMechanism: mechanism, TLS: tlsConfig},
	})
	return &Consumer{reader: reader}, nil
}

// Consume blocks: Fetch → handler → Commit, one message at a time, in
// partition order, until ctx is done or handler/Fetch returns an error. A
// handler error stops the loop with that message left uncommitted — the
// caller decides whether to restart Consume; on restart the consumer group
// resumes from the last committed offset, redelivering it. Returns nil on
// clean ctx cancellation.
func (c *Consumer) Consume(ctx context.Context, handler func(ctx context.Context, msg Message) error) error {
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("kafka: fetch failed")
		}

		if err := handler(ctx, toMessage(&m)); err != nil {
			return err
		}

		if err := c.reader.CommitMessages(ctx, m); err != nil {
			return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("kafka: commit failed")
		}
	}
}

// Close releases the underlying connection. The Consumer must not be used
// afterward.
func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("kafka: close failed")
	}
	return nil
}

// toMessage converts a kafka-go Message to our Message, nil-safe on headers.
func toMessage(m *kafkago.Message) Message {
	var headers map[string]string
	if len(m.Headers) > 0 {
		headers = make(map[string]string, len(m.Headers))
		for _, h := range m.Headers {
			headers[h.Key] = string(h.Value)
		}
	}
	return Message{Topic: m.Topic, Key: m.Key, Value: m.Value, Headers: headers}
}
