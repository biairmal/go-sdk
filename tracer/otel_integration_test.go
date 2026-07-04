package tracer

import (
	"context"
	"testing"
	"time"
)

// TestOTelTracer_Integration exercises the real OTLP/gRPC exporter construction
// and span lifecycle. It does not require a reachable collector: the gRPC
// client dials lazily, so span creation and id generation succeed regardless.
// Shutdown may fail to flush without a live collector at localhost:4317; that
// is logged, not fatal, since this test's purpose is verifying the real
// exporter wiring, not asserting delivery to a collector.
func TestOTelTracer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: exercises the real OTLP/gRPC exporter")
	}

	tr, err := NewOTel(Config{
		ServiceName: "tracer-integration-test",
		Endpoint:    "localhost:4317",
		Insecure:    true,
		SampleRate:  1,
	})
	if err != nil {
		t.Fatalf("NewOTel() = %v, want nil", err)
	}

	ctx, span := tr.Start(context.Background(), "integration-span", WithSpanKind(SpanKindServer))
	span.SetAttributes(map[string]any{"test": true})
	span.End()

	sc := span.SpanContext()
	if sc.TraceID == "" {
		t.Error("SpanContext().TraceID is empty, want a generated trace id")
	}
	if sc.SpanID == "" {
		t.Error("SpanContext().SpanID is empty, want a generated span id")
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := tr.Shutdown(shutdownCtx); err != nil {
		t.Logf("Shutdown() = %v (no live collector at localhost:4317; wiring is still verified)", err)
	}
}
