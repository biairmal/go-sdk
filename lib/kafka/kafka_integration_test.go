//go:build integration

package kafka

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// createTopic creates topic via the plaintext listener (always available,
// independent of the mechanism under test) — kafka-go's Writer defaults
// AllowAutoTopicCreation to false, so Produce fails on a topic that doesn't
// already exist.
func createTopic(t *testing.T, topic string) {
	t.Helper()
	conn, err := kafkago.Dial("tcp", "localhost:9092")
	if err != nil {
		t.Fatalf("Dial() error = %v, want nil (is the plaintext listener up at localhost:9092?)", err)
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		t.Fatalf("Controller() error = %v, want nil", err)
	}
	controllerConn, err := kafkago.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		t.Fatalf("Dial(controller) error = %v, want nil", err)
	}
	defer func() { _ = controllerConn.Close() }()

	err = controllerConn.CreateTopics(kafkago.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
	if err != nil {
		t.Fatalf("CreateTopics() error = %v, want nil", err)
	}

	// CreateTopics returns once the controller accepts the request, not once
	// the topic's metadata is queryable cluster-wide — a Produce right after
	// this call can still race "Unknown Topic Or Partition". Poll until the
	// topic is actually visible before handing control back to the caller.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := conn.ReadPartitions(topic); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("topic %q did not become visible within 10s of CreateTopics()", topic)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// errStopConsuming is returned by the test handler after the first message,
// to stop Consume's otherwise-infinite loop without wrapping it in an
// errorz value (Consume returns handler errors as-is).
var errStopConsuming = errors.New("stop after first message")

// Integration tests for a real produce/consume round trip against every
// AuthConfig.Mechanism. Requires the broker from
// ../../docker-compose.integration.yaml:
//
//	scripts/gen-kafka-certs.sh
//	docker compose -f docker-compose.integration.yaml up -d
//	go test -tags=integration ./kafka/...
//
// One broker, one listener per mechanism (see the compose file's header
// comment for the port/credential map); each case below dials its own port.

func TestIntegration_Client_ProduceConsume(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires a live Kafka broker")
	}

	certDir := filepath.Join("..", "..", ".kafka-certs")
	tests := []struct {
		name   string
		broker string
		auth   AuthConfig
	}{
		{
			name:   "none",
			broker: "localhost:9092",
			auth:   AuthConfig{Mechanism: MechanismNone},
		},
		{
			name:   "plain",
			broker: "localhost:9093",
			auth:   AuthConfig{Mechanism: MechanismPlain, Username: "testuser", Password: "testpass"},
		},
		{
			name:   "scram-sha256",
			broker: "localhost:9094",
			auth:   AuthConfig{Mechanism: MechanismScramSHA256, Username: "testuser", Password: "testpass"},
		},
		{
			name:   "scram-sha512",
			broker: "localhost:9095",
			auth:   AuthConfig{Mechanism: MechanismScramSHA512, Username: "testuser", Password: "testpass"},
		},
		{
			name:   "mtls",
			broker: "localhost:9096",
			auth: AuthConfig{
				Mechanism: MechanismMTLS,
				TLS: TLSConfig{
					CertFile: filepath.Join(certDir, "client.pem"),
					KeyFile:  filepath.Join(certDir, "client.key"),
					CAFile:   filepath.Join(certDir, "ca.pem"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Brokers: []string{tt.broker}, Auth: tt.auth}
			client, err := New(cfg)
			if err != nil {
				t.Fatalf("New() error = %v, want nil", err)
			}
			defer func() { _ = client.Close() }()

			topic := fmt.Sprintf("kafka-integration-test-%s-%d", tt.name, time.Now().UnixNano())
			createTopic(t, topic)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = client.Produce(ctx, topic, []byte("key"), []byte("value"), map[string]string{"h": "v"})
			if err != nil {
				t.Fatalf("Produce() error = %v, want nil (is a broker reachable at %s?)", err, tt.broker)
			}

			groupID := fmt.Sprintf("kafka-integration-test-group-%s-%d", tt.name, time.Now().UnixNano())
			consumer, err := NewConsumer(cfg, groupID, topic)
			if err != nil {
				t.Fatalf("NewConsumer() error = %v, want nil", err)
			}
			defer func() { _ = consumer.Close() }()

			var got Message
			err = consumer.Consume(ctx, func(_ context.Context, msg Message) error {
				got = msg
				return errStopConsuming
			})
			if !errors.Is(err, errStopConsuming) {
				t.Fatalf("Consume() error = %v, want errStopConsuming", err)
			}
			if got.Topic != topic || !bytes.Equal(got.Key, []byte("key")) || !bytes.Equal(got.Value, []byte("value")) {
				t.Errorf("Consume() got = %+v, want topic=%s key=key value=value", got, topic)
			}
			if got.Headers["h"] != "v" {
				t.Errorf("Consume() got.Headers[h] = %q, want %q", got.Headers["h"], "v")
			}
		})
	}
}
