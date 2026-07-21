package tracer

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/biairmal/go-sdk/lib/logger"
)

// otelTracer implements Tracer using the OpenTelemetry SDK with an OTLP/gRPC exporter.
type otelTracer struct {
	provider *sdktrace.TracerProvider
	tracer   oteltrace.Tracer
	log      logger.Logger
}

// NewOTel builds a Tracer that exports spans via OTLP/gRPC. Cfg carries the
// YAML-able knobs; zero-valued fields fall back to DefaultConfig(). Opts
// inject an optional logger.Logger for diagnostics.
//
// The returned Tracer owns the underlying exporter and TracerProvider;
// callers must call Shutdown during graceful shutdown to flush buffered spans.
func NewOTel(cfg Config, opts ...Option) (Tracer, error) {
	cfg = withDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	exporter, err := newExporter(cfg)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newResource(cfg)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))),
	)

	t := &otelTracer{
		provider: provider,
		tracer:   provider.Tracer(cfg.ServiceName, oteltrace.WithInstrumentationVersion(cfg.ServiceVersion)),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(t)
		}
	}
	return t, nil
}

// newExporter builds the OTLP/gRPC span exporter for cfg.
func newExporter(cfg Config) (sdktrace.SpanExporter, error) {
	dialOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		dialOpts = append(dialOpts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(context.Background(), dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("tracer: create otlp exporter: %w", err)
	}
	return exporter, nil
}

// newResource builds the OTel resource identifying this service.
func newResource(cfg Config) *sdkresource.Resource {
	return sdkresource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	)
}

// Start begins a new span named name as a child of any span in ctx.
func (t *otelTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	cfg := buildSpanConfig(opts...)

	startOpts := []oteltrace.SpanStartOption{oteltrace.WithSpanKind(toOTelKind(cfg.kind))}
	if len(cfg.attributes) > 0 {
		startOpts = append(startOpts, oteltrace.WithAttributes(toAttributes(cfg.attributes)...))
	}

	ctx, span := t.tracer.Start(ctx, name, startOpts...)
	return ctx, &otelSpan{span: span}
}

// Shutdown flushes buffered spans and releases exporter resources.
func (t *otelTracer) Shutdown(ctx context.Context) error {
	if err := t.provider.Shutdown(ctx); err != nil {
		if t.log != nil {
			t.log.ErrorfWithContext(ctx, "tracer: shutdown: %v", err)
		}
		return fmt.Errorf("tracer: shutdown: %w", err)
	}
	return nil
}

// toOTelKind maps our SpanKind onto the OTel SDK's equivalent.
func toOTelKind(kind SpanKind) oteltrace.SpanKind {
	switch kind {
	case SpanKindInternal:
		return oteltrace.SpanKindInternal
	case SpanKindServer:
		return oteltrace.SpanKindServer
	case SpanKindClient:
		return oteltrace.SpanKindClient
	case SpanKindProducer:
		return oteltrace.SpanKindProducer
	case SpanKindConsumer:
		return oteltrace.SpanKindConsumer
	default:
		return oteltrace.SpanKindUnspecified
	}
}

// toAttributes converts a plain key-value map into OTel attributes. Values
// are converted by concrete type where possible; anything else is rendered
// via fmt.Sprint.
func toAttributes(attrs map[string]any) []attribute.KeyValue {
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kvs = append(kvs, toAttribute(k, v))
	}
	return kvs
}

// toAttribute converts a single key-value pair into an OTel attribute.
func toAttribute(k string, v any) attribute.KeyValue {
	switch val := v.(type) {
	case string:
		return attribute.String(k, val)
	case bool:
		return attribute.Bool(k, val)
	case int:
		return attribute.Int(k, val)
	case int64:
		return attribute.Int64(k, val)
	case float64:
		return attribute.Float64(k, val)
	default:
		return attribute.String(k, fmt.Sprint(val))
	}
}

// otelSpan implements Span using an OTel SDK span.
type otelSpan struct {
	span oteltrace.Span
}

// End completes the span.
func (s *otelSpan) End() {
	s.span.End()
}

// SetError marks the span as failed and records err.
func (s *otelSpan) SetError(err error) {
	if err == nil {
		return
	}
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

// SetAttributes attaches key-value attributes to the span.
func (s *otelSpan) SetAttributes(attrs map[string]any) {
	if len(attrs) == 0 {
		return
	}
	s.span.SetAttributes(toAttributes(attrs)...)
}

// SpanContext returns the span's trace/span identifiers.
func (s *otelSpan) SpanContext() SpanContext {
	sc := s.span.SpanContext()
	return SpanContext{
		TraceID:   sc.TraceID().String(),
		SpanID:    sc.SpanID().String(),
		IsSampled: sc.IsSampled(),
	}
}
