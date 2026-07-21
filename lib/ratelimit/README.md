# Ratelimit Package

`ratelimit` provides interface-first rate limiting: a `Limiter` checks whether a keyed request is allowed under a configured rate, with an in-memory token-bucket backend for a single monolith instance and a Redis sliding-window backend for scale-out deployments sharing one limit across replicas.

## Overview

Rate limiting protects a service (and its downstream dependencies) from being overwhelmed by a single caller, whether that's a misbehaving client, a retry storm, or abuse. `ratelimit` wraps two well-understood algorithms behind a small `Limiter` interface so `httpkit/middleware` (and application code) never needs to know which backend is in play: [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate) (token bucket) for a single process, and a Redis-backed sliding-window log for a fleet of replicas that must share one combined quota. `httpkit/middleware.RateLimit` is the standard HTTP integration — it derives a key per request, checks the limiter, and responds with the standard rate-limit headers.

## Features

- **Interface-first**: `Limiter` hides the backend; swap `NewInMemory`/`NewRedis`/`FromConfig` without touching call sites.
- **In-memory token bucket**: `NewInMemory` gives each key a continuous refill rate (`Rate` permits/sec) and burst capacity (`Burst`), backed by `golang.org/x/time/rate`. Idle keys are evicted opportunistically so memory stays bounded under a churning key space (e.g. per-IP limits).
- **Redis sliding-window**: `NewRedis` allows `Burst` requests per `Window`, counted with a Lua script (`ZADD`/`ZREMRANGEBYSCORE`/`ZCARD` run atomically in one round trip) so the limit is enforced consistently across every process pointed at the same Redis — no race between the check and the increment.
- **Config-driven backend selection**: `FromConfig` picks the backend from `Config.Backend` ("memory" or "redis") so switching deployment topology is a config change, not a code change.
- **Retry-aware `Result`**: every `Allow` call returns `Limit`/`Remaining`/`RetryAfter`, enough to set standard rate-limit response headers without recomputing anything.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/ratelimit
```

### Basic usage

```go
package main

import (
    "context"
    "fmt"

    "github.com/biairmal/go-sdk/lib/ratelimit"
)

func main() {
    l := ratelimit.NewInMemory(ratelimit.Config{Rate: 10, Burst: 20, MaxKeys: 10_000})

    result, err := l.Allow(context.Background(), "user:42")
    if err != nil {
        // backend error (only possible with the redis backend) — decide fail-open/closed
        panic(err)
    }
    if !result.Allowed {
        fmt.Printf("denied, retry after %s\n", result.RetryAfter)
        return
    }
    fmt.Println("allowed, remaining:", result.Remaining)
}
```

### Redis backend (shared across replicas)

```go
client, _ := redis.NewClient(redis.DefaultConfig())
l := ratelimit.NewRedis(client, ratelimit.Config{Burst: 100, Window: time.Minute}) // 100 req/min, shared
```

### Selecting the backend from config

```go
l, err := ratelimit.FromConfig(ratelimit.Config{
    Backend: "redis",
    Burst:   100,
    Window:  time.Minute,
}, redisClient) // redisClient is required (non-nil) only when Backend is "redis"
```

### HTTP server middleware

```go
handler := middleware.Chain(mux,
    middleware.RateLimit(limiter, middleware.KeyByIP),
    middleware.Auth(validator, middleware.WithPolicy(pol)),
)
```

Place `RateLimit` before `Auth` when keying by IP (protects the auth check itself from being hammered); use `KeyByUser` after `Auth` instead when you want per-authenticated-user quotas. See [httpkit/README.md](../httpkit/README.md) for the full middleware chain.

## Limitations

- **In-memory limit is per-process**: `NewInMemory` does not coordinate across replicas — behind a load balancer with N instances, the effective limit is up to N× the configured one. Use the redis backend when a single combined quota across replicas matters.
- **Redis backend requires `Eval` support**: it needs a `redis.Client` whose `Eval` reaches a real Redis server (Lua scripting); it will not work against a command-subset stand-in that doesn't implement `EVAL`.
- **No built-in fail-open/closed policy in the package itself**: `Limiter.Allow` returns the backend error as-is; the caller (or `httpkit/middleware.RateLimit`) decides whether to fail open or closed. The middleware defaults to fail-open.
- **`Config.Rate` is memory-only, `Config.Window` is redis-only**: the two backends use different subsets of `Config`'s fields (see the field docs on `Config`); setting the "wrong" one for your backend is silently ignored rather than rejected.

## Dependencies

- [golang.org/x/time](https://pkg.go.dev/golang.org/x/time/rate) – the in-memory token-bucket implementation.
- [redis](../redis/README.md) – the Redis backend reuses the SDK's `redis.Client`.

## See also

- [httpkit](../httpkit/README.md) – `middleware.RateLimit` is the standard server-side integration point.
- [redis](../redis/README.md) – `redis.Client`, including the `Eval` method used by the redis backend.
- [errorz](../errorz/README.md) – the error type `Config.Validate()` and `FromConfig` return.
