package queue

import (
	"context"

	"github.com/biairmal/go-sdk/lib/kafka"
)

// kafkaPublisher publishes through a kafka.Client.
type kafkaPublisher struct {
	client *kafka.Client
}

// NewKafka builds a Publisher backed by client. client is a required,
// already-constructed dependency (mirrors ratelimit.NewRedis).
func NewKafka(client *kafka.Client) Publisher {
	return kafkaPublisher{client: client}
}

// Publish extracts key/headers from opts and calls client.Produce.
func (p kafkaPublisher) Publish(ctx context.Context, topic string, message []byte, opts ...PublishOption) error {
	o := applyOptions(opts)
	return p.client.Produce(ctx, topic, o.key, message, o.headers)
}
