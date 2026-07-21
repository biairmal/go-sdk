package tracer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNoOpTracer_StartAndEnd(t *testing.T) {
	tr := NewNoOp()

	ctx, span := tr.Start(context.Background(), "op")
	if ctx == nil {
		t.Fatal("Start() returned nil context")
	}

	span.SetAttributes(map[string]any{"key": "value"})
	span.SetError(errors.New("boom"))
	span.End()

	sc := span.SpanContext()
	if sc.TraceID != "" || sc.SpanID != "" || sc.IsSampled {
		t.Errorf("SpanContext() = %+v, want zero value", sc)
	}
}

func TestNoOpTracer_Shutdown(t *testing.T) {
	tr := NewNoOp()
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg:  Config{ServiceName: "svc", Endpoint: "localhost:4317", SampleRate: 1},
		},
		{
			name:    "missing service name",
			cfg:     Config{Endpoint: "localhost:4317", SampleRate: 1},
			wantErr: true,
		},
		{
			name:    "missing endpoint",
			cfg:     Config{ServiceName: "svc", SampleRate: 1},
			wantErr: true,
		},
		{
			name:    "sample rate too high",
			cfg:     Config{ServiceName: "svc", Endpoint: "localhost:4317", SampleRate: 1.5},
			wantErr: true,
		},
		{
			name:    "sample rate negative",
			cfg:     Config{ServiceName: "svc", Endpoint: "localhost:4317", SampleRate: -0.1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestWithDefaults(t *testing.T) {
	cfg := withDefaults(Config{ServiceName: "svc", Endpoint: "localhost:4317"})
	if cfg.ServiceVersion != "0.0.0" {
		t.Errorf("ServiceVersion = %q, want %q", cfg.ServiceVersion, "0.0.0")
	}
	if cfg.SampleRate != 1.0 {
		t.Errorf("SampleRate = %v, want 1.0", cfg.SampleRate)
	}
}

func TestNewOTel_InvalidConfig(t *testing.T) {
	if _, err := NewOTel(Config{}); err == nil {
		t.Fatal("NewOTel() with empty config = nil error, want error")
	}
}

// TestNewOTel_ValidConfig exercises the real exporter/resource/span
// construction paths. It never calls Shutdown with a long deadline, so it
// stays fast even without a reachable collector: the gRPC client dials
// lazily and Span.End() only enqueues into the in-memory batch processor.
func TestNewOTel_ValidConfig(t *testing.T) {
	tr, err := NewOTel(Config{
		ServiceName: "svc",
		Endpoint:    "localhost:4317",
		Insecure:    true,
		SampleRate:  1,
	}, WithLogger(nil))
	if err != nil {
		t.Fatalf("NewOTel() = %v, want nil", err)
	}

	ot, ok := tr.(*otelTracer)
	if !ok {
		t.Fatalf("NewOTel() returned %T, want *otelTracer", tr)
	}
	if ot.log != nil {
		t.Errorf("log = %v, want nil (WithLogger(nil))", ot.log)
	}

	ctx, span := tr.Start(context.Background(), "op",
		WithSpanKind(SpanKindClient),
		WithAttributes(map[string]any{"k": "v"}),
	)
	if ctx == nil {
		t.Fatal("Start() returned nil context")
	}

	span.SetAttributes(map[string]any{
		"str": "s", "bool": true, "int": 1, "int64": int64(2), "float64": 2.5, "other": []string{"x"},
	})
	span.SetAttributes(nil) // no-op
	span.SetError(nil)      // no-op
	span.SetError(errors.New("boom"))
	span.End()

	sc := span.SpanContext()
	if sc.TraceID == "" {
		t.Error("SpanContext().TraceID is empty, want a generated trace id")
	}
	if sc.SpanID == "" {
		t.Error("SpanContext().SpanID is empty, want a generated span id")
	}
	if !sc.IsSampled {
		t.Error("IsSampled = false, want true (SampleRate 1.0)")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_ = tr.Shutdown(shutdownCtx) // exercise Shutdown; no live collector, so an error is expected here
}

func TestToOTelKind(t *testing.T) {
	tests := []SpanKind{
		SpanKindUnspecified, SpanKindInternal, SpanKindServer,
		SpanKindClient, SpanKindProducer, SpanKindConsumer, SpanKind(99),
	}
	for _, kind := range tests {
		if got := toOTelKind(kind); got.String() == "" {
			t.Errorf("toOTelKind(%v) returned an empty kind", kind)
		}
	}
}

func TestToAttribute(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  any
	}{
		{name: "string", key: "s", val: "hello"},
		{name: "bool", key: "b", val: true},
		{name: "int", key: "i", val: 42},
		{name: "int64", key: "i64", val: int64(42)},
		{name: "float64", key: "f", val: 3.14},
		{name: "fallback", key: "o", val: []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := toAttribute(tt.key, tt.val)
			if string(kv.Key) != tt.key {
				t.Errorf("Key = %q, want %q", kv.Key, tt.key)
			}
		})
	}
}

func TestBuildSpanConfig(t *testing.T) {
	cfg := buildSpanConfig(
		WithSpanKind(SpanKindServer),
		WithAttributes(map[string]any{"a": 1}),
		WithAttributes(map[string]any{"b": 2}),
		nil,
	)

	if cfg.kind != SpanKindServer {
		t.Errorf("kind = %v, want %v", cfg.kind, SpanKindServer)
	}
	if len(cfg.attributes) != 2 {
		t.Errorf("attributes = %v, want 2 entries", cfg.attributes)
	}
}

func TestBuildSpanConfig_DefaultKind(t *testing.T) {
	cfg := buildSpanConfig()
	if cfg.kind != SpanKindInternal {
		t.Errorf("default kind = %v, want %v", cfg.kind, SpanKindInternal)
	}
}
