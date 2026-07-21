# Tracer Package

`tracer` provides interface-first distributed tracing: a `Tracer` starts `Span`s, an OpenTelemetry OTLP/gRPC backend ships them to a collector (e.g. Tempo), and a `NoOp` backend is available for tests and environments where tracing is disabled.

## Overview

Services need to see how a request flows across process boundaries — which downstream calls it made, where time was spent, which one failed. `tracer` wraps [OpenTelemetry](https://opentelemetry.io/)'s tracing SDK behind a small `Tracer`/`Span` interface so callers never import the OTel API directly. `httpkit/middleware.Tracing` starts a server span per request, extracts inbound W3C trace context (`traceparent`/`tracestate`) so this service's spans join the caller's trace, and publishes the resulting trace id via [`ctxkit`](../ctxkit/README.md) so `ctxkit.LoggerExtractor()` puts it on every log line — no separate log exporter needed to correlate logs with traces.

## Features

- **Interface-first**: `Tracer`/`Span` hide the OTel SDK; swap backends (`NewOTel`, `NewNoOp`) without touching call sites.
- **OTLP/gRPC export**: `NewOTel` ships spans to any OTLP-compatible collector (Tempo, Jaeger, an OTel Collector).
- **W3C propagation**: `httpkit/middleware.Tracing` extracts/joins the caller's trace via the standard `traceparent` header.
- **`ctxkit` integration**: every started span's trace id is published via `ctxkit.WithTraceID`, so it shows up in logs automatically.
- **`NoOp` backend**: `NewNoOp()` discards everything — use it in tests or to disable tracing without branching call sites.
- **Span kinds & attributes**: `WithSpanKind` and `WithAttributes` mirror OTel's span-kind taxonomy and key-value attributes without leaking OTel types.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/tracer
```

### Basic usage

```go
package main

import (
    "context"

    "github.com/biairmal/go-sdk/lib/tracer"
)

func main() {
    tr, err := tracer.NewOTel(tracer.Config{
        ServiceName: "orders-api",
        Endpoint:    "localhost:4317",
        Insecure:    true, // no TLS to a local collector
        SampleRate:  1.0,
    })
    if err != nil {
        panic(err)
    }
    defer tr.Shutdown(context.Background())

    ctx, span := tr.Start(context.Background(), "process-order")
    defer span.End()

    if err := processOrder(ctx); err != nil {
        span.SetError(err)
    }
}
```

### HTTP server middleware

```go
handler := middleware.Chain(mux,
    middleware.RequestID(),
    middleware.Correlation(),
    middleware.Tracing(tr),   // extracts traceparent, starts a server span, publishes ctxkit.WithTraceID
    middleware.Logging(log, nil),
)
```

Place `Tracing` after `RequestID`/`Correlation` and before `Logging` so the trace id is available to the request logger.

### Custom spans and attributes

```go
ctx, span := tr.Start(ctx, "charge-card",
    tracer.WithSpanKind(tracer.SpanKindClient),
    tracer.WithAttributes(map[string]any{"provider": "stripe"}),
)
defer span.End()

span.SetAttributes(map[string]any{"amount_cents": 1999})
if err != nil {
    span.SetError(err)
}
```

### Disabling tracing

```go
tr := tracer.NewNoOp() // same Tracer interface; every call is a no-op
```

### SQL tracing (consumer-side)

`sqlkit` itself is not instrumented. To trace SQL calls, register an [`otelsql`](https://github.com/XSAM/otelsql) driver in your application and open `sqlkit` against that driver name — no `sqlkit` change is required.

## Options

| Option | Description |
|--------|-------------|
| `WithLogger(log logger.Logger)` | Optional logger for internal diagnostics (e.g. shutdown/flush failures). Nil-safe; defaults to silent. |

`SpanOption`s (passed to `Start`):

| Option | Description |
|--------|-------------|
| `WithSpanKind(kind SpanKind)` | Sets the span's kind (`SpanKindServer`, `SpanKindClient`, …). Default `SpanKindInternal`. |
| `WithAttributes(attrs map[string]any)` | Attaches initial key-value attributes at span creation, in addition to `Span.SetAttributes` later. |

## Limitations

- **`SampleRate` zero value**: an unset `SampleRate` (Go zero value `0`) is treated as "not configured" and defaults to `1.0` (sample everything); there is currently no way to explicitly configure "sample nothing" via a zero-valued `Config`. Use `NewNoOp()` to disable tracing entirely.
- **No baggage propagation**: only the W3C `traceparent`/`tracestate` trace-context headers are propagated; OTel baggage is out of scope for v1.
- **Attribute value types**: `SetAttributes`/`WithAttributes` convert `string`, `bool`, `int`, `int64`, and `float64` natively; any other type is rendered via `fmt.Sprint`.

## Dependencies

- [go.opentelemetry.io/otel](https://pkg.go.dev/go.opentelemetry.io/otel) + `/trace`, `/sdk`, `/exporters/otlp/otlptrace/otlptracegrpc`, `/propagation` – tracing SDK and OTLP/gRPC exporter.
- [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc) – transport for the OTLP exporter.

## See also

- [ctxkit](../ctxkit/README.md) – `WithTraceID`/`TraceID` carry the trace id through context and into logs.
- [httpkit](../httpkit/README.md) – `middleware.Tracing` is the standard server-side integration point.
- [logger](../logger/README.md) – `ctxkit.LoggerExtractor()` surfaces `trace_id` on every log line once `Tracing` has run.
