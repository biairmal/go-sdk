package queue

import (
	"context"

	"github.com/biairmal/go-sdk/lib/kafka"
)

// kafkaSubscriber subscribes through a kafka.Consumer, built fresh (and
// closed) inside each Subscribe call — one per topic, matching
// kafka.Consumer's one-topic-per-instance model.
type kafkaSubscriber struct {
	cfg     *kafka.Config
	groupID string
}

// NewKafkaSubscriber builds a Subscriber backed by cfg/groupID. Both are
// required, already-validated-at-Subscribe-time dependencies (mirrors
// NewKafka's client.go).
func NewKafkaSubscriber(cfg *kafka.Config, groupID string) Subscriber {
	return kafkaSubscriber{cfg: cfg, groupID: groupID}
}

// Subscribe builds a kafka.Consumer for topic, runs its Consume loop
// (adapting kafka.Message to queue.Message for handler), and closes the
// Consumer when the loop returns.
func (s kafkaSubscriber) Subscribe(ctx context.Context, topic string, handler Handler) error {
	consumer, err := kafka.NewConsumer(s.cfg, s.groupID, topic)
	if err != nil {
		return err
	}
	defer func() { _ = consumer.Close() }()

	return consumer.Consume(ctx, func(ctx context.Context, msg kafka.Message) error {
		return handler(ctx, Message{Topic: msg.Topic, Key: msg.Key, Value: msg.Value, Headers: msg.Headers})
	})
}
