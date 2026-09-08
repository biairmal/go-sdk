package queue

import (
	"testing"

	"github.com/biairmal/go-sdk/lib/kafka"
	"github.com/biairmal/go-sdk/lib/logger"
)

func validKafkaConfig() kafka.Config {
	return kafka.Config{Brokers: []string{"localhost:9092"}, Auth: kafka.AuthConfig{Mechanism: kafka.MechanismNone}}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "noop", cfg: Config{Backend: BackendNoOp}},
		{name: "logging", cfg: Config{Backend: BackendLogging}},
		{name: "kafka valid", cfg: Config{Backend: BackendKafka, Kafka: validKafkaConfig()}},
		{name: "kafka invalid nested config", cfg: Config{Backend: BackendKafka}, wantErr: true},
		{name: "unknown backend", cfg: Config{Backend: "bogus"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig_IsValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() = %v, want nil", err)
	}
}

func TestFromConfig(t *testing.T) {
	t.Run("noop backend", func(t *testing.T) {
		pub, err := FromConfig(&Config{Backend: BackendNoOp}, nil, nil)
		if err != nil {
			t.Fatalf("FromConfig() error = %v, want nil", err)
		}
		if _, ok := pub.(noOp); !ok {
			t.Fatalf("FromConfig() type = %T, want noOp", pub)
		}
	})

	t.Run("logging backend requires a logger", func(t *testing.T) {
		_, err := FromConfig(&Config{Backend: BackendLogging}, nil, nil)
		if err == nil {
			t.Fatal("FromConfig() error = nil, want non-nil for missing logger")
		}
	})

	t.Run("logging backend with logger", func(t *testing.T) {
		pub, err := FromConfig(&Config{Backend: BackendLogging}, logger.NewNoOp(), nil)
		if err != nil {
			t.Fatalf("FromConfig() error = %v, want nil", err)
		}
		if _, ok := pub.(logging); !ok {
			t.Fatalf("FromConfig() type = %T, want logging", pub)
		}
	})

	t.Run("kafka backend requires a client", func(t *testing.T) {
		_, err := FromConfig(&Config{Backend: BackendKafka, Kafka: validKafkaConfig()}, nil, nil)
		if err == nil {
			t.Fatal("FromConfig() error = nil, want non-nil for missing kafka client")
		}
	})

	t.Run("kafka backend with client", func(t *testing.T) {
		pub, err := FromConfig(&Config{Backend: BackendKafka, Kafka: validKafkaConfig()}, nil, &kafka.Client{})
		if err != nil {
			t.Fatalf("FromConfig() error = %v, want nil", err)
		}
		if _, ok := pub.(kafkaPublisher); !ok {
			t.Fatalf("FromConfig() type = %T, want kafkaPublisher", pub)
		}
	})

	t.Run("invalid config surfaces the validation error", func(t *testing.T) {
		_, err := FromConfig(&Config{Backend: "bogus"}, nil, nil)
		if err == nil {
			t.Fatal("FromConfig() error = nil, want non-nil for invalid config")
		}
	})
}
