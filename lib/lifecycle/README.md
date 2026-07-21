# Lifecycle Package

`lifecycle` drives ordered, deadline-bounded graceful shutdown for an `http.Server` and whatever else your process owns — the last piece of wiring that turns "the process exits" into "the process stops taking new traffic, finishes what it started, and cleans up in order, without hanging forever."

## Overview

`Run` blocks until a shutdown signal (or a canceled context) arrives, then drives a fixed sequence: flip a readiness flag so `httpkit.Readiness` starts returning 503 (load balancers stop routing), run an optional pre-shutdown hook, wait out a drain delay so that readiness change actually propagates, drain in-flight HTTP requests via `srv.Shutdown`, then close every registered resource (DB, Redis, tracer flush, …) sequentially in registration order. A second shutdown signal at any point abandons the remaining budget, hard-closes the server, and returns immediately — the operator's "stop being polite" escape hatch. `Config` holds only the three timing knobs; everything else (the readiness flag, signal set, logger, closers) is a live object or func, so it's wired via options, not YAML.

## Features

- **LB-safe drain delay**: a configurable pause between flipping readiness and calling `srv.Shutdown`, so a load balancer / k8s endpoint controller has time to observe the 503 and stop routing before the listener actually stops accepting connections.
- **Split timeouts**: `ShutdownTimeout` (HTTP drain) and `CloserTimeout` (resource cleanup) are independent budgets — a slow HTTP drain under load can't silently starve DB/Redis/tracer cleanup down to zero.
- **Ordered, logged closers**: registered `Closer`s run sequentially in registration order (not concurrently, not LIFO), with each one's duration and outcome logged via the optional `logger.Logger`.
- **Forced exit on a second signal**: a second SIGINT/SIGTERM (or a second programmatic trigger) at any point abandons the remaining budget, hard-closes the server, and returns `ErrForcedShutdown` immediately instead of waiting out the configured timeouts.
- **Testable without real OS signals**: `Run` can be triggered by canceling the passed-in `context.Context` as well as by an OS signal, so tests (and programmatic shutdown paths) don't need to send real signals.
- **Structural closer adapters**: `CloserFromTracer`/`CloserFromDB`/`CloserFromRedis` adapt any type with a matching `Shutdown(ctx) error` or `Close() error` method — no dependency on `tracer`, `sqlkit`, or `redis`, so `lifecycle` stays import-free of its siblings.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/lifecycle
```

### Basic usage

```go
package main

import (
    "context"
    "errors"
    "net/http"
    "sync/atomic"

    "github.com/biairmal/go-sdk/lib/errorz"
    "github.com/biairmal/go-sdk/lib/httpkit"
    "github.com/biairmal/go-sdk/lib/lifecycle"
    "github.com/biairmal/go-sdk/lib/logger"
)

func main() {
    log := logger.NewZerolog(&logger.Options{})

    var ready atomic.Bool
    ready.Store(true)

    mux := http.NewServeMux()
    mux.HandleFunc("/readyz", httpkit.Readiness(func(_ context.Context) error {
        if !ready.Load() {
            return errorz.ServiceUnavailable()
        }
        return nil
    }))

    srv := &http.Server{Addr: ":8080", Handler: mux}
    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Error("listen failed", logger.F("error", err.Error()))
        }
    }()

    err := lifecycle.Run(context.Background(), srv, lifecycle.DefaultConfig(),
        lifecycle.WithReadiness(&ready),
        lifecycle.WithLogger(log),
    )
    if err != nil {
        log.Error("shutdown finished with errors", logger.F("error", err.Error()))
    }
}
```

### Closing dependencies in order

```go
err := lifecycle.Run(ctx, srv, cfg,
    lifecycle.WithReadiness(&ready),
    lifecycle.WithLogger(log),
    // Registration order = close order: tracer flush first (captures spans
    // from requests that just finished draining), then redis, then db last
    // (the most foundational dependency, kept alive longest).
    lifecycle.WithCloser("tracer", lifecycle.CloserFromTracer(tr)),
    lifecycle.WithCloser("redis", lifecycle.CloserFromRedis(rdb)),
    lifecycle.WithCloser("db", lifecycle.CloserFromDB(db)),
)
if errors.Is(err, lifecycle.ErrForcedShutdown) {
    // a second signal cut shutdown short; some closers may not have run
}
```

### Programmatic shutdown (no OS signal)

`Run` also returns once the passed-in `context.Context` is done, so a process can trigger the same graceful sequence from code (e.g. an admin endpoint, a supervisor) instead of an OS signal:

```go
ctx, cancel := context.WithCancel(context.Background())
// later, from anywhere:
cancel() // triggers the same shutdown sequence as a signal
```

## Options

| Option | Description |
|--------|-------------|
| `WithReadiness(ready *atomic.Bool)` | Flips `ready` to `false` as soon as shutdown starts. Default: no readiness flag (flip skipped). |
| `WithSignals(sig ...os.Signal)` | Overrides the OS signals that trigger shutdown. Default: `os.Interrupt`, `syscall.SIGTERM`. |
| `WithLogger(log logger.Logger)` | Optional logger for shutdown progress (signal received, hook failure, per-closer duration/outcome, forced-exit warning). Default: silent. |
| `WithCloser(name string, c Closer)` | Registers a named `Closer`, run in the closer phase in registration order. Repeatable; a nil `Closer` is ignored. |
| `WithShutdownFunc(fn func(ctx context.Context) error)` | A hook run once, synchronously, right after the readiness flip and before the drain delay. Not bounded by `ShutdownTimeout`/`CloserTimeout` — keep it fast. |

## Limitations

- **`CloserFromDB`/`CloserFromRedis` ignore the deadline for the underlying call**: `*sqlkit.DB.Close()` and `redis.Client.Close()` take no `context.Context`, so `CloserTimeout` can't preempt a hanging close — it can only stop `Run` from waiting on it (via a second signal), not cancel the call itself.
- **Structural adapters don't catch a typed-nil pointer**: `CloserFromTracer(nil)`/`CloserFromDB(nil)`/`CloserFromRedis(nil)` correctly return a nil `Closer` for a literal `nil`, but a `nil` value of a concrete pointer type (e.g. a `*sqlkit.DB` variable that's nil) passed through the interface parameter is not caught — Go's usual typed-nil caveat.
- **No distributed coordination**: each process runs its own independent shutdown sequence; there's no shared "drain all replicas together" behavior (not needed — each replica's LB entry is independent).
- **A forced (second-signal) exit may skip closers entirely**: if the second signal arrives during the drain delay or the HTTP shutdown phase, `Run` returns before the closer phase ever starts — resources registered via `WithCloser` are not closed. This is intentional (the operator asked to stop waiting), but it means state cleanup on a forced exit is best-effort at the OS level, not this package's.

## Dependencies

None beyond the standard library.

## See also

- [httpkit](../httpkit/README.md) – `httpkit.Readiness` is the natural pairing for `WithReadiness`; its handler reads the same flag `Run` flips.
- [tracer](../tracer/README.md) / [sqlkit](../sqlkit/README.md) / [redis](../redis/README.md) – typical `Closer` targets via `CloserFromTracer`/`CloserFromDB`/`CloserFromRedis`.
- [logger](../logger/README.md) – the target of `WithLogger`.
- [errorz](../errorz/README.md) – the error type `Config.Validate()` returns.
