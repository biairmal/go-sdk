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
