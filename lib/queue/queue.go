// Package queue abstracts async, fire-and-forget message publishing behind a
// swappable backend: NewNoOp discards, NewLogging logs (a stand-in for a
// real broker in dev/tests), and NewKafka publishes through a kafka.Client.
// Consumers depend only on the Publisher interface and Config — never on a
// specific broker client — the same way ratelimit depends on redis without
// leaking a redis type into its own interface.
//
// Example usage:
//
//	pub, err := queue.FromConfig(&cfg, log, kafkaClient)
//	err = pub.Publish(ctx, "orders.created", body, queue.WithKey([]byte(orderID)))
package queue

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/queue/mock_queue.go -package=mockqueue github.com/biairmal/go-sdk/lib/queue Publisher

import "context"

// Publisher abstracts async message publishing behind a swappable backend.
// Implementations must be safe for concurrent use.
type Publisher interface {
	// Publish sends message to topic. Fire-and-forget from the caller's
	// perspective; opts customize per-message behavior the active backend
	// can honor (ignored otherwise, never an error).
	Publish(ctx context.Context, topic string, message []byte, opts ...PublishOption) error
}

// publishOptions holds the per-call settings PublishOption funcs populate.
type publishOptions struct {
	key     []byte
	headers map[string]string
}

// PublishOption customizes one Publish call. Only options with a consistent
// meaning across backends belong here — backend-only behavior (ack level,
// compression, partition count, ...) is config on that specific backend,
// never a PublishOption.
type PublishOption func(*publishOptions)

// WithKey sets the message's ordering key (Kafka partition key, SQS FIFO
// message-group-id, ...). Backends without ordering ignore it.
func WithKey(key []byte) PublishOption {
	return func(o *publishOptions) {
		o.key = key
	}
}

// WithHeaders attaches metadata headers to the message.
func WithHeaders(headers map[string]string) PublishOption {
	return func(o *publishOptions) {
		o.headers = headers
	}
}

// applyOptions folds opts into a fresh publishOptions, for backends that
// honor key/headers (currently only NewKafka).
func applyOptions(opts []PublishOption) publishOptions {
	var o publishOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
