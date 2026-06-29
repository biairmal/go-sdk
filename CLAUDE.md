# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# First-time setup
make install-tools       # install gofumpt, golangci-lint, govulncheck

# CI (runs format → lint-fix → test-unit → coverage → deps-verify, fail-fast)
# Always run this before considering any implementation complete.
make check               # or: make ci

# Tests
make test-unit           # go test -short ./...
make test-integration    # go test ./...   (includes Redis integration tests)
make test-race           # go test -race -short ./...

# Single package/test
go test -short ./repository/...
go test -run TestName ./redis/...

# Coverage
make coverage            # writes out/coverage.out and out/coverage.html
make coverage-view       # open report in browser

# Format and lint
make format              # gofumpt
make lint-fix            # golangci-lint --fix
make vulncheck           # govulncheck ./...
```

## Architecture

This is a shared Go SDK (module `github.com/biairmal/go-sdk`) — a collection of independent sub-packages with no required entry point. Consumers import only what they need.

### Package map

| Package | Role |
|---|---|
| `config` | Viper-based loader; `.env` loading via godotenv; `${VAR}` substitution in config files |
| `errorz` | `*Error` type with code/message/source/meta; sentinel errors (`ErrNotFound`, …); constructors (`NotFound()`, `BadRequest()`, …) that wrap sentinels for `errors.Is` |
| `httpkit` | Handler adapter, middleware chain, response envelope, health/readiness endpoints, thin HTTP client |
| `logger` | `Logger` interface; Zerolog backend; no-op for tests |
| `sqlkit` | `*DB` wrapper over `database/sql`; leader/follower with round-robin and health fallback; connection pool config; transaction injection |
| `redis` | `redis.Client` interface wrapping go-redis/v9; pipeline support |
| `repository` | Generic interfaces: `Repository[T,ID]`, `ReadRepository`, `WriteRepository`, `TransactionalRepository` |
| `repository/sql` | `SQLRepository[T,ID]` — reflection-based CRUD using `db` struct tags; multi-dialect placeholders; `WithDialect`, `WithSelectColumns`, `WithIDColumn` options |
| `repository/cache` | `CachedRepository[T,ID]` decorator (WIP); wraps any `Repository`; write-through / write-around / write-behind strategies |
| `serializer` | Thin wrappers around `encoding/json` (`ToJSON`, `ParseJSON`) |
| `common/dto` | `PageRequest` / `PageResponse` DTOs |

### Key design patterns

**`httpkit/handler` adapter** — handlers are `func(*http.Request) (any, error)`, converted to `http.HandlerFunc` by `handler.Handle`. On error, `StatusCodeFromError` maps `errorz` codes to HTTP status. On success, returning `*response.Success` lets the handler set a custom HTTP status code.

**`errorz` sentinel pattern** — constructors (`NotFound()`, etc.) return a new `*Error` with `Err` set to a package-level `sentinelError`. Check with `errors.Is(err, errorz.ErrNotFound)`, never by type assertion to sentinel directly.

**`repository/sql` reflection** — `SQLRepository` reads `db:"column_name"` struct tags at construction time. ID auto-detection: zero int64 → `LastInsertId`; zero UUID/string → `RETURNING id` (Postgres); non-zero → explicit insert. `GetConnection`/`GetReadConnection` on `BaseRepository` route writes to leader and reads to follower via `sqlkit.DB`.

**`sqlkit.DB` leader/follower** — `DB.Leader()` always returns the write connection. `DB.Follower()` does round-robin across healthy followers; falls back to leader when all followers are unhealthy. Health checks run in a background goroutine.

**Test conventions** — unit tests use `testing.Short()` guard or `-short` flag. Integration tests (e.g., Redis) live in `*_integration_test.go` files and require a live service. Unit tests must use table-driven style (`[]struct{ ... }` test cases iterated with `t.Run`).

### Cross-cutting rules for new packages

**`errorz`** — use only at the HTTP boundary (`httpkit`). Lower-level packages (`sqlkit`, `redis`, `repository`, `config`) must return plain errors or sentinel errors. The translation to `*errorz.Error` happens in application-layer handlers, not inside the SDK packages themselves. `errorz` codes are HTTP-semantic and don't belong in data or cache layers.

**`logger`** — libraries should return errors, not log. Accept `logger.Logger` as an **optional** constructor parameter (nil = silent) only when the package has internal state transitions genuinely worth tracing (e.g. query execution, connection lifecycle). Never use stdlib `log` — callers cannot suppress it. Packages like `config`, `redis`, and `serializer` should surface errors and leave logging to the caller.

### Linter rules (`.golangci.yml`)

- Max line length: 120 chars
- Max function length: 100 lines / 50 statements
- `errcheck.check-type-assertions: true` — always handle assertion failures
- Test files are exempt from `errcheck`, `gosec`, `funlen`, `gocognit`, `gocyclo`
- `.pb.go` and `.gen.go` files are excluded from linting
