# Architecture

Reference for *why* the SDK is shaped the way it is. The enforceable rules live in [../AGENTS.md](../AGENTS.md); this document explains the reasoning behind them.

## Modular, no entry point

The SDK is a set of **independent sub-packages**. There is no root package to import and no initialization order. A consumer takes `errorz` without pulling in `sqlkit`, or `redis` without `httpkit`. This keeps the dependency surface small for each consumer and lets packages evolve independently.

**Implication for authors:** avoid cross-package imports between siblings unless there is a clear reason. New shared types usually belong in a small leaf package (e.g. `lib/common/dto`) rather than creating a web of interdependencies.

## Layering & dependency direction

A typical consumer (e.g. `guest-management-be`) stacks the packages like this:

```
HTTP boundary        httpkit (handler adapter, middleware, response envelope)
      │  maps errorz codes → HTTP status (the only HTTP-specific step)
Application/service   (consumer code: business logic, transaction orchestration)
      │
Data access          repository  →  repository/sql, repository/cache
      │
Infrastructure       sqlkit (database/sql, leader/follower)   redis (go-redis)
```

Dependencies point **downward** only. Lower layers never import the HTTP layer and never know about HTTP *transport*. They may, however, produce `errorz` errors — see below.

## Errors: errorz as the ecosystem-wide error type

`errorz` is the shared structured-error type for the whole ecosystem, **not** an HTTP-layer concern. Every layer and every SDK package may return `*errorz.Error`: `sqlkit`, `redis`, `repository`, `config`, and consumer usecase / infrastructure / domain code alike.

Its codes (`CodeNotFound`, `CodeBadRequest`, …) are a **transport-agnostic taxonomy**. `httpkit` maps them to HTTP status (`NotFound` → 404, `BadRequest` → 400, …) at the very edge, but the same taxonomy could map to gRPC status codes, CLI exit codes, or a message-queue dead-letter reason. The mapping lives at the transport boundary; the error type itself travels through every layer.

Guidance:

- Return an `*errorz.Error` with an appropriate **code**, and **wrap the underlying cause** so the chain is preserved (`errors.Is` / `errors.As` keep working). Don't discard the original error.
- Compare sentinels with `errors.Is(err, errorz.ErrNotFound)` — sentinels travel; type assertions don't.
- Because the SDK packages now depend on `errorz`, it is a **foundational leaf package** (alongside `logger` and `common/dto`, all under `lib/`) that the modular [package-independence](#modular-no-entry-point) rule explicitly permits as a shared dependency.

See the canonical snippets in [PATTERNS.md](PATTERNS.md#sentinel-errors-errorz).

## Logging responsibility

Libraries **return errors instead of logging them**, so callers stay in control of output. `logger.Logger` is accepted only as an *optional* constructor parameter, and only by packages with internal state worth tracing (query execution in `repository/sql`, connection lifecycle in `sqlkit`). It is always nil-safe: a nil logger means silence, never a panic.

Stdlib `log` is banned in library code because callers cannot suppress or redirect it.

## Key infrastructure behaviours

- **`sqlkit.DB` leader/follower** — `Leader()` always returns the write connection; `Follower()` does round-robin across healthy followers and falls back to the leader when all followers are unhealthy. Health checks run in a background goroutine, off the hot path.
- **Transaction injection** — `sqlkit.InjectTx(ctx, tx)` puts a `*sql.Tx` on the context; `sqlkit.ExtractTx(ctx)` retrieves it. Repositories route through `GetConnection`/`GetReadConnection` so an injected transaction is used transparently when present.
- **`repository/sql` reflection** — `SQLRepository` reads `db:"column_name"` struct tags at construction. ID auto-detection: zero `int64` → `LastInsertId`; zero UUID/string → `RETURNING id` (Postgres); non-zero → explicit insert.
- **`httpkit` handler adapter** — handlers are `func(*http.Request) (any, error)`, converted to `http.HandlerFunc` by `handler.Handle`. On error, `StatusCodeFromError` maps `errorz` codes to HTTP status; on success, returning `*response.Success` lets a handler set a custom status code.
