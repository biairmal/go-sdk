//go:build integration

package redis

import (
	"context"
	"testing"
	"time"
)

// Integration tests for Pipeline and TxPipeline. Require a running Redis instance
// (e.g. default localhost:6379). Run with: go test -tags=integration ./redis/...

func TestIntegration_Pipeline_and_TxPipeline_return_non_nil(t *testing.T) {
	cfg := DefaultConfig()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Redis not available (required for integration test): %v", err)
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

func TestIntegration_Pipeline_Exec_and_Discard(t *testing.T) {
	cfg := DefaultConfig()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Redis not available (required for integration test): %v", err)
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
			key := "pipeline-integration-test-key"
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
