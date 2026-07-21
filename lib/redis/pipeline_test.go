package redis

import (
	"context"
	"testing"
	"time"
)

// TestPipeline_and_TxPipeline_return_non_nil runs only when a real Redis is
// available (integration). It uses table-driven tests to assert that
// Pipeline() and TxPipeline() return non-nil Pipeliner implementations.
func TestPipeline_and_TxPipeline_return_non_nil(t *testing.T) {
	cfg := DefaultConfig()
	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available (NewClient failed): %v", err)
	}
	defer func() { _ = client.Close() }()

	tests := []struct {
		name   string
		getter func() Pipeliner
	}{
		{"Pipeline returns non-nil", func() Pipeliner { return client.Pipeline() }},
		{"TxPipeline returns non-nil", func() Pipeliner { return client.TxPipeline() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipe := tt.getter()
			if pipe == nil {
				t.Fatal("Pipeliner must not be nil")
			}
		})
	}
}

func TestPipeline_Exec_and_Discard(t *testing.T) {
	cfg := DefaultConfig()
	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name    string
		pipe    Pipeliner
		exec    bool
		discard bool
	}{
		{
			name:    "Pipeline Exec after Set",
			pipe:    client.Pipeline(),
			exec:    true,
			discard: false,
		},
		{
			name:    "TxPipeline Exec after Set",
			pipe:    client.TxPipeline(),
			exec:    true,
			discard: false,
		},
		{
			name:    "Pipeline Discard without Exec",
			pipe:    client.Pipeline(),
			exec:    false,
			discard: true,
		},
		{
			name:    "TxPipeline Discard without Exec",
			pipe:    client.TxPipeline(),
			exec:    false,
			discard: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "pipeline-test-key"
			_ = tt.pipe.Set(ctx, key, "val", 0)
			if tt.discard {
				if err := tt.pipe.Discard(); err != nil {
					t.Errorf("Discard() = %v", err)
				}
				return
			}
			if err := tt.pipe.Exec(ctx); err != nil {
				t.Errorf("Exec() = %v", err)
			}
		})
	}
}
