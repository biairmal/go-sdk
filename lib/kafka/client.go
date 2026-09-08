// Package kafka wraps connection and authentication to an Apache Kafka
// cluster: a Client built from Config produces messages to any topic. It has
// no messaging semantics of its own (no topic abstraction, no
// publish/subscribe interface) — that seam lives one layer up, in queue,
// which depends on this package the way ratelimit depends on redis.
//
// Client is a concrete type, not an interface: there's nothing to swap
// underneath it (same reasoning as crypto.Encryptor), so it ships no mock.
// queue.Publisher is the interface consumers and tests use instead.
package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Client produces messages to a Kafka cluster over one shared connection
// (SASL + TLS built once from Config, into a *kafkago.Transport). Exports
// only what queue's Kafka backend needs; a Reader-backed consumer would be a
// new type in this package, not a new package, reusing buildSASLMechanism /
// buildTLSConfig.
type Client struct {
	writer *kafkago.Writer
}

// Option configures New with optional, non-serializable behavior. None
// exist yet — added if a caller ever needs to override transport/writer
// internals config can't express.
type Option func(*Client)

// New validates cfg, resolves cfg.Auth to a SASL mechanism and TLS config
// once, and returns a Client backed by a shared kafka.Writer/Transport. The
// per-message Topic field is why one Client can publish to any topic
// (queue.Publisher.Publish names it), instead of being pinned to one topic
// at construction.
func New(cfg *Config, opts ...Option) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	mechanism, err := buildSASLMechanism(&cfg.Auth)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := buildTLSConfig(&cfg.Auth)
	if err != nil {
		return nil, err
	}

	c := &Client{
		writer: &kafkago.Writer{
			Addr:      kafkago.TCP(cfg.Brokers...),
			Transport: &kafkago.Transport{SASL: mechanism, TLS: tlsConfig},
			Balancer:  &kafkago.LeastBytes{},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Produce sends one message with key/value/headers to topic, blocking until
// the broker acknowledges it or ctx is done.
func (c *Client) Produce(ctx context.Context, topic string, key, value []byte, headers map[string]string) error {
	msg := kafkago.Message{Topic: topic, Key: key, Value: value, Headers: toHeaders(headers)}
	if err := c.writer.WriteMessages(ctx, msg); err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("kafka: produce failed")
	}
	return nil
}

// Close flushes any buffered messages and releases the underlying
// connection. Safe to call once; the Client must not be used afterward.
func (c *Client) Close() error {
	if err := c.writer.Close(); err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("kafka: close failed")
	}
	return nil
}

// toHeaders converts a plain header map to kafka-go's []Header. nil when
// headers is empty, so an unheadered Produce call doesn't allocate.
func toHeaders(headers map[string]string) []kafkago.Header {
	if len(headers) == 0 {
		return nil
	}
	out := make([]kafkago.Header, 0, len(headers))
	for k, v := range headers {
		out = append(out, kafkago.Header{Key: k, Value: []byte(v)})
	}
	return out
}
