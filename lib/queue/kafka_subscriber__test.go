package queue

import (
	"context"
	"testing"

	"github.com/biairmal/go-sdk/lib/kafka"
)

func TestNewKafkaSubscriber_SubscribeSurfacesConsumerErrors(t *testing.T) {
	sub := NewKafkaSubscriber(&kafka.Config{}, "group")

	err := sub.Subscribe(context.Background(), "topic", func(context.Context, Message) error { return nil })
	if err == nil {
		t.Fatal("Subscribe() error = nil, want non-nil for an invalid kafka.Config")
	}
}
