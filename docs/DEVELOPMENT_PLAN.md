# go-sdk Development Plan — Observability, Auth, Validation & Reliability

> **Purpose.** Canonical, agent-executable roadmap for extending `github.com/biairmal/go-sdk` with
> production-grade cross-cutting building blocks. Written so an AI agent (or human) can pick up any phase and
> implement it correctly without re-deriving context. Read [AGENTS.md](../AGENTS.md) first for the hard rules.

## Status at a glance

Phases are listed in **build order** — each builds on the ones above it.

| # | Package | What it adds | Status |
|---|---|---|---|
| 1 | `ctxkit` | Canonical request-scoped context keys + logger extractor | ✅ **Done** |
| 2 | `validator` | Struct validation via tags (wraps go-playground/validator) | ✅ **Done** |
| 3 | `tracer` | Distributed tracing (OpenTelemetry) | ✅ **Done** |
| 4 | `auth` | Token issuing + validation, monolith-first | ✅ **Done** |
| 5 | `metrics` | Request instrumentation (Prometheus) | ✅ **Done** |
| 6 | `ratelimit` | Rate limiting (in-memory + Redis) | ✅ **Done** |
| 7 | `circuitbreaker` | Outbound dependency protection | ✅ **Done** |
| 8 | `lifecycle` | Graceful shutdown | ✅ **Done** |
| 9 | `crypto` | Field-level authenticated encryption (`Encrypt`/`Decrypt`, AES-256-GCM) + deterministic `BlindIndex` (HMAC-SHA256) for exact-match lookup on encrypted fields; separate keys for each, via `Config` | ✅ **Done** |
| 10 | `queue` | Async message publishing abstraction (`Publisher.Publish`), swappable backend + `NoOp`/logging fake | ✅ **Done** |
| 11 | `queue`/`kafka` | Consumer side: `kafka.Consumer` (`kafka.Reader`-backed) + `queue.Subscriber` | ✅ **Done** |

Ordered **easiest-independent-first**. The only hard constraint: `ctxkit` (Phase 1, done) must precede
`tracer`/`auth`/`metrics`, which write their context values through it. Everything else is independent —
`validator` goes second as a low-risk warm-up that establishes the package pattern. Phases 6–8 can land in any order.

---

## 1. Context & Goals

The SDK already ships: `errorz` (structured errors), `logger` (zerolog interface), `httpkit` (handler adapter +
middleware + client), `sqlkit` (leader/follower DB), `repository` (generic CRUD), `repository/cache`, `redis`,
`config` (Viper/YAML), `serializer`, `ctxkit` (context keys). This plan adds the remaining cross-cutting building
blocks needed for reliable microservices: distributed tracing, auth, metrics, struct validation, rate limiting,
circuit breaking, and graceful shutdown.

### Design principles (building-blocks SDK, **not** a framework)

- Interface-first, swappable backends (real backend + `NoOp` each).
- No hidden globals / init magic. Everything constructed explicitly and injected.
- Opt-in — consumers import only what they need.
- Stdlib types at the edges (`context.Context`, `http.Handler`, `*http.Client`, `error`).
- **Config-first, YAML-first** — see the [Configuration convention in AGENTS.md](../AGENTS.md#configuration). Every
  configurable package exposes a `mapstructure`-tagged `Config` struct (embeddable in app config, loadable via
  `config.Load`), a `DefaultConfig()`, and a `Validate()`. `WithX` options are reserved for optional deps and
  non-serializable behavior (loggers, clients, callbacks). Required deps are positional constructor params.
- Monolith-first, microservices-later — same interfaces serve both; transition is config, not code.

### Repository conventions (MUST — from AGENTS.md / docs/PATTERNS.md)

- Constructors `NewX`; **config in a tagged `Config` struct** (not `WithX`); `WithX` only for optional deps /
  non-serializable behavior, nil/zero-safe; return the **interface** not the concrete type.
- Signatures return `error`, never `*errorz.Error` (typed-nil trap). Wrap causes + attach an `errorz` code;
  callers compare via `errors.Is` on sentinels.
- Optional `logger.Logger` only where there is real internal state; guard every call `if l == nil { return }`.
- **Every exported interface ships a mock.** Add a `//go:generate mockgen` directive above the interface writing
  into `mocks/<pkg>/` (package `mock<pkg>`), then `make mocks`. So: `tracer.Tracer`, `auth.Validator`/`Claims`/
  `Issuer`, `metrics.Recorder`, `ratelimit.Limiter`, `circuitbreaker.Breaker`, `validator.Validator`,
  `lifecycle.Closer` each get one. Also ship a hand-written `NoOp`/fake where a stand-in beats call verification.
  See [TESTING.md](TESTING.md) and the [new-package checklist](NEW_PACKAGE_CHECKLIST.md).
- I/O funcs take `context.Context` first (`noctx`). Files `lower_with_underscores.go`. Tests same-package,
  `*__test.go`, table-driven. Integration tests `*_integration_test.go`, `t.Skip` under `testing.Short()`.
- Linters: line ≤ 120, func ≤ 100 lines / 50 stmts, cyclomatic < 15, cognitive < 25. Split big constructors
  into `build…`/`configure…` helpers. Always handle type-assertion `ok`.
- **DoD per package:** `make check` green; table-driven tests; `README.md`; row in AGENTS.md package map;
  doc comments on every exported symbol.

---

## 2. Context propagation model

Four canonical request-scoped values; each has one writer and is surfaced in logs automatically via `ctxkit`.

| Field | Written by | Header (in/out) | Purpose |
|---|---|---|---|
| `request_id` | `middleware.RequestID` | `X-Request-Id` | Per-request id, unique to this hop |
| `correlation_id` | `middleware.Correlation` | `X-Correlation-Id` | Flows **across** services for one logical op |
| `trace_id` | `middleware.Tracing` | W3C `traceparent` | OpenTelemetry trace id |
| `user_id` | `middleware.Auth` | — | Authenticated subject (claims `sub`) |

Outbound `httpkit/client` requests forward `X-Correlation-Id` and `traceparent` so the chain continues downstream.

---

## Phase 1 — `ctxkit` (Structured Context Extraction) ✅ DONE

One source of truth for request-scoped context values; replaces the fragile string-key extractor in `logger`.
Dependency direction: `ctxkit → logger` (one-way); `logger` stays a leaf, unchanged. Consumers pass
`ctxkit.LoggerExtractor()` into `logger.Options.ContextExtractor`.

**Shipped files:** `ctxkit/ctxkit.go` (typed keys + `WithRequestID/RequestID`, `WithCorrelationID/CorrelationID`,
`WithTraceID/TraceID`, `WithUserID/UserID` — all empty-safe), `ctxkit/extractor.go` (`LoggerExtractor()`),
`ctxkit/ctxkit__test.go`, `ctxkit/README.md`, `httpkit/middleware/correlation.go` (`Correlation()` +
`CorrelationIDHeader`). `httpkit/middleware/requestid.go` also writes `ctxkit.WithRequestID` now.

**Remaining integration edits (apply as later phases land):** ~~tracer mw uses `ctxkit.WithTraceID`~~ done (Phase 3);
~~auth mw uses `ctxkit.WithUserID`~~ done (Phase 4); `httpkit/client` forwards correlation + traceparent on outbound requests.
**Deps:** none.

---

## Phase 2 — `validator` (Struct validation via tags) ✅ DONE

**Shipped files:** `validator/config.go` (`Config{TagName,FieldNameTag}` + `DefaultConfig()` + `Validate()`),
`validator/validator.go` (`Validator` interface, `FieldLevel` alias, `New(cfg, ...Option)`, mockgen directive),
`validator/playground.go` (go-playground/validator/v10 backend + `errorz` translation), `validator/options.go`
(`WithCustomValidation`), `validator/validator__test.go`, `validator/README.md`. Mock generated into
`mocks/validator/mock_validator.go`.

Thin wrapper around `github.com/go-playground/validator/v10` for validating structs by their `validate` tags,
returning SDK-native `errorz` errors so validation failures flow through `httpkit` as clean 400s with per-field
detail. Interface-first so the backend stays swappable; no framework coupling. Independent of `ctxkit` — only
depends on `errorz` — so it's the easiest first build and establishes the package pattern (interface → backend →
`errorz` errors → tests → README → `make check`).

**Files:**

| File | Contents |
|---|---|
| `validator/config.go` | `Config` (mapstructure-tagged) + `DefaultConfig()` + `Validate()` |
| `validator/validator.go` | `Validator` interface + `New(cfg, ...Option)` constructor |
| `validator/playground.go` | go-playground/validator backend + error translation to `errorz` |
| `validator/options.go` | `WithCustomValidation` (func — non-serializable, so an option) |
| `validator/validator__test.go` | table-driven: valid/invalid structs, custom rules, error shape |
| `validator/README.md` | usage + error format |

### Config & interface

```go
// Config holds the YAML-able validator settings.
type Config struct {
    // TagName is the struct tag key rules are read from. Default "validate".
    TagName string `mapstructure:"tag_name"`
    // FieldNameTag, when set (e.g. "json"), makes error field names match that tag
    // instead of the Go field name. Default "" (Go field names).
    FieldNameTag string `mapstructure:"field_name_tag"`
}

func DefaultConfig() Config { return Config{TagName: "validate"} }

// Validator validates values against struct tags.
type Validator interface {
    // ValidateStruct validates s using its struct tags. Returns nil when valid, or an error
    // (errorz code CodeBadRequest) whose Meta carries per-field messages under "fields".
    ValidateStruct(s any) error
    // ValidateVar validates a single value against a tag expression, e.g. ValidateVar(email, "required,email").
    ValidateVar(field any, tag string) error
    // Register adds a custom validation function under a tag name (also settable via option).
    Register(tag string, fn func(fl FieldLevel) bool) error
}

// FieldLevel is a thin alias over the backend's field accessor so callers writing
// custom validations don't import go-playground directly.
type FieldLevel = validator.FieldLevel

// New builds a Validator. Config carries the YAML-able knobs; the option is for the
// non-serializable custom-rule funcs.
func New(cfg Config, opts ...Option) Validator
```

### Options (non-serializable only)

| Option | Description |
|--------|-------------|
| `WithCustomValidation(tag string, fn func(FieldLevel) bool)` | Register a custom rule (a func — can't live in YAML). |

### Error translation
On failure the backend receives `validator.ValidationErrors`; translate each field error into a human-readable
message and return `errorz.BadRequest().WithMessage("validation failed").WithMeta("fields", map[string]string{...})`.
Non-validation errors (e.g. passing a non-struct to `ValidateStruct`) are wrapped with `errorz.Wrap(err).WithCode(
errorz.CodeInternal)`. `httpkit/handler/status.go` already maps `CodeBadRequest` → 400, and the response envelope
surfaces `Meta`, so field errors reach the client with no extra wiring.

### Usage sketch
```go
v := validator.New(validator.Config{TagName: "validate", FieldNameTag: "json"})

type CreateUser struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"gte=0,lte=130"`
}

func handler(r *http.Request) (any, error) {
    var body CreateUser
    if err := serializer.ParseJSON(r.Body, &body); err != nil { return nil, err }
    if err := v.ValidateStruct(body); err != nil { return nil, err } // → 400 with per-field messages
    // ... happy path
}
```

**Deps:** `github.com/go-playground/validator/v10`. Uses `errorz` (foundational leaf) for error output.

---

## Phase 3 — `tracer` (Distributed Tracing) ✅ DONE

**Shipped files:** `tracer/tracer.go` (`Tracer`, `Span`, `SpanContext`, `SpanKind` consts, `SpanOption` +
`WithSpanKind`/`WithAttributes`, mockgen directive), `tracer/config.go` (`Config` + `DefaultConfig()` +
`Validate()`), `tracer/options.go` (`WithLogger`), `tracer/otel.go` (`NewOTel` — OTLP/gRPC exporter + SDK
`TracerProvider`), `tracer/noop.go` (`NewNoOp`), `tracer/tracer__test.go`, `tracer/otel_integration_test.go`
(short-guarded), `tracer/README.md`. Mock generated into `mocks/tracer/mock_tracer.go`. Also added
`httpkit/middleware/tracing.go` (`Tracing(t)` + `tracing_test.go`).

Interface-first tracing, OTel OTLP-gRPC backend + `NoOp`. Server middleware starts a span/request, extracts &
propagates W3C trace context, publishes `trace_id` via `ctxkit`.

**Files:** `tracer/config.go` (`Config{service_name, service_version, endpoint, insecure, sample_rate}`
mapstructure-tagged + `DefaultConfig()` + `Validate()`), `tracer/tracer.go` (`Tracer`, `Span`,
`SpanContext{TraceID,SpanID,IsSampled}`, `SpanOption`, span-kind consts), `tracer/otel.go`
(`NewOTel(cfg Config, ...Option)` — option: optional `logger.Logger`), `tracer/noop.go`, `tracer/tracer__test.go`,
`tracer/otel_integration_test.go` (short-guarded), `tracer/README.md`, `httpkit/middleware/tracing.go` (`Tracing(t)`).

```go
type Config struct {
    ServiceName    string  `mapstructure:"service_name"`
    ServiceVersion string  `mapstructure:"service_version"`
    Endpoint       string  `mapstructure:"endpoint"`     // OTLP gRPC, e.g. "localhost:4317"
    Insecure       bool    `mapstructure:"insecure"`
    SampleRate     float64 `mapstructure:"sample_rate"`  // 0..1, default 1.0
}
func NewOTel(cfg Config, opts ...Option) (Tracer, error) // Option injects an optional logger.Logger
func NewNoOp() Tracer
```

`Tracer.Start(ctx, name, opts) (context.Context, Span)`, `Shutdown(ctx) error`. Middleware: extract inbound
`traceparent`/`tracestate` via `otel/propagation.TraceContext`, start server span `"METHOD /path"`,
`ctxkit.WithTraceID`, reuse `responseCapture` (from `logging.go`) to set span error on 5xx, `defer span.End()`.
Chain position: after RequestID/Correlation, before Logging. SQL: document consumer-side `otelsql` driver
registration (no `sqlkit` change).

**Logs → Grafana (not part of this package).** Traces push to Tempo via OTLP; logs are a separate signal. Keep the
app logging structured JSON to stdout and let an agent (Grafana Alloy / Promtail / OTel Collector `filelog`
receiver) ship to Loki. No in-app log exporter needed. Trace↔log correlation already works because
`ctxkit.LoggerExtractor()` puts `trace_id` on every log line. Use `logger.FormatJSON` in prod.

**Deps:** `go.opentelemetry.io/otel` + `/trace` + `/sdk` + `/exporters/otlp/otlptrace(+grpc)` + `/propagation`
(v1.33.0), `google.golang.org/grpc` v1.70.0.

---

## Phase 4 — `auth` (Token Issuing + Validation, monolith-first) ✅ DONE

**Shipped files:** `auth/validator.go` (`Validator` + `ValidatorFunc` + mockgen directive for `Validator,Claims,Issuer`),
`auth/claims.go` (`Claims`, `mapClaims`, `NewClaims`, `getPath`, `ContextWithClaims`→`ctxkit.WithUserID`,
`ClaimsFromContext`, `SubjectFromContext`), `auth/errors.go` (token sentinels wrapping `errorz.Unauthorized`),
`auth/options.go` (shared `Option`: `WithExpectedIssuer`/`WithExpectedAudience`/`WithLeeway`/`WithPublicKey`/
`WithJWKSURL`/`WithHTTPClient`), `auth/jwt.go` (shared local-JWT core; alg-pinned parser — **added beyond the
original file list** to keep hs256/rs256 under the lint caps), `auth/hs256.go` (`NewHS256`), `auth/rs256.go`
(`NewRS256` + PEM parsers), `auth/jwks.go` (`jwksCache`: RWMutex, TTL + unknown-kid-cooldown refresh, stale-on-failure),
`auth/issuer.go` (`Issuer`, `NewHS256Issuer`/`NewRS256Issuer`, issue-time + construction options),
`auth/remote.go` (`NewRemote`, `TokenForward`, `ClaimsMapping`, `RemoteConfig`; 401/403→401, else→502),
`auth/policy.go` (`Policy`, `Rule`, `IsProtected`, `matchPattern`), `auth/cache.go` (`NewCached` TTL cache),
`auth/config.go` (`Config` + `DefaultConfig`/`Validate`/`Policy`, `FromConfig`, `IssuerFromConfig`), tests
(`auth/*__test.go`), `auth/README.md`, `httpkit/middleware/auth.go` (`Auth(v, WithPolicy(pol))` + `auth_test.go`).
Mock generated into `mocks/auth/mock_auth.go`; `./auth/...` added to `MOCK_PKGS`. **Naming note:** validator-side
expectations are `WithExpectedIssuer`/`WithExpectedAudience` and the claims injector is exported as
`ContextWithClaims` (the spec's `WithIssuer`/`WithAudience`/`injectClaims` collided in-package or needed export).

Reusable auth that works in a **monolith today** (in-process issue + validate) and splits into services **later**
by config only. `Validator` interface is the seam. HTTP-agnostic core; HTTP adapter in `httpkit/middleware/auth.go`.

**Modes:** `local` (in-process JWT), `remote` (call identity service), `func` (`ValidatorFunc` adapter).
Local ships HS256 + RS256 (+ optional JWKS) plus a token `Issuer`. Remote validation is **fully mappable**
(dotted-path field mapping over arbitrary JSON). Route protection is config-driven; glob matcher supports both
`*` (single segment) and `**` (recursive). Optional TTL validation cache.

### Monolith → microservices transition

| Stage | `auth.mode` | Validator | Network hop |
|---|---|---|---|
| Monolith | `local` | `NewHS256` / `NewRS256(WithPublicKey)` | No |
| Monolith, custom logic | `func` | `ValidatorFunc` over identity verify | No |
| Split | `remote` | `NewRemote(...)` | Yes |
| Split, key-based | `local` + `jwks_url` | `NewRS256(WithJWKSURL(...))` | Cached key fetch |

Smoothest path: RS256 + JWKS — static public key in monolith, point `jwks_url` at identity on split; constructor
never changes.

**Files:** `auth/validator.go` (`Validator` + `ValidatorFunc`), `auth/issuer.go` (`Issuer`, `NewHS256Issuer`,
`NewRS256Issuer`, `WithTTL/WithAudience/WithRoles/WithIssuedAt`), `auth/claims.go` (`Claims` + `mapClaims`,
`ClaimsFromContext`, `SubjectFromContext`, `injectClaims`→`ctxkit.WithUserID`, `getPath`), `auth/remote.go`
(`NewRemote`, `TokenForward`, `ClaimsMapping`), `auth/hs256.go`, `auth/rs256.go`, `auth/jwks.go` (`jwksCache`,
RWMutex, TTL/kid refresh), `auth/policy.go` (`Policy`, `Rule`, `IsProtected`, `matchPattern`), `auth/cache.go`
(TTL cache wrapping any `Validator`), `auth/errors.go` (token sentinels wrapping `errorz.Unauthorized()`),
`auth/config.go` (`mapstructure` `Config`, `FromConfig`, `IssuerFromConfig`), tests, `auth/README.md`,
`httpkit/middleware/auth.go` (`Auth(v, WithPolicy(pol))`).

`Claims`: `Subject()`, `Roles()`, `Get(key)(any,bool)`, `Raw()`. `Issue` sets `sub/iss/aud/iat/exp` + caller
claims, signs with golang-jwt. Refresh tokens out of scope for v1. `matchPattern`: split by `/`, `*` = one
segment, `**` = zero+ segments, first matching rule wins → `Public`, else `DefaultProtected`. Token errors map
through existing `StatusCodeFromError`. Middleware: skip when policy says public, else bearer token →
`Unauthorized()` 401 on missing, `Validate`, `injectClaims` on success.

**Config YAML:** `auth.mode`, `default_protected`, `rules[]`, `local{algorithm,hs256_secret,rs256_public_key_path,
jwks_url,issuer,audience}`, `issuer{algorithm,hs256_secret,rs256_private_key_path,default_ttl,issuer,audience}`,
`remote{url,method,forward{in,name,prefix},mapping{claims_path,subject_field,roles_field,active_field},timeout}`,
`cache_ttl`. **Transition = flip `mode` to `remote` (or set `local.jwks_url`); no Go changes; `Issuer` stays with identity.**

**Deps:** `github.com/golang-jwt/jwt/v5` v5.2.2. Remote/JWKS/matcher use stdlib.

---

## Phase 5 — `metrics` (Prometheus) ✅ DONE

**Shipped files:** `metrics/config.go` (`Config{Namespace,HTTPBuckets}` mapstructure-tagged + `DefaultConfig()` +
`Validate()` — namespace format + positive-bucket checks), `metrics/metrics.go` (`Recorder{CounterInc,
HistogramObserve,GaugeAdd}`, `Labels`, `HTTPRequestsTotal`/`HTTPRequestDuration`/`HTTPRequestsInFlight` name
consts, mockgen directive), `metrics/prometheus.go` (`NewPrometheus(cfg, ...Option)` — dynamic per-name metric
registration cached in maps guarded by `sync.RWMutex`; standard HTTP metrics pre-registered eagerly; registration
and label-set-mismatch failures logged via the optional logger, never returned — `Recorder` methods have no error
return), `metrics/noop.go` (`NewNoOp`), `metrics/options.go` (`WithRegisterer`, `WithLogger`),
`metrics/metrics__test.go`, `metrics/README.md`. Mock generated into `mocks/metrics/mock_metrics.go`;
`./metrics/...` added to `MOCK_PKGS`. Also added `httpkit/middleware/metrics.go` (`Metrics(rec, *MetricsOptions{
PathNormalizer})` + `metrics_test.go`).

`Recorder` abstraction (Prometheus backend + `NoOp`) so `httpkit/middleware` stays Prometheus-free.

**Files:** `metrics/config.go` (`Config{namespace, http_buckets}` mapstructure-tagged + `DefaultConfig()`),
`metrics/metrics.go` (`Recorder{CounterInc,HistogramObserve,GaugeAdd}`, `Labels`), `metrics/noop.go`,
`metrics/prometheus.go` (`NewPrometheus(cfg Config, ...Option)` — option injects `prometheus.Registerer`),
`metrics/metrics__test.go`, `metrics/README.md`, `httpkit/middleware/metrics.go`
(`Metrics(rec, *MetricsOptions{PathNormalizer})` — `PathNormalizer` is a func, so it stays an option struct).

```go
type Config struct {
    Namespace   string    `mapstructure:"namespace"`
    HTTPBuckets []float64 `mapstructure:"http_buckets"` // default prometheus.DefBuckets
}
func NewPrometheus(cfg Config, opts ...Option) (Recorder, error) // Option: WithRegisterer(prometheus.Registerer)
```

Pre-registered: `<ns>_http_requests_total` (counter), `<ns>_http_request_duration_seconds` (histogram),
`<ns>_http_requests_in_flight` (gauge), labels `method,path,status_code`. Middleware: in-flight +1 on entry,
defer -1 + counter + duration; reuse `responseCapture`; place outermost. `path` low-cardinality via `PathNormalizer`.
Tests use isolated `prometheus.NewRegistry()`. **Deps:** `github.com/prometheus/client_golang` v1.21.1.

---

## Phase 6 — `ratelimit` ✅ DONE

**Shipped files:** `ratelimit/ratelimit.go` (`Limiter{Allow(ctx,key)(Result,error)}`, `Result{Allowed,Limit,
Remaining,RetryAfter}`, mockgen directive), `ratelimit/config.go` (`Config{Backend,Rate,Burst,Window,MaxKeys}`
mapstructure-tagged + `DefaultConfig()` + `Validate()` + `FromConfig(cfg, redisClient)`), `ratelimit/memory.go`
(`NewInMemory` — per-key `golang.org/x/time/rate.Limiter`, opportunistic idle eviction every `sweepEvery` calls
and when `MaxKeys` is reached), `ratelimit/redis.go` (`NewRedis` — Lua sliding-window log over a Redis sorted
set, atomic trim+count+admit in one `Eval` round trip), `ratelimit/config__test.go`, `ratelimit/memory__test.go`,
`ratelimit/redis__test.go` (fake `redis.Client`), `ratelimit/redis_integration_test.go` (`//go:build integration`,
mirrors `redis/pipeline_integration_test.go`'s convention), `ratelimit/README.md`. Mock generated into
`mocks/ratelimit/mock_ratelimit.go`; `./ratelimit/...` added to `MOCK_PKGS`. Also added
`httpkit/middleware/ratelimit.go` (`RateLimit(l, keyFn, opts)`, `KeyByIP/KeyByUser/KeyByHeader`,
`WithFailClosed`) + `ratelimit_test.go`.

**Deviation from the original spec:** the redis backend needed atomicity beyond a single command (trim + count +
conditionally admit), and `redis.Client` had no `Eval`/sorted-set support. Extended `redis.Client` with
`Eval(ctx, script, keys, args...) (interface{}, error)` (+ mock) rather than downgrading to a fixed-window
counter, so the redis backend is a true sliding-window log, not an approximation.

`Limiter` interface, two backends: in-memory token bucket (monolith) + Redis distributed (scale-out).

```go
type Config struct {
    Backend string        `mapstructure:"backend"`  // "memory" | "redis"
    Rate    float64       `mapstructure:"rate"`     // permits/sec
    Burst   int           `mapstructure:"burst"`
    Window  time.Duration `mapstructure:"window"`
    MaxKeys int           `mapstructure:"max_keys"` // in-memory eviction cap
}
```

Middleware: allow → `X-RateLimit-Limit/-Remaining`; deny → 429 (`errorz.TooManyRequests()`) + `Retry-After`;
backend error → fail-open by default. `KeyByUser` reads `ctxkit.UserID` (place after Auth).
**Deps:** `golang.org/x/time` v0.9.0; Redis backend reuses existing `redis` pkg.

---

## Phase 7 — `circuitbreaker` ✅ DONE

**Shipped files:** `circuitbreaker/config.go` (`Config{FailureThreshold,FailureRatio,OpenTimeout,HalfOpenMaxCalls}`
mapstructure-tagged + `DefaultConfig()` + `Validate()` + `withDefaults`), `circuitbreaker/breaker.go`
(`State` enum + `String()`, `ErrOpen` sentinel wrapping `errorz.ErrServiceUnavailable` + `wrapOpen()`, `Breaker`
interface, `Do[T]` generic free function, `breaker` struct, `NewBreaker(cfg, ...Option)`, mockgen directive),
`circuitbreaker/state.go` (state-machine internals: `Execute`/`State` methods, `before`/`after` admission +
outcome recording with a generation counter so a stale in-flight call's result can't corrupt a newer state,
`onClosedResultLocked`/`shouldTripLocked`, `onHalfOpenResultLocked`, `setStateLocked`, `notify` — callback fires
outside the lock so it may safely re-enter the breaker), `circuitbreaker/options.go` (`WithOnStateChange`,
`WithIsSuccessful`, `WithClock` — the last is beyond the original file list, added for deterministic
`OpenTimeout`-expiry tests per the "injectable clock" requirement below), `circuitbreaker/config__test.go`,
`circuitbreaker/breaker__test.go` (consecutive/ratio trip, half-open close/reopen/concurrency-limit/stale-result,
panic safety, callback reentrancy, custom success classifier, `Do[T]`, a concurrency stress test), `circuitbreaker/README.md`.
Mock generated into `mocks/circuitbreaker/mock_circuitbreaker.go`; `./circuitbreaker/...` added to `MOCK_PKGS`.

**Deviations from the original spec:** (1) added `WithClock` (not in the original Options table, but the phase
prose calls for an "injectable clock"). (2) `NewBreaker(cfg, ...Option) Breaker` does **not** call
`cfg.Validate()` itself — it only fills zero-valued fields via `withDefaults`, matching `ratelimit.NewInMemory`'s
precedent (a pure, non-I/O constructor); callers that load `Config` from YAML should call `Validate()` themselves
before constructing. (3) `FailureRatio` reuses `FailureThreshold` as its minimum-sample gate rather than adding
an undocumented `MinRequests` field, so the ratio rule never trips on a handful of unlucky calls.

Own lean closed→open→half-open state machine, no third-party dep. Protects outbound dependency calls.

**Files:** `circuitbreaker/config.go` (`Config{failure_threshold, failure_ratio, open_timeout, half_open_max_calls}`
mapstructure-tagged + `DefaultConfig()`), `circuitbreaker/breaker.go` (`Breaker{Execute,State}`, `State` enum,
`NewBreaker(cfg, ...Option)`, `Do[T]`, `ErrOpen`), `circuitbreaker/options.go` (`WithOnStateChange`,
`WithIsSuccessful` — funcs, so options), `circuitbreaker/breaker__test.go`, `circuitbreaker/README.md`.

```go
type Config struct {
    FailureThreshold int           `mapstructure:"failure_threshold"`
    FailureRatio     float64       `mapstructure:"failure_ratio"`
    OpenTimeout      time.Duration `mapstructure:"open_timeout"`      // open→half-open probe delay
    HalfOpenMaxCalls int           `mapstructure:"half_open_max_calls"`
}
func NewBreaker(cfg Config, opts ...Option) Breaker // Options: WithOnStateChange(fn), WithIsSuccessful(fn)
```

Thread-safe (`sync.Mutex`), injectable clock. `ErrOpen` wraps `errorz.ServiceUnavailable()` (`CodeServiceUnavailable`)
→ clean 503 at the edge. Trip on threshold/ratio → Open; after OpenTimeout → HalfOpen probes; success closes,
failure re-opens. **Deps:** none.

---

## Phase 8 — `lifecycle` (Graceful Shutdown) ✅ DONE

**Shipped files:** `lifecycle/config.go` (`Config{DrainDelay,ShutdownTimeout,CloserTimeout}` mapstructure-tagged +
`DefaultConfig()` — 5s/15s/15s + `Validate()` + unexported `withDefaults`), `lifecycle/closer.go` (`Closer`
interface + mockgen directive, `CloserFunc`, `CloserFromTracer`/`CloserFromDB`/`CloserFromRedis` — all three
**structural** (accept an inline `interface{ Shutdown(ctx) error }` / `interface{ Close() error }` rather than the
real `tracer.Tracer`/`*sqlkit.DB`/`redis.Client` types), `lifecycle/options.go` (`WithReadiness`, `WithSignals`,
`WithLogger`, `WithCloser`, `WithShutdownFunc`), `lifecycle/lifecycle.go` (`Run(ctx, srv, cfg, opts...) error`,
`ErrForcedShutdown` sentinel, unexported `runner`/`namedCloser`/`(*runner).run` core decoupled from the real OS
signal channel, `waitOrForced`, `runPhase`, `(*runner).forceExit`/`runClosers` + logging helpers),
`lifecycle/lifecycle__test.go` (table-driven + scenario tests: happy path, context-triggered shutdown, hook-error
joining, forced exit during the drain delay and during the closer phase via synchronized fake signal channels,
closer adapters, `Config.Validate`), `lifecycle/README.md`. Mock generated into
`mocks/lifecycle/mock_lifecycle.go` (`./lib/...` wildcard in `MOCK_PKGS` picked it up with no Makefile change).

**Deviations from the original spec:** (1) `CloserFromTracer`/`CloserFromDB`/`CloserFromRedis` take a small
inline structural interface instead of the real `tracer.Tracer`/`*sqlkit.DB`/`redis.Client` types, so `lifecycle`
imports none of its siblings — keeping it genuinely dependency-free per the "keep packages independent" rule,
at the cost of not catching a typed-nil concrete pointer passed through the adapter (documented in the README's
Limitations). (2) `Run`'s core logic lives in an unexported `(*runner).run(ctx, srv, cfg, sigCh)` that takes the
signal channel as a parameter, with the exported `Run` just wiring up `signal.Notify` — this is what makes the
forced-exit-on-second-signal behavior testable without sending real OS signals (push fake `os.Signal` values into
a test-owned channel instead), the same testability motivation that added `circuitbreaker.WithClock`. (3) Added a
`srv == nil` guard returning `errorz.BadRequest()` (not in the original file list) since `srv` is an unconditionally
required positional dependency, matching `ratelimit.FromConfig`'s precedent for required-dep validation.

Ordered, deadline-bounded shutdown across HTTP + resources (DB, Redis, tracer flush), with an LB-safe drain
delay, per-phase budgets, and a forced-exit escape hatch. See "Production-readiness notes" below for the
reasoning behind each deviation from a naive "trap signal → Shutdown → close everything" implementation.

**Files:** `lifecycle/config.go` (`Config{drain_delay, shutdown_timeout, closer_timeout}` mapstructure-tagged +
`DefaultConfig()` — 5s / 15s / 15s), `lifecycle/lifecycle.go` (`Run(ctx, *http.Server, cfg Config, opts ...Option)
error`; options are non-serializable: `WithReadiness(*atomic.Bool)`, `WithSignals(...os.Signal)`, `WithLogger`,
`WithCloser(name string, c Closer)`, `WithShutdownFunc`), `lifecycle/closer.go` (`Closer{Close(ctx)error}`,
adapters `CloserFromTracer/CloserFromDB/CloserFromRedis`), `lifecycle/lifecycle__test.go`, `lifecycle/README.md`.
(`Config` only holds the three durations; readiness flag, signal set, and closers are live objects/funcs, so they
stay options.)

**Behavior:**

1. `signal.Notify` on a buffered channel (`WithSignals`, default `SIGINT, SIGTERM`).
2. On the **first** signal: flip readiness (so `httpkit.Readiness` starts returning 503) → sleep `DrainDelay` →
   `srv.Shutdown(ctx)` bounded by `ShutdownTimeout` → run closers **in registration order** (see ordering note
   below), each bounded by a slice of `CloserTimeout`, logging each closer's name/duration/error via the optional
   logger → `errors.Join` the shutdown error and every closer error and return.
3. On a **second** signal received at any point after the first (drain delay, `srv.Shutdown`, or closer loop):
   abandon the remaining budget immediately, log at warn level, and return/exit without waiting out the
   configured timeouts. This is the operator's "stop being polite" escape hatch when a dependency hangs.

**Deps:** none (stdlib).

### Production-readiness notes (why the spec looks like this)

- **Drain delay (`DrainDelay`, default 5s) between flipping readiness and calling `srv.Shutdown`.** Flipping
  readiness and shutting down back-to-back races the load balancer / k8s endpoint controller: they need a beat to
  observe the 503 and stop routing before the listener actually closes, or a slice of in-flight requests get
  connection-refused instead of served. The delay is a plain `time.Sleep`, interruptible by a second signal (see
  above) so a manual double Ctrl-C still exits fast in local dev.
- **Closer ordering is registration order (not LIFO), and callers are expected to register least-recoverable
  first.** The worked example registers `WithCloser("tracer", ...)`, `WithCloser("redis", ...)`,
  `WithCloser("db", ...)` — the intent is tracer flush first (captures spans for requests that just finished
  draining, while they're still fresh), then redis, then db last (the most foundational dependency, kept alive
  longest in case another closer's `Close` needs to write a final record). Document this explicitly in
  `lifecycle/README.md` so consumers don't assume LIFO and register in the wrong order.
- **Split timeouts (`ShutdownTimeout`, `CloserTimeout`) instead of one shared `timeout`.** A single budget lets a
  slow HTTP drain (lots of in-flight requests) starve the closer phase down to near-zero, silently skipping the
  DB/Redis/tracer cleanup under load — exactly when it matters most. Splitting gives closers a guaranteed floor
  independent of how long the drain took. `CloserTimeout` is the budget for the *whole* closer phase (closers run
  sequentially in registration order, not concurrently, so ordering guarantees hold); log per-closer duration so
  an incident review can see which one ate the budget.
- **Forced exit on a second signal.** Without it, an operator who sends SIGTERM during an incident and then gets
  impatient (or a dependency's `Close` hangs) has no way to escalate short of `SIGKILL`, which skips logging/flush
  entirely. Trapping the second signal lets `lifecycle` log what it was doing when it bailed.

Total worst-case wall time with defaults: `DrainDelay(5s) + ShutdownTimeout(15s) + CloserTimeout(15s)` ≈ 35s —
tune per-service; a service with a slow LB propagation or a slow DB pool drain should raise the relevant knob
independently rather than one global number.

---

## Phase 9 — `crypto` (Field-level PII Encryption) ✅ DONE

**Shipped files:** `crypto/config.go` (`Config{EncryptionKey,BlindIndexKey}` + `DefaultConfig()` — deliberately
invalid, since both keys are required secrets — + `Validate()`, enforcing key separation), `crypto/crypto.go`
(`Encryptor`, `New(cfg) (*Encryptor, error)`, `Encrypt`/`Decrypt`/`BlindIndex` methods, `ErrDecryptionFailed`
sentinel wrapping `errorz.ErrInternal`), `crypto/config__test.go`, `crypto/crypto__test.go` (round-trip, nonce
non-determinism, wrong-key rejection, tamper detection, blind-index determinism/collision behavior),
`crypto/README.md`. No mock — no exported interface (see deviation below).

Requested by `guest-management-be` B8 (`guests` PII encryption on `email`/`phone`) — see
[guest-management-be B8](../../guest-management-be/docs/DEVELOPMENT_PLAN.md), which sketched a rough shape
during its own design pass and explicitly deferred the SDK package itself to here.

**Deviation from the B8 sketch:** B8 sketched free functions taking a raw `key []byte` on every call
(`Encrypt(key, plaintext []byte) (string, error)`). Building it, this SDK's `New(cfg, ...) X` constructor
convention fits better: `New(cfg Config) (*Encryptor, error)` validates and decodes both keys **once**, and
the returned `*Encryptor`'s methods take only the plaintext/ciphertext/value — no way to accidentally pass the
blind-index key where the encryption key belongs, or vice versa. `guest-management-be`'s `cryptoPIIEncryptor`
adapter just holds one `*Encryptor` instead of two `[]byte` keys.

One implementation only — **AES-256-GCM** for `Encrypt`/`Decrypt`, **HMAC-SHA256** for `BlindIndex` — no
swappable backend. Unlike `tracer`/`auth`/`metrics`/`ratelimit`/`circuitbreaker`, this package skips the
interface + `NoOp` + mock pattern: there's nothing to swap, and a "no-op encryptor" is a footgun, not a fake
worth shipping. `*Encryptor` is a concrete type; a consumer that needs a test double fakes it at its own
interface boundary, same as `guest-management-be` already does with `PIIEncryptor`.

**Files:**

| File | Contents |
|---|---|
| `crypto/config.go` | `Config{EncryptionKey, BlindIndexKey string}` (mapstructure, base64) + `DefaultConfig()` + `Validate()` |
| `crypto/crypto.go` | `Encryptor` type, `New(cfg Config) (*Encryptor, error)`, `Encrypt`/`Decrypt`/`BlindIndex` methods, `ErrDecryptionFailed` sentinel |
| `crypto/crypto__test.go` | table-driven: round-trip, tamper detection (flipped byte fails auth), wrong-key rejection, blind-index determinism + collision-resistance, nonce uniqueness across calls |
| `crypto/README.md` | usage + key-generation snippet + security notes (never log plaintext/keys, key separation) |

### Config & interface

```go
// Config holds the key material for field encryption and blind indexing. Both fields are
// standard-base64-encoded; source real values from a secret store, never commit them.
type Config struct {
    // EncryptionKey decodes to exactly 32 bytes — the AES-256-GCM key.
    EncryptionKey string `mapstructure:"encryption_key"`
    // BlindIndexKey decodes to at least 16 bytes — the HMAC-SHA256 key. Must differ from
    // EncryptionKey: an encryption key and a MAC key serve different purposes and must not be reused.
    BlindIndexKey string `mapstructure:"blind_index_key"`
}

func DefaultConfig() Config { return Config{} } // no safe default secret; both fields are required
func (c Config) Validate() error // decodes both, checks lengths, and that they differ

// Encryptor performs authenticated field encryption and deterministic blind indexing over one
// validated key pair.
type Encryptor struct { /* unexported decoded keys */ }

// New validates cfg (calling Validate()) and builds an Encryptor.
func New(cfg Config) (*Encryptor, error)

// Encrypt returns a base64-encoded, randomly-nonced AES-256-GCM ciphertext of plaintext.
// Encrypting the same plaintext twice yields different output — semantic security by design.
func (e *Encryptor) Encrypt(plaintext []byte) (string, error)

// Decrypt reverses Encrypt. Returns ErrDecryptionFailed (wraps errorz.Internal) on a bad key,
// truncated/corrupted ciphertext, or a failed auth tag (tamper detection).
func (e *Encryptor) Decrypt(ciphertext string) ([]byte, error)

// BlindIndex returns a deterministic, hex-encoded HMAC-SHA256 of value, for exact-match lookups
// against an encrypted column. Same value + same key always produce the same output. Callers
// normalize value (case-folding, trimming, digits-only phone, ...) before calling — that's the
// consuming feature's business rule, not this package's concern.
func (e *Encryptor) BlindIndex(value string) string
```

### Implementation notes

- **Ciphertext format:** `base64( nonce(12 bytes) || AES-GCM-seal(plaintext) )`. The nonce is
  `crypto/rand`-generated per call and travels with the ciphertext — standard AES-GCM practice, no separate
  nonce column needed.
- **Key separation is enforced, not just documented:** `Validate()` rejects a `Config` where
  `EncryptionKey == BlindIndexKey`.
- **Errors:** `Config.Validate()` failures → `errorz.BadRequest()` (bad startup config, same precedent as
  `ratelimit`/`tracer`). `Decrypt` failures → sentinel `ErrDecryptionFailed` wrapping `errorz.Internal()` — a
  bad key, corrupted data, or a forged ciphertext is a data-integrity fault, not a client input problem.
- **No logger option.** No internal state transitions worth tracing — encrypt/decrypt/hash are pure,
  single-shot operations and failures already surface as errors (matches `serializer`'s reasoning).

**Non-goals** (deferred — nothing here is required by B8's acceptance criteria):
- **Key rotation / multi-key / key versioning.** One active key pair per `Encryptor`. Rotating means
  re-encrypting existing rows with a new `Encryptor` at the app layer; the SDK doesn't need an "old vs new
  key" concept until a real rotation requirement exists.
- **KMS/HSM/secret-manager integration.** `Config` takes raw (base64) key bytes; sourcing them securely (env
  var, Vault, AWS Secrets Manager, ...) is the consuming app's job, same as every other SDK package's secrets
  (`auth.hs256_secret`, DB passwords).
- **Pluggable cipher/algorithm choice.** AES-256-GCM and HMAC-SHA256 are hardcoded, not config options — a
  config knob for an algorithm nobody's asked to change is exactly the unused flexibility this SDK's design
  principles warn against. A different algorithm, if ever needed, is a new function, not a flag.
- **Streaming / large-payload encryption.** Sized for short PII field values (names, emails, phone numbers),
  not files or blobs — loads the whole plaintext/ciphertext into memory, no chunking.

**Deps:** none — `crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/rand`, `crypto/sha256`,
`encoding/base64`, `encoding/hex` (all stdlib).

---

## Phase 10 — `queue` (Async Message Publishing Abstraction) ✅ DONE

Requested by `guest-management-be` B8 (`guests.InvitationPublisher`) — see
[guest-management-be B8](../../guest-management-be/docs/DEVELOPMENT_PLAN.md). `guests.SendInvitation` needs to
publish an `InvitationMessage` for later async delivery, but no queue/messaging package exists in this SDK yet, so
`guest-management-be` declared a local `InvitationPublisher` interface and wired only a `LoggingInvitationPublisher`
(logs the message, sends nothing) as a stopgap. Per this SDK's design principles (interface-first, swappable
backends, monolith-first), that publish concern belongs here, the same way `crypto` absorbed B8's PII-encryption
sketch in Phase 9 — `guest-management-be` should depend on an interface + config, never a specific broker client.

**Broker confirmed: Kafka.** Two packages, not one, mirroring how `ratelimit` depends on the existing `redis`
package rather than embedding a Redis client inline:

- **`kafka`** — connection + auth only. Wraps `github.com/segmentio/kafka-go`'s `kafka.Writer`. No messaging
  semantics, no topic abstraction beyond passing the name through.
- **`queue`** — the `Publisher` interface + swappable backends (`NoOp`, `Logging`, `Kafka`). This is what
  `guest-management-be` imports; it never sees a Kafka type directly, so a future non-Kafka backend (or a test
  double) is a config change, not a `guests` code change.

**Producer only, this phase.** `guest-management-be`'s only current need is fire-and-forget publish.
`segmentio/kafka-go` models producer (`kafka.Writer`) and consumer (`kafka.Reader`) as separate types, so a
consumer later is a new `kafka.Reader`-backed type in this same package plus a `queue.Subscriber` interface next
to `Publisher` — not a new package, since it reuses the same `AuthConfig`/TLS-building code this phase ships
(`buildSASLMechanism` and the `tls.Config` builder are called once per connection either way). Building
`Subscriber` speculatively now, with no consumer use case yet, is exactly the unused-flexibility this SDK's
design principles warn against (see Phase 9's non-goals for the same reasoning applied to key rotation).

### `kafka` — connection + auth

| File | Contents |
|---|---|
| `kafka/config.go` | `Config{Brokers, Auth}` (mapstructure) + `DefaultConfig()` + `Validate()` |
| `kafka/auth.go` | `AuthConfig` + `buildSASLMechanism` (mechanism → `sasl.Mechanism`) + TLS config building |
| `kafka/client.go` | `Client`, `New(cfg Config, opts ...Option) (*Client, error)`, `Produce`, `Close` |
| `kafka/kafka__test.go` | table-driven: config validation, mechanism selection |
| `kafka/kafka_integration_test.go` | `//go:build integration`, `t.Skip` under `testing.Short()` — real broker round trip |
| `kafka/README.md` | usage + auth setup per mechanism |

```go
// Config holds broker connection settings, mapstructure-tagged for config.Load.
type Config struct {
    Brokers []string   `mapstructure:"brokers"`
    Auth    AuthConfig `mapstructure:"auth"`
}

// AuthConfig selects one SASL/TLS mechanism. Only one is active per Config —
// same shape as auth.Config's mode switch, not a struct per mechanism.
type AuthConfig struct {
    // Mechanism: "none" | "plain" | "scram-sha256" | "scram-sha512" | "mtls".
    Mechanism string `mapstructure:"mechanism"`
    Username  string `mapstructure:"username"` // plain, scram-*
    Password  string `mapstructure:"password"` // plain, scram-*
    TLS       TLSConfig `mapstructure:"tls"`    // mtls, or alongside SASL over TLS
}

type TLSConfig struct {
    CertFile string `mapstructure:"cert_file"`
    KeyFile  string `mapstructure:"key_file"`
    CAFile   string `mapstructure:"ca_file"`
}

func DefaultConfig() Config
func (c Config) Validate() error // brokers non-empty; mechanism in the allowed set;
                                  // plain/scram require username+password; mtls requires TLS paths

// Client wraps a segmentio/kafka-go kafka.Writer over a shared *kafka.Transport
// (the Transport carries SASL + TLS, built once from Config). Exports only
// what queue's Kafka backend needs today; a Reader-backed consumer reuses
// buildSASLMechanism/buildTLSConfig, not a new package, once a real consumer
// story exists.
type Client struct { /* unexported *kafkago.Writer */ }

func New(cfg Config, opts ...Option) (*Client, error) // builds sasl.Mechanism + tls.Config from cfg.Auth once,
                                                        // assigns both to a shared *kafka.Transport
func (c *Client) Produce(ctx context.Context, topic string, key, value []byte, headers map[string]string) error
func (c *Client) Close() error
```

`New` resolves `Auth.Mechanism` to a `sasl.Mechanism` in one switch (`plain.Mechanism{Username, Password}`,
`scram.Mechanism(scram.SHA256, username, password)`/`scram.SHA512`) plus a `tls.Config` when `mtls` or when TLS
fields are set alongside SASL — `segmentio/kafka-go`'s `sasl/plain` and `sasl/scram` packages implement the
mechanisms, this package only wires config to them, so there is no hand-rolled SCRAM/PLAIN protocol code to
maintain. `Produce` builds a `kafka.Message{Topic, Key, Value, Headers}` per call and passes it to
`Writer.WriteMessages` — the per-message `Topic` field is why one `Client`/`Writer` can publish to any topic
`queue.Publisher.Publish` names, instead of being pinned to one topic at construction.

### `queue` — publishing abstraction

| File | Contents |
|---|---|
| `queue/queue.go` | `Publisher` interface, `PublishOption`, `WithKey`/`WithHeaders`, mockgen directive |
| `queue/config.go` | `Config{Backend, Kafka}` (mapstructure) + `DefaultConfig()` + `Validate()` + `FromConfig` |
| `queue/noop.go` | `NewNoOp()` |
| `queue/logging.go` | `NewLogging(log logger.Logger)` — generalizes `guest-management-be`'s stopgap |
| `queue/kafka.go` | `NewKafka(client *kafka.Client)` |
| `queue/queue__test.go` | table-driven: noop/logging behavior, option application |
| `queue/README.md` | usage + backend selection |

```go
// Publisher abstracts async message publishing behind a swappable backend.
type Publisher interface {
    // Publish sends message to topic. Fire-and-forget from the caller's
    // perspective; opts customize per-message behavior the active backend
    // can honor (ignored otherwise, never an error).
    Publish(ctx context.Context, topic string, message []byte, opts ...PublishOption) error
}

// PublishOption customizes one Publish call. Only options with a consistent
// meaning across backends belong here — backend-only behavior (ack level,
// compression, partition count, ...) is config on that specific backend,
// never a PublishOption.
type PublishOption func(*publishOptions)

// WithKey sets the message's ordering key (Kafka partition key, SQS FIFO
// message-group-id, ...). Backends without ordering ignore it.
func WithKey(key []byte) PublishOption

// WithHeaders attaches metadata headers to the message.
func WithHeaders(headers map[string]string) PublishOption

type Config struct {
    Backend string      `mapstructure:"backend"` // "noop" | "logging" | "kafka"
    Kafka   kafka.Config `mapstructure:"kafka"`
}

func DefaultConfig() Config
func (c Config) Validate() error
// FromConfig builds a Publisher from cfg.Backend; "kafka" requires an
// already-constructed *kafka.Client (mirrors ratelimit.FromConfig(cfg, redisClient)).
func FromConfig(cfg Config, kafkaClient *kafka.Client) (Publisher, error)

func NewNoOp() Publisher
func NewLogging(log logger.Logger) Publisher
func NewKafka(client *kafka.Client) Publisher
```

`NewKafka`'s `Publish` extracts `key`/`headers` from `opts` and calls `client.Produce(ctx, topic, key, message,
headers)`. `NewLogging` logs topic + message length (never the payload itself — it may carry PII) and returns nil,
replacing `guest-management-be`'s `loggingInvitationPublisher` once wired.

**Mocks:** `//go:generate mockgen` on `queue.Publisher` → `mocks/queue/mock_queue.go`, `./queue/...` added to
`MOCK_PKGS`. `kafka.Client` is a concrete type (like `crypto.Encryptor`), not an interface — nothing to swap
underneath it, so no mock; `queue.Publisher` is the seam consumers and tests use instead.

**Non-goals** (deferred, not required by B8's acceptance criteria):
- **Consumer/`Subscriber`.** Designed in Phase 11 below, added as a `kafka.Reader`-backed type when a real
  consumer story exists.
- **OAUTHBEARER / Kerberos / AWS MSK IAM.** Not built into `segmentio/kafka-go`'s `sasl` package and nothing here
  needs them yet; adding one later is a custom `sasl.Mechanism` implementation plus one more `case` in
  `buildSASLMechanism`, not a redesign.
- **Delivery guarantees tuning (acks, batching, compression, retries/DLQ).** `kafka.Writer`'s defaults apply;
  expose as `kafka.Config` fields only when a concrete reliability requirement shows up.
- **Multi-topic routing / schema registry / Avro.** Callers pass an opaque `[]byte` and their own topic string,
  same as every other SDK serialization boundary (`serializer`).

**Deps:** `github.com/segmentio/kafka-go` (+ `kafka-go/sasl/plain`, `kafka-go/sasl/scram`).

---

## Phase 11 — `queue` Subscriber (Consumer) ✅ DONE

Deferred from Phase 10's own non-goals, then implemented per the design below: `kafka.Consumer` (`kafka/consumer.go`
+ `kafka/consumer__test.go`, extends `kafka/kafka_integration_test.go`'s produce test into a produce/consume round
trip) and `queue.Subscriber` (`queue/subscriber.go`, `queue/kafka_subscriber.go` + tests). Shipped exactly as
designed — no deviations. `NewConsumer`/`NewKafkaSubscriber` take `*Config`, not `Config` by value, matching the
`hugeParam`-driven precedent `kafka.New`/`queue.FromConfig` already set in Phase 10 (the sketch below predates that
lint pass).

**Shape:** two additions, no new packages — extends the two Phase 10 packages exactly as Phase 10 predicted.

- **`kafka.Consumer`** — wraps `segmentio/kafka-go`'s `kafka.Reader`, bound to one `(Config, groupID, topic)` at
  construction (`Reader`'s own model: one `Reader` = one group + one topic's partitions — there's no producer-side
  "any topic per call" equivalent on the consume side). Reuses `buildSASLMechanism`/`buildTLSConfig` from
  `kafka/auth.go`, wired into a `kafka.Dialer` instead of `Writer`'s `Transport` (`Reader` takes a `Dialer`, not a
  `Transport` — different kafka-go type, same two builder funcs, no duplication).
- **`queue.Subscriber`** — the interface `guest-management-be` would import; mirrors `Publisher`'s per-call-topic
  shape. `NewKafkaSubscriber(cfg, groupID)` builds a fresh `kafka.Consumer` inside each `Subscribe` call (scoped to
  that call's topic) and closes it when the call returns.

### API sketch

```go
// kafka/consumer.go
type Consumer struct { /* unexported *kafkago.Reader */ }

// NewConsumer validates cfg and returns a Consumer bound to one group +
// topic. Reuses buildSASLMechanism/buildTLSConfig via a kafka.Dialer.
func NewConsumer(cfg *Config, groupID, topic string) (*Consumer, error)

// Consume blocks: Fetch → handler → Commit, one message at a time, in
// partition order, until ctx is done or handler/Fetch returns an error.
// A handler error stops the loop with that message left uncommitted — see
// Delivery semantics below. Returns nil on clean ctx cancellation.
func (c *Consumer) Consume(ctx context.Context, handler func(ctx context.Context, msg Message) error) error

// Close releases the underlying connection. The Consumer must not be used
// afterward.
func (c *Consumer) Close() error
```

```go
// queue/queue.go (addition alongside Publisher)

// Message is one delivery handed to a Subscriber's Handler.
type Message struct {
    Topic   string
    Key     []byte
    Value   []byte
    Headers map[string]string
}

// Handler processes one Message. A nil error commits it (advances the
// offset); a non-nil error stops the owning Subscribe call with the message
// uncommitted — see Phase 11's Delivery semantics.
type Handler func(ctx context.Context, msg Message) error

// Subscriber abstracts a blocking consume loop behind a swappable backend.
type Subscriber interface {
    // Subscribe blocks, delivering messages from topic to handler one at a
    // time, until ctx is done or an unrecoverable backend error occurs.
    Subscribe(ctx context.Context, topic string, handler Handler) error
}
```

```go
// queue/kafka_subscriber.go
func NewKafkaSubscriber(cfg *kafka.Config, groupID string) Subscriber
```

### Delivery semantics — the one real design decision here

At-least-once, manual commit, **no in-process retry/backoff, no DLQ**: `Consume`'s loop is `Fetch → handler(msg) →
if err != nil { return err }` (loop stops, that message stays uncommitted) `→ Commit → next`. A handler error is
fatal to that `Subscribe` call — the caller (a goroutine the app owns, e.g. started alongside `lifecycle.Run` and
canceled by the same shutdown context) decides whether to restart it. On restart, the consumer group resumes from
the last committed offset, so the failed message (and anything fetched-but-uncommitted after it) is redelivered.
This is the simplest correct at-least-once strategy — crash-and-resume, not in-loop retry — and matches Phase 10's
own "delivery guarantees tuning" non-goal: retry/backoff/DLQ is a real feature with real tradeoffs (max attempts,
poison-message handling), not a default a subscriber should silently apply.

**No `NoOp`/`Logging` Subscriber backends.** `Publisher.NewNoOp` lets a caller "publish to nowhere" — a meaningful
no-op. A subscriber with nothing to consume from isn't: there's no message source to fake, so a `NoOpSubscriber`
would just block on `<-ctx.Done()` forever — a test/wiring convenience, not a production stand-in. Build it only
if a concrete test need shows up; nothing here requires it speculatively.

### Non-goals

- **Multi-topic single-group fan-in** (`kafka.ReaderConfig.GroupTopics`). One `Consumer` = one topic; a service
  that needs several topics runs several `Subscribe` calls (goroutines) — same as running several independent
  consumers in any other design. `GroupTopics` exists in kafka-go for a narrower case (identical processing across
  topics under one group) nothing here needs yet.
- **Retry/backoff/DLQ.** See Delivery semantics above — crash-and-resume is the v1 strategy; a real retry policy
  is a follow-up once a concrete reliability requirement exists (same posture as Phase 10's deferred delivery
  tuning).
- **Concurrent/batched handler dispatch.** `Consume` processes one message at a time, in order, on the calling
  goroutine — matches `kafka.Reader`'s own per-partition ordering guarantee. Fan-out to worker goroutines, if
  throughput ever needs it, is the caller's concern, not this package's.
- **`NoOp`/`Logging` Subscriber backends.** See above.
- **OAUTHBEARER / Kerberos / AWS MSK IAM, consumer-side tuning knobs** (min/max bytes, max wait, queue capacity,
  ...) beyond group + topic. kafka-go defaults apply; same posture as Phase 10's producer non-goals — add a
  `Config` field only when a concrete need shows up.

**Deps:** none beyond what Phase 10 already added (`segmentio/kafka-go`).

---

## Recommended middleware chain

```go
log := logger.NewZerolog(&logger.Options{ContextExtractor: ctxkit.LoggerExtractor()})
handler := middleware.Chain(mux,
    middleware.Metrics(rec, nil),                 // outermost: count all incl. panics
    middleware.Recover(),
    middleware.RequestID(),                       // ctxkit.WithRequestID
    middleware.Correlation(),                     // ctxkit.WithCorrelationID
    middleware.Tracing(tr),                       // ctxkit.WithTraceID
    middleware.Logging(log, nil),                 // logs request_id + correlation_id + trace_id
    middleware.RateLimit(limiter, middleware.KeyByIP),
    middleware.Auth(validator, middleware.WithPolicy(pol)), // ctxkit.WithUserID
)
srv := &http.Server{Addr: ":8080", Handler: handler}
_ = lifecycle.Run(ctx, srv,
    lifecycle.WithReadiness(&ready),
    lifecycle.WithCloser("tracer", lifecycle.CloserFromTracer(tr)),
    lifecycle.WithCloser("redis", lifecycle.CloserFromRedis(rdb)),
    lifecycle.WithCloser("db", lifecycle.CloserFromDB(db)),
)
```

Wrap outbound dependency calls with a `circuitbreaker.Breaker`. Validate request bodies in handlers with the
`validator` package. Every `log.*WithContext(ctx, ...)` surfaces `request_id`, `correlation_id`, `trace_id`, `user_id`.

---

## AGENTS.md package-map additions

`ctxkit` row already added. Add the rest as each phase lands:

| Package | Role |
|---|---|
| `tracer` | `Tracer`/`Span` interfaces; OTel OTLP-gRPC backend; noop; W3C propagation; server middleware |
| `auth` | `Validator`/`Claims`/`Issuer`; remote (mappable) + HS256/RS256/JWKS; config-driven route `Policy`; token issuing |
| `metrics` | `Recorder` interface; Prometheus backend; HTTP request count/duration/in-flight middleware |
| `validator` | `Validator` interface wrapping go-playground/validator; struct-tag validation; `errorz` field errors |
| `ratelimit` | `Limiter` interface; in-memory + Redis backends; 429 middleware with headers |
| `circuitbreaker` | Own closed/open/half-open state machine; `Do[T]`; wraps outbound calls; `ErrOpen`→503 |
| `lifecycle` | `Run()` graceful shutdown: signal trap, readiness drain, ordered `Closer` cleanup under deadline |

---

## Critical existing files to reference

- `logger/zerolog.go` — `defaultContextExtractor` (replaced by `ctxkit.LoggerExtractor`); `ContextExtractor` / `Field`.
- `httpkit/middleware/requestid.go` — typed-key + exported-key pattern; hex id generator.
- `httpkit/middleware/logging.go` — `responseCapture` reused by tracing/metrics/ratelimit middleware.
- `httpkit/handler/status.go` — `StatusCodeFromError` (errorz code → HTTP status).
- `httpkit/client/client.go` — `Do[T]/Get[T]/Post[T]`; remote auth + outbound propagation.
- `errorz/error.go` — `Unauthorized()`, `BadRequest()`, `TooManyRequests()`, `ServiceUnavailable()`, `Code*`, `Wrap`/`WithCode`.
- `config/env.go` + config README — `Load(dst, opts...)` used by every `FromConfig`.
- `redis/client.go` — `redis.Client` interface (ratelimit Redis backend + lifecycle closer).
- `sqlkit/db.go` — `*DB.Close()` for lifecycle closer; `otelsql` driver-level tracing note.
- `serializer/serializer.go` — `ParseJSON` used alongside `validator` in request handlers.
