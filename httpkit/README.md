# httpkit Package

Router-agnostic HTTP utilities for Go: handler adapter, middlewares (recover, logging, request ID), response envelope, error-to-HTTP mapping for [errorz](../errorz/), health and readiness handlers, and a thin client. All public APIs use only `net/http` types (`http.Handler`, `http.HandlerFunc`, `func(http.Handler) http.Handler`), so they work with the standard library and with [go-chi](https://github.com/go-chi/chi) without extra adapters.

## Overview

- **Response**: `BaseResponse[T]`, `ErrorPayload`, `JSON()`, and success helpers (`OK`, `Created`, `NoContent`) for a consistent API envelope.
- **Handler**: `handler.Func` and `handler.Handle` convert a function `func(*http.Request) (any, error)` into an `http.HandlerFunc`; errors are mapped to HTTP status via errorz codes and written with the same envelope.
- **Middleware**: `Chain`, `Recover`, `Logging` (with optional request/response and body logging), and `RequestID`; all have signature `func(http.Handler) http.Handler`.
- **Health / Readiness**: `Health()` always returns 200 (liveness); `Readiness(check)` returns 200 if `check(ctx)` is nil, otherwise 503.
- **Client**: Thin client that decodes responses into `response.BaseResponse[T]`.

## Response format

Success and error use the same envelope shape:

- **Success**: `BaseResponse` with `Data` set, `Error` nil, `Code` "OK", `Message` "success", and `Timestamp`.
- **Error**: `BaseResponse` with `Error` set to `ErrorPayload` (code, message, source_system, meta), `Data` nil, and `Code` "ERROR".

## Handler usage

Use `handler.Handle(handler.Func)` so your handler returns `(any, error)` and the adapter writes the envelope and sets status:

```go
import (
    "net/http"
    "github.com/biairmal/go-sdk/httpkit/handler"
    "github.com/biairmal/go-sdk/httpkit/response"
    "github.com/biairmal/go-sdk/errorz"
)

// Success: return response.OK(data), response.Created(data), or response.NoContent().
h := handler.Handle(func(r *http.Request) (any, error) {
    return response.OK(map[string]string{"pong": "ok"}), nil
})

// Error: return errorz errors; status is derived from errorz code.
h := handler.Handle(func(r *http.Request) (any, error) {
    return nil, errorz.NotFound()
})
```

## Middleware order

Apply middlewares so that the first in the list is the outermost (runs first on request, last on response). Recommended order: **Recover**, then **RequestID** (optional), then **Logging**.

- **Recover**: Catches panics and writes a 500 response with the error envelope.
- **RequestID**: Injects or reads `X-Request-Id`, puts it in context; use `middleware.RequestIDKey` with your logger’s ContextExtractor.
- **Logging**: Logs request and/or response (path, IP, method, status, duration, optional body). Use `Logging(log, opts)`; if `opts` is nil, request and response with body are logged. Set `LoggingOptions.LogRequest`, `LogResponse`, `LogRequestBody`, `LogResponseBody`, and `MaxBodyBytesForLogging` to tune.

## Health and readiness

- **Health (liveness)**: `httpkit.Health()` — always 200, optional JSON body `{"status":"ok"}`.
- **Readiness**: `httpkit.Readiness(check)` — runs `check(ctx)`; 200 if nil, 503 if non-nil (body uses the same error envelope).

## Mounting examples

### Standard library

```go
import (
    "net/http"
    "github.com/biairmal/go-sdk/httpkit"
    "github.com/biairmal/go-sdk/httpkit/handler"
    "github.com/biairmal/go-sdk/httpkit/middleware"
    "github.com/biairmal/go-sdk/httpkit/response"
    "github.com/biairmal/go-sdk/logger"
)

mux := http.NewServeMux()
mux.HandleFunc("/health", httpkit.Health())
mux.HandleFunc("/ready", httpkit.Readiness(nil))
mux.Handle("/api/ping", handler.Handle(func(r *http.Request) (any, error) {
    return response.OK(map[string]string{"pong": "ok"}), nil
}))

log := logger.NewZerolog(&logger.Options{...})
h := middleware.Chain(mux,
    middleware.Recover(),
    middleware.Logging(log, nil),
)
server := &http.Server{Handler: h}
```

### go-chi

```go
import (
    "github.com/go-chi/chi/v5"
    "github.com/biairmal/go-sdk/httpkit"
    "github.com/biairmal/go-sdk/httpkit/handler"
    "github.com/biairmal/go-sdk/httpkit/middleware"
    "github.com/biairmal/go-sdk/httpkit/response"
    "github.com/biairmal/go-sdk/logger"
)

r := chi.NewRouter()
r.Use(middleware.Recover(), middleware.Logging(log, nil))
r.Get("/health", httpkit.Health())
r.Get("/ready", httpkit.Readiness(nil))
r.Get("/api/ping", handler.Handle(func(r *http.Request) (any, error) {
    return response.OK(map[string]string{"pong": "ok"}), nil
}))

server := &http.Server{Handler: r}
```

## Error-to-HTTP mapping

`httpkit.StatusCodeFromError(err)` (and `handler.StatusCodeFromError(err)`) maps errorz codes to HTTP status (e.g. `ERR_NOT_FOUND` → 404, `ERR_BAD_REQUEST` → 400). Unknown codes and non-errorz errors yield 500. The handler adapter and recover middleware use this automatically.

## Client

Use `client.New(nil)` for default client, or pass your own `*http.Client`. `client.Get[T]`, `client.Post[T]`, and `client.Do[T]` build the request, perform it, and decode the body into `response.BaseResponse[T]`. They also return status code and raw body for callers that need them. Error response bodies can be decoded into the same envelope shape used by the server.
