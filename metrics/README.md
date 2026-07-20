# Metrics Package

`metrics` provides interface-first application metrics: a `Recorder` records counters, histograms, and gauges by name, a [Prometheus](https://prometheus.io/) backend exposes them for scraping, and a `NoOp` backend is available for tests and environments where metrics are disabled.

## Overview

Services need cheap, aggregate visibility into request volume, error rate, and latency — the metrics pillar of observability, alongside [`tracer`](../tracer/README.md) (traces) and `logger` (logs). `metrics` wraps [`prometheus/client_golang`](https://github.com/prometheus/client_golang) behind a small `Recorder` interface so `httpkit/middleware` (and application code) never imports Prometheus types directly. `httpkit/middleware.Metrics` records the standard [RED-method](https://www.weave.works/blog/the-red-method-key-metrics-for-microservices-architecture/) HTTP metrics — request count, duration, in-flight — for every request with no extra wiring.

## Features

- **Interface-first**: `Recorder` hides the Prometheus client; swap backends (`NewPrometheus`, `NewNoOp`) without touching call sites.
- **Dynamic metric registration**: call `CounterInc`/`HistogramObserve`/`GaugeAdd` with any metric name — the Prometheus backend registers it on first use with the label keys you pass, and caches it for reuse.
- **Pre-registered HTTP metrics**: `NewPrometheus` eagerly registers `http_requests_total` (counter), `http_request_duration_seconds` (histogram), and `http_requests_in_flight` (gauge) so the first request never pays a registration cost.
- **Fire-and-forget**: `Recorder` methods have no error return — a metrics call never fails the caller's request. Backend-level failures (e.g. a metric name reused with a different label set) are logged via an optional `logger.Logger`, never propagated.
- **`NoOp` backend**: `NewNoOp()` discards everything — use it in tests or to disable metrics without branching call sites.
- **Cardinality control**: `httpkit/middleware.Metrics` accepts a `PathNormalizer` so path-parameterized routes (`/orders/42`) don't mint a new Prometheus time series per id.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/metrics
```

### Basic usage

```go
package main

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/biairmal/go-sdk/metrics"
)

func main() {
    rec, err := metrics.NewPrometheus(metrics.Config{Namespace: "orders"})
    if err != nil {
        panic(err)
    }

    rec.CounterInc("orders_processed_total", metrics.Labels{"status": "ok"})

    // Expose the default registry for Prometheus to scrape.
    http.Handle("/metrics", promhttp.Handler())
    _ = http.ListenAndServe(":8080", nil)
}
```

### HTTP server middleware

```go
handler := middleware.Chain(mux,
    middleware.Metrics(rec, nil), // outermost: counts every request, incl. panics
    middleware.Recover(),
    middleware.RequestID(),
    middleware.Logging(log, nil),
)
```

Place `Metrics` outermost so a panic recovered further in the chain is still counted against the in-flight gauge and recorded with its final (500) status code.

### Normalizing paths to control cardinality

```go
handler := middleware.Chain(mux,
    middleware.Metrics(rec, &middleware.MetricsOptions{
        PathNormalizer: func(r *http.Request) string {
            return routeName(r) // e.g. from your router's matched pattern: "/orders/{id}"
        },
    }),
    // ...
)
```

Without a `PathNormalizer`, the raw URL path is used as the `path` label — every distinct id in a path-parameterized route (`/orders/42`, `/orders/43`, ...) mints a new Prometheus time series, which can exhaust memory on the scraper. Always supply one backed by your router's matched route pattern in production.

### Disabling metrics

```go
rec := metrics.NewNoOp() // same Recorder interface; every call is a no-op
```

## Options

| Option | Description |
|--------|-------------|
| `WithRegisterer(r prometheus.Registerer)` | Registers metrics against `r` instead of `prometheus.DefaultRegisterer`. Nil-safe. Tests typically pass an isolated `prometheus.NewRegistry()` so parallel tests don't collide on metric names. |
| `WithLogger(log logger.Logger)` | Optional logger for internal diagnostics (e.g. a metric name reused with a different label set). Nil-safe; defaults to silent. |

## Limitations

- **Label-key consistency**: the first call to `CounterInc`/`HistogramObserve`/`GaugeAdd` for a given metric name fixes its label keys; later calls with a different key set fail at the Prometheus client level and are logged, not applied. Keep a metric's label set constant across all call sites.
- **No metric-name validation at call time**: `Recorder` methods don't reject malformed Prometheus metric names up front — a bad name only surfaces (via the optional logger) when the client library rejects registration or collection.
- **Histogram buckets are fixed at construction**: `Config.HTTPBuckets` applies to `http_request_duration_seconds` only; custom histograms created dynamically via `HistogramObserve` use the same `HTTPBuckets` boundaries (there's no per-metric bucket override).

## Dependencies

- [github.com/prometheus/client_golang](https://github.com/prometheus/client_golang) – Prometheus metrics client and registry.

## See also

- [tracer](../tracer/README.md) – distributed tracing; the other cross-cutting observability signal alongside metrics.
- [httpkit](../httpkit/README.md) – `middleware.Metrics` is the standard server-side integration point.
- [errorz](../errorz/README.md) – the error type `Config.Validate()` returns.
