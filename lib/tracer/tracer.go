// Package tracer provides interface-first distributed tracing: a Tracer
// starts Spans, an OpenTelemetry OTLP/gRPC backend ships them to a collector
// (e.g. Tempo), and a NoOp backend is available for tests and environments
// where tracing is disabled. Server middleware (httpkit/middleware.Tracing)
// extracts inbound W3C trace context, starts a span per request, and
// publishes the trace id via ctxkit so it appears on every log line.
//
// Example usage:
//
//	tr, err := tracer.NewOTel(tracer.Config{
//		ServiceName: "orders-api",
//		Endpoint:    "localhost:4317",
//		Insecure:    true,
//		SampleRate:  1.0,
//	})
//	if err != nil {
//		return err
//	}
//	defer tr.Shutdown(context.Background())
//
//	ctx, span := tr.Start(ctx, "process-order")
//	defer span.End()
//	if err := doWork(ctx); err != nil {
//		span.SetError(err)
//	}
package tracer

import "context"

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/tracer/mock_tracer.go -package=mocktracer github.com/biairmal/go-sdk/lib/tracer Tracer

// Tracer starts spans and flushes them on shutdown.
type Tracer interface {
	// Start begins a new span named name as a child of any span carried by
	// ctx, returning a context carrying the new span alongside the Span
	// itself. Callers must call Span.End() (typically via defer).
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
	// Shutdown flushes any buffered spans and releases exporter resources.
	// Call it once during graceful shutdown; further Start calls after
	// Shutdown are backend-defined (the OTel backend becomes a no-op).
	Shutdown(ctx context.Context) error
}

// Span represents a single unit of work within a trace.
type Span interface {
	// End completes the span, recording its duration. Safe to call once;
	// implementations must tolerate at most one meaningful End.
	End()
	// SetError marks the span as failed and records err. A nil err is a no-op.
	SetError(err error)
	// SetAttributes attaches key-value attributes to the span. Values should
	// be primitive types (string, bool, numeric) or slices thereof.
	SetAttributes(attrs map[string]any)
	// SpanContext returns the span's trace/span identifiers.
	SpanContext() SpanContext
}

// SpanContext carries a span's trace/span identifiers, mirroring the W3C
// trace context fields without leaking the OTel SDK's types into this
// package's exported API.
type SpanContext struct {
	// TraceID is the hex-encoded id shared by every span in the trace.
	TraceID string
	// SpanID is the hex-encoded id of this span.
	SpanID string
	// IsSampled reports whether this span was sampled (i.e. exported).
	IsSampled bool
}

// SpanKind classifies a span's relationship to the unit of work it
// represents, mirroring OpenTelemetry's span-kind taxonomy.
type SpanKind int

// Span kinds. SpanKindInternal is the default when no WithSpanKind option is given.
const (
	SpanKindUnspecified SpanKind = iota
	SpanKindInternal
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// spanConfig holds the settings applied by SpanOption funcs.
type spanConfig struct {
	kind       SpanKind
	attributes map[string]any
}

// buildSpanConfig applies opts over a spanConfig defaulted to SpanKindInternal.
func buildSpanConfig(opts ...SpanOption) spanConfig {
	cfg := spanConfig{kind: SpanKindInternal}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// SpanOption configures a span at Start time.
type SpanOption func(*spanConfig)

// WithSpanKind sets the span's kind. Default SpanKindInternal.
func WithSpanKind(kind SpanKind) SpanOption {
	return func(c *spanConfig) { c.kind = kind }
}

// WithAttributes attaches initial key-value attributes to the span, in
// addition to any set later via Span.SetAttributes. A nil/empty map is a no-op.
func WithAttributes(attrs map[string]any) SpanOption {
	return func(c *spanConfig) {
		if len(attrs) == 0 {
			return
		}
		if c.attributes == nil {
			c.attributes = make(map[string]any, len(attrs))
		}
		for k, v := range attrs {
			c.attributes[k] = v
		}
	}
}
