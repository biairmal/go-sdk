# Ctxkit Package

`ctxkit` owns the canonical request-scoped context keys shared across the SDK — request ID, correlation ID, trace ID, and user ID — and provides typed accessors plus a ready-made logger extractor so those values appear in every log line automatically.

## Overview

Cross-cutting middleware (request ID, correlation, tracing, auth) each need to stash a small value in `context.Context` and have it surface later in logs and downstream calls. Without a shared convention every package invents its own key, and the `logger` extractor has to guess at string keys that may not match. `ctxkit` fixes that: one unexported, typed key per field, one setter/getter pair, and one `LoggerExtractor()` that the logger reads. Producers write with `WithX`; the logger and any consumer read with `X`.

## Features

- **Canonical typed keys**: four request-scoped fields (`request_id`, `correlation_id`, `trace_id`, `user_id`) behind an unexported key type, so values never collide with other packages' context keys.
- **Empty-safe setters**: `WithX(ctx, "")` returns the context unchanged, so callers never overwrite a value with an empty string.
- **Logger integration**: `LoggerExtractor()` returns a `logger.ContextExtractor` that emits exactly the fields present — wire it once at logger construction.
- **One-way dependency**: `ctxkit` imports `logger`, never the reverse, keeping `logger` a leaf package.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/ctxkit
```

### Basic usage

```go
package main

import (
    "context"

    "github.com/biairmal/go-sdk/ctxkit"
    "github.com/biairmal/go-sdk/logger"
)

func main() {
    // Build the logger so it surfaces canonical context fields automatically.
    log := logger.NewZerolog(&logger.Options{
        ContextExtractor: ctxkit.LoggerExtractor(),
    })

    ctx := ctxkit.WithRequestID(context.Background(), "req-123")
    ctx = ctxkit.WithUserID(ctx, "user-42")

    // Logs include request_id=req-123 and user_id=user-42.
    log.InfoWithContext(ctx, "processing request")
}
```

### Reading values

```go
reqID := ctxkit.RequestID(ctx)         // "" if absent
userID := ctxkit.UserID(ctx)
traceID := ctxkit.TraceID(ctx)
corrID := ctxkit.CorrelationID(ctx)
```

In the standard middleware chain the values are set for you: `middleware.RequestID` → `request_id`,
`middleware.Correlation` → `correlation_id`, `middleware.Tracing` → `trace_id`, `middleware.Auth` → `user_id`.

## Limitations

- **String values only**: the canonical fields are strings by design (ids); richer payloads should use your own context keys.
- **Fixed field set**: the four canonical fields are intentionally not extensible here — add domain-specific values with your own typed keys and a custom `logger.ContextExtractor` if needed.

## See also

- [logger](../logger/README.md) – consumes `LoggerExtractor()` to surface fields in every `*WithContext` call.
- [httpkit](../httpkit/README.md) – `middleware.RequestID`, `middleware.Correlation`, `middleware.Tracing`, and `middleware.Auth` are the standard producers.
