package tracer

import "context"

// NewNoOp returns a Tracer whose spans do nothing. Use it in tests and in
// environments where tracing is disabled, without branching call sites on a
// nil Tracer.
func NewNoOp() Tracer {
	return noOpTracer{}
}

// noOpTracer implements Tracer with no-op spans.
type noOpTracer struct{}

// Start returns ctx unchanged and a Span that discards everything.
func (noOpTracer) Start(ctx context.Context, _ string, _ ...SpanOption) (context.Context, Span) {
	return ctx, noOpSpan{}
}

// Shutdown is a no-op.
func (noOpTracer) Shutdown(_ context.Context) error { return nil }

// noOpSpan implements Span with no-op methods.
type noOpSpan struct{}

func (noOpSpan) End()                           {}
func (noOpSpan) SetError(_ error)               {}
func (noOpSpan) SetAttributes(_ map[string]any) {}
func (noOpSpan) SpanContext() SpanContext       { return SpanContext{} }
