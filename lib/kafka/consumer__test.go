package kafka

import (
	"testing"

	kafkago "github.com/segmentio/kafka-go"
)

func validConfig() *Config {
	return &Config{Brokers: []string{"localhost:9092"}, Auth: AuthConfig{Mechanism: MechanismNone}}
}

func TestNewConsumer_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		groupID string
		topic   string
		wantErr bool
	}{
		{name: "valid", cfg: validConfig(), groupID: "g", topic: "t"},
		{name: "invalid config", cfg: &Config{}, groupID: "g", topic: "t", wantErr: true},
		{name: "empty groupID", cfg: validConfig(), groupID: "", topic: "t", wantErr: true},
		{name: "empty topic", cfg: validConfig(), groupID: "g", topic: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewConsumer(tt.cfg, tt.groupID, tt.topic)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewConsumer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if c == nil || c.reader == nil {
				t.Fatal("NewConsumer() built a Consumer with a nil reader")
			}
			_ = c.Close()
		})
	}
}

func TestToMessage(t *testing.T) {
	t.Run("nil headers stay nil", func(t *testing.T) {
		m := &kafkago.Message{Topic: "t", Key: []byte("k"), Value: []byte("v")}
		got := toMessage(m)
		if got.Topic != "t" || string(got.Key) != "k" || string(got.Value) != "v" || got.Headers != nil {
			t.Errorf("toMessage() = %+v", got)
		}
	})

	t.Run("headers converted", func(t *testing.T) {
		m := &kafkago.Message{
			Topic:   "t",
			Headers: []kafkago.Header{{Key: "h1", Value: []byte("v1")}, {Key: "h2", Value: []byte("v2")}},
		}
		got := toMessage(m)
		if got.Headers["h1"] != "v1" || got.Headers["h2"] != "v2" {
			t.Errorf("toMessage().Headers = %v", got.Headers)
		}
	})
}
