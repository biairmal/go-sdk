# AGENTS.md

**Canonical development guidelines for `github.com/biairmal/go-sdk`.**

This is the single source of truth for how code in this repository must be written — by humans and by AI assistants alike. It is read automatically by AGENTS.md-aware tools (Cursor, Zed, Aider, GitHub Copilot coding agent, Claude Code via [CLAUDE.md](./CLAUDE.md)). If you are an AI agent: **read this file before making any change**, and treat the [Authoring rules](#authoring-rules) and [Definition of Done](#definition-of-done) as hard requirements.

For deeper reference: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) · [docs/PATTERNS.md](docs/PATTERNS.md) · [docs/NEW_PACKAGE_CHECKLIST.md](docs/NEW_PACKAGE_CHECKLIST.md) · human onboarding in [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Orientation

A shared Go SDK (module `github.com/biairmal/go-sdk`, Go 1.25.1) — a collection of **independent sub-packages with no required entry point**. Consumers import only what they need. All public APIs use standard-library types where possible (`http.Handler`, `*sql.DB`, `context.Context`) so the SDK stays framework-agnostic.

### Package map

| Package | Role |
|---|---|
| `config` | Viper-based loader; `.env` loading via godotenv; `${VAR}` substitution in config files |
| `ctxkit` | Canonical typed context keys + accessors (request/correlation/trace/user IDs); `LoggerExtractor()` for automatic log fields |
| `errorz` | `*Error` type with code/message/source/meta; sentinel errors (`ErrNotFound`, …); constructors (`NotFound()`, `BadRequest()`, …) that wrap sentinels for `errors.Is` |
| `httpkit` | Handler adapter, middleware chain, response envelope, health/readiness endpoints, thin HTTP client |
| `logger` | `Logger` interface; Zerolog backend; no-op for tests |
| `sqlkit` | `*DB` wrapper over `database/sql`; leader/follower with round-robin and health fallback; connection pool config; transaction injection |
| `redis` | `redis.Client` interface wrapping go-redis/v9; pipeline support; `Eval` for atomic Lua scripts |
| `repository` | Generic interfaces: `Repository[T,ID]`, `ReadRepository`, `WriteRepository`, `TransactionalRepository` |
| `repository/sql` | `SQLRepository[T,ID]` — reflection-based CRUD using `db` struct tags; multi-dialect placeholders; `WithDialect`, `WithSelectColumns`, `WithIDColumn` options |
| `repository/cache` | `CachedRepository[T,ID]` decorator (WIP); wraps any `Repository`; write-through / write-around / write-behind strategies |
| `serializer` | Thin wrappers around `encoding/json` (`ToJSON`, `ParseJSON`) |
| `validator` | `Validator` interface wrapping go-playground/validator/v10; struct-tag + single-value validation; `errorz` field errors |
| `tracer` | `Tracer`/`Span` interfaces; OTel OTLP/gRPC backend + `NoOp`; W3C propagation; server middleware (`httpkit/middleware.Tracing`) |
| `auth` | `Validator`/`Claims`/`Issuer`; remote (mappable) + HS256/RS256/JWKS; config-driven route `Policy`; TTL cache; token issuing; server middleware (`httpkit/middleware.Auth`) |
| `metrics` | `Recorder` interface; Prometheus backend + `NoOp`; dynamic metric registration; HTTP request count/duration/in-flight middleware (`httpkit/middleware.Metrics`) |
| `ratelimit` | `Limiter` interface; in-memory token bucket (`golang.org/x/time/rate`) + Redis sliding-window (Lua) backends; 429 middleware with headers (`httpkit/middleware.RateLimit`) |
| `common/dto` | `PageRequest` / `PageResponse` DTOs |

> When you add a package, **add a row here** (see [Authoring rules](#authoring-rules)).

---

## Commands

```bash
# First-time setup — installs gofumpt, golangci-lint, govulncheck
make install-tools

# CI gate — runs format → lint-fix → test-unit → coverage → vulncheck → deps-verify (fail-fast).
# MUST pass before any change is considered complete.
make check               # alias: make ci

# Tests
make test-unit           # go test -short ./...
make test-integration    # go test ./...   (includes live-service integration tests)
make test-race           # go test -race -short ./...

# Single package / single test
go test -short ./repository/...
go test -run TestName ./redis/...

# Coverage  → out/coverage.out, out/coverage.html
make coverage
make coverage-view

# Format, lint, vulnerabilities
make format              # gofumpt
make lint-fix            # golangci-lint --fix
make vulncheck           # govulncheck ./...

# Discover everything
make help
```

---

## Authoring rules

These are **MUST**-level unless stated otherwise. They are derived from existing conventions in this codebase, not invented — see [docs/PATTERNS.md](docs/PATTERNS.md) for the canonical templates.

### Package boundaries & layering

- **Keep packages independent.** Don't add imports between sibling SDK packages without a clear reason; a consumer should be able to import one package without dragging in unrelated ones. `errorz`, `logger`, and `common/dto` are the **foundational leaf packages** that other packages and apps may freely depend on.
- **`errorz` is the ecosystem-wide structured error type.** Use it across all layers and SDK packages (`sqlkit`, `redis`, `repository`, `config`, …) and in app usecase / infrastructure / domain code. Its codes are a **transport-agnostic taxonomy**; `httpkit` maps them to HTTP status at the edge, but they are **not** HTTP-only and may map to gRPC, CLI exit codes, etc. Return an `*errorz.Error` with an appropriate code and **wrap the underlying error** to preserve the chain.
- Check sentinel errors with `errors.Is(err, errorz.ErrNotFound)` — **never** by type-asserting the sentinel directly.
- **Public APIs use stdlib types** (`http.Handler`, `*sql.DB`, `context.Context`). Don't leak third-party types into exported signatures unless they are the package's whole purpose (e.g. `redis` wrapping go-redis).

### Logging

- **Libraries return errors; they do not log.** Never use the stdlib `log` package — callers cannot suppress it.
- Accept `logger.Logger` as an **optional, nil-safe** constructor parameter (nil = silent) **only** when the package has internal state transitions genuinely worth tracing (e.g. query execution, connection lifecycle). Packages like `config`, `serializer`, and `redis` should surface errors and leave logging to the caller. Guard every use: `if r.log == nil { return }`.

### API shape

- Data-layer and I/O functions take **`context.Context` as the first parameter**.
- Constructors are named **`NewX`** (e.g. `NewSQLRepository`, `NewDB`).
- **Configuration values go in a `Config` struct, not in `WithX` options** (see [Configuration](#configuration) below). Functional `WithX` options are reserved for **optional dependencies and non-serializable behavior** (loggers, alternate `http.Client`, callbacks, custom funcs). A `WithX` returns an option that mutates the receiver, applied via `for _, opt := range opts { opt(r) }`; options must be nil/zero-safe.
- Prefer **interface before implementation** — define the contract (`Repository[T,ID]`, `redis.Client`) and return it from constructors where it aids testability.
- **Declare the `error` interface in signatures — never the concrete `*errorz.Error`.** Construct `errorz` *values* inside implementations and return them as `error`; callers extract with `errors.As`/`errors.Is` (or predicate helpers like `repository.IsNotFound`). Returning a concrete `*errorz.Error` invites the Go typed-nil bug (a nil `*errorz.Error` is a **non-nil** `error`) and couples every implementer to errorz. This is how `repository.Repository` already works ([repository/repository.go](repository/repository.go)).

### Configuration

Configuration is **struct-first, YAML-first** so a consuming microservice can embed an SDK package's `Config` into its own app config and populate everything from a single file via [`config.Load`](config/config.go) (Viper + mapstructure). This is the established shape for `sqlkit.New(ctx, *Config)`, `redis.NewClient(*Config)`, and `logger.NewZerolog(*Options)`.

- **Expose an exported `Config` struct.** Every serializable field carries a **`mapstructure:"snake_case"`** tag; nested config structs are tagged too. Without tags, mapstructure won't split camelCase (`MaxOpenConns` would only bind to YAML `maxopenconns`, never `max_open_conns`).
- **Non-serializable fields get `mapstructure:"-"`** (funcs, interfaces — e.g. `logger.Options.ContextExtractor`). These are set in code, not YAML.
- **Provide `DefaultConfig()`** with sane defaults, and have `New` fill zero-valued fields from it so YAML only needs to override what differs.
- **Provide `Validate() error`** returning an `errorz` error (e.g. `errorz.BadRequest()`); call it inside `New`.
- **Constructor shape:** pure → `New(cfg Config, opts ...Option)`; I/O → `New(ctx, cfg, opts ...Option)`. **Required live dependencies are positional** constructor params (e.g. a `redis.Client`, `*sqlkit.DB`); `WithX` options are only for **optional** deps and non-serializable behavior.
- **Rule of thumb:** anything YAML can hold → a tagged `Config` field; anything it can't (a live client, a logger, a callback) → a positional param or a `WithX` option.
- `time.Duration` (`"5s"`) and comma-slices decode out of the box — `config.Load` uses Viper's default `Unmarshal` hooks. No per-package decode wiring needed.

### Tests

- Unit tests are **table-driven**: `[]struct{ name string; … }` iterated with `t.Run(tt.name, …)`. Same-package (`package errorz`, not `errorz_test`) unless there's a reason otherwise.
- Guard slow/integration-only code with `testing.Short()`; integration tests that need a live service go in **`*_integration_test.go`** and run under `make test-integration` (no `-short`).
- New behaviour ships with tests in the same change.
- **New exported interfaces get a mock.** Add a `//go:generate mockgen` directive above the interface that writes into `mocks/<pkg>/` (package `mock<pkg>`, a name distinct from the real package), then run `make mocks` and commit the generated files. Ship a hand-written fake/`NoOp` in the package too where a working stand-in beats call verification (like `logger.NewNoOp()`). Mocks live in the separate `mocks` module so gomock stays out of the main module; see [docs/TESTING.md](docs/TESTING.md).

### Documentation

- Every package **MUST** have a package-level doc comment and a `README.md`.
- Package `README.md` files **MUST** follow the section order and conventions in [docs/README_TEMPLATE.md](docs/README_TEMPLATE.md) (Title + description → Overview → Features → Usage → Options → Limitations → Dependencies → See also; optional sections allowed in their defined slots).
- Every new package **MUST** be added to the [package map](#package-map) above.
- Exported types and functions get doc comments.

### Linter compliance (enforced by [`.golangci.yml`](.golangci.yml))

- Line length **≤ 120** (`lll`).
- Function length **≤ 100 lines / 50 statements** (`funlen`); cyclomatic complexity **< 15** (`gocyclo`); cognitive complexity **< 25** (`gocognit`).
- **Always handle type-assertion failures** (`errcheck.check-type-assertions: true`) and blank assignments (`check-blank: true`).
- Error names follow `errname`/`errorlint` (sentinels prefixed `Err…`; wrap/compare with `errors.Is`/`errors.As`).
- File names are `lower_with_underscores.go`. `gofumpt` + `goimports` formatting is mandatory (`make format`).
- Test files are exempt from `errcheck`, `gosec`, `funlen`, `gocognit`, `gocyclo`. `.pb.go` / `.gen.go` are excluded from linting.

---

## Definition of Done

A change is **not complete** until every box is checked:

- [ ] `make check` passes (format, lint-fix, unit tests, coverage, vulncheck, deps-verify).
- [ ] New/changed behaviour has **table-driven tests**; live-service paths covered by `*_integration_test.go`.
- [ ] Touched/new package has an up-to-date **package doc comment and `README.md`**.
- [ ] Errors use `errorz` with an appropriate code and **wrap the underlying cause** (`errors.Is`-friendly).
- [ ] Any `logger.Logger` use is **optional and nil-safe**; no stdlib `log`.
- [ ] No new **unjustified cross-package imports**.
- [ ] If a package was added, the [package map](#package-map) row exists.
- [ ] Public API uses stdlib types; constructors are `NewX`; options are `WithX`.
- [ ] Any configuration is an exported `Config` struct with **`mapstructure` snake_case tags** (non-serializable fields tagged `-`), a `DefaultConfig()`, and a `Validate()`; `WithX` is used only for optional deps / non-serializable behavior.
- [ ] Every new exported interface has a **`//go:generate mockgen` directive** writing into `mocks/<pkg>/`; `make mocks` was run and the generated files committed.

---

## Deeper reference

| Document | What it covers |
|---|---|
| [docs/DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md) | Roadmap for the observability/tracing/auth/reliability packages (ctxkit, tracer, auth, metrics, ratelimit, circuitbreaker, lifecycle) |
| [docs/TESTING.md](docs/TESTING.md) | How consumers test against SDK seams: fakes-first, the `mocks` module (gomock), miniredis, go-sqlmock |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Layering model, error-layer separation, dependency direction |
| [docs/PATTERNS.md](docs/PATTERNS.md) | Copy-paste templates: options, sentinel errors, table-driven tests, context, leader/follower |
| [docs/NEW_PACKAGE_CHECKLIST.md](docs/NEW_PACKAGE_CHECKLIST.md) | Step-by-step for adding a sub-package |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Human setup, dev loop, PR expectations |
