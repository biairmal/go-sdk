package tracer

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// newMemTracer builds an OTel tracer that exports into an in-memory exporter.
func newMemTracer(t *testing.T) (*otelTracer, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	return newOTelTracer(withDefaults(Config{ServiceName: "test", Endpoint: "unused"}), exp), exp
}

// flushed returns the spans exported so far.
func flushed(t *testing.T, tr *otelTracer, exp *tracetest.InMemoryExporter) tracetest.SpanStubs {
	t.Helper()
	if err := tr.provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}
	return exp.GetSpans()
}

func TestNewOTel_SetsGlobalProviderAndPropagator(t *testing.T) {
	tr, exp := newMemTracer(t)

	ctx, parent := tr.Start(context.Background(), "parent")
	_, child := otel.Tracer("x").Start(ctx, "child")
	child.End()
	parent.End()

	spans := flushed(t, tr, exp)
	if len(spans) != 2 {
		t.Fatalf("exported %d spans, want 2", len(spans))
	}
	got, want := spans[0], parent.SpanContext()
	if got.Name != "child" || got.SpanContext.TraceID().String() != want.TraceID ||
		got.Parent.SpanID().String() != want.SpanID {
		t.Errorf("child span %q trace=%s parent=%s, want child of %+v",
			got.Name, got.SpanContext.TraceID(), got.Parent.SpanID(), want)
	}
	if _, ok := otel.GetTextMapPropagator().(propagation.TraceContext); !ok {
		t.Errorf("global propagator = %T, want propagation.TraceContext", otel.GetTextMapPropagator())
	}
}
