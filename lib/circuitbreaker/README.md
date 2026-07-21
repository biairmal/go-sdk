# Circuitbreaker Package

`circuitbreaker` protects calls to an unreliable dependency with a lean, dependency-free closed→open→half-open state machine: it lets calls through while healthy, trips to reject calls immediately once failures cross a configured threshold, and later admits a bounded number of probe calls to test recovery before closing again.

## Overview

Wrapping an outbound call (an HTTP request to another service, a flaky DB replica, anything that can fail systemically) in a `Breaker` stops a struggling dependency from being hammered by retries, and stops your own process from burning goroutines and connections waiting on calls that are very likely to fail. `circuitbreaker` implements its own small state machine — no third-party dependency — so behavior stays fully under this SDK's control. It composes naturally with `ratelimit` (protects *you* from callers) and `auth`/`tracer`/`metrics` at the other layers of the stack.

## Features

- **Own closed→open→half-open state machine**: no external dependency; `sync.Mutex`-guarded and safe for concurrent use.
- **Two independent trip conditions**: `FailureThreshold` consecutive failures (good under low traffic) or `FailureRatio` once at least `FailureThreshold` requests have been observed (good under sustained partial failure that never strings `FailureThreshold` failures back to back).
- **Bounded Half-Open probing**: `HalfOpenMaxCalls` caps concurrent probe calls and doubles as the number of consecutive successes required to close again; a single probe failure reopens immediately.
- **Stale-result safe**: an in-flight call whose outcome arrives after the breaker has already transitioned (e.g. a concurrent probe reopened it first) is silently ignored rather than corrupting the new state.
- **Panic-safe**: a panic from the wrapped call is recorded as a failure and then re-raised — `Execute` never swallows it.
- **`Do[T]`**: a generic free function wrapping `Execute` for calls that return a value in addition to an error.
- **`ErrOpen` → clean 503**: wraps `errorz.ErrServiceUnavailable`, so it resolves to HTTP 503 via `httpkit`'s `StatusCodeFromError` with no extra wiring.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/circuitbreaker
```

### Basic usage

```go
package main

import (
    "context"
    "errors"
    "fmt"

    "github.com/biairmal/go-sdk/lib/circuitbreaker"
)

func main() {
    b := circuitbreaker.NewBreaker(circuitbreaker.DefaultConfig())

    err := b.Execute(context.Background(), func(ctx context.Context) error {
        return callFlakyDependency(ctx)
    })
    if errors.Is(err, circuitbreaker.ErrOpen) {
        fmt.Println("dependency looks unhealthy; falling back")
        return
    }
    if err != nil {
        fmt.Println("call failed:", err)
    }
}

func callFlakyDependency(ctx context.Context) error { return nil }
```

### Calls that return a value

```go
user, err := circuitbreaker.Do(ctx, b, func(ctx context.Context) (User, error) {
    return userClient.Get(ctx, id)
})
```

### Classifying failures

By default any non-nil error trips the breaker. Override this so client errors (bad input) don't count against a dependency that's actually healthy:

```go
b := circuitbreaker.NewBreaker(cfg, circuitbreaker.WithIsSuccessful(func(err error) bool {
    if err == nil {
        return true
    }
    var ez *errorz.Error
    if errors.As(err, &ez) && ez.Code == errorz.CodeBadRequest {
        return true // the dependency is fine; the caller sent bad input
    }
    return false
}))
```

### Observing state transitions

```go
b := circuitbreaker.NewBreaker(cfg, circuitbreaker.WithOnStateChange(func(from, to circuitbreaker.State) {
    log.WithContext(ctx).Warn("circuit breaker state changed",
        logger.Field{Key: "from", Value: from.String()},
        logger.Field{Key: "to", Value: to.String()},
    )
    rec.GaugeAdd("circuitbreaker_open", stateDelta(from, to), metrics.Labels{"dependency": "payments"})
}))
```

## Options

| Option | Description |
|--------|-------------|
| `WithOnStateChange(fn func(from, to State))` | Callback invoked after every real state transition, outside the internal lock. Default: none. |
| `WithIsSuccessful(fn func(err error) bool)` | Classifies `Execute`'s error as a breaker success/failure. Default: `err == nil`. |
| `WithClock(now func() time.Time)` | Overrides the time source. Intended for deterministic tests of `OpenTimeout` expiry; leave unset in production. Default: `time.Now`. |

## Limitations

- **`NewBreaker` doesn't call `Config.Validate()` itself**: zero-valued fields are filled from `DefaultConfig()`, so a zero `Config` always works, but a config loaded from YAML with genuinely invalid values (e.g. a negative `FailureThreshold`) is not rejected at construction — call `cfg.Validate()` yourself after `config.Load` if you need that.
- **`FailureRatio` reuses `FailureThreshold` as its minimum sample size**: there's no separate "minimum requests" field. This keeps `Config` matching the plan's field list, but means raising `FailureThreshold` also delays when the ratio rule starts being evaluated.
- **`State()` lazily resolves an elapsed `OpenTimeout`** the same way `Execute` does (moving Open→Half-Open as a side effect of the read) — a health check that only calls `State()` will still observe the transition without needing to drive a real call through `Execute`.
- **No distributed/shared state**: each `Breaker` instance tracks failures for its own process. Behind N replicas, each has an independent circuit; there's no shared Redis-backed variant (unlike `ratelimit`).

## Dependencies

None beyond the standard library. Uses `errorz` (foundational leaf) for `ErrOpen`.

## See also

- [ratelimit](../ratelimit/README.md) – the complementary building block: protects *you* from too many inbound callers, where `circuitbreaker` protects *you* from a struggling outbound dependency.
- [errorz](../errorz/README.md) – the error type `ErrOpen` wraps and that `StatusCodeFromError` maps to a 503.
- [metrics](../metrics/README.md) / [logger](../logger/README.md) – natural targets for `WithOnStateChange`.
