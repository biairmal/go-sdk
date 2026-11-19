# Tech debt

Findings from the full review of `go-sdk` (all of `lib/`, `docs/`, `scripts/`, CI, compose file). Ordered by priority:
work top to bottom. Tick the box when the fix is merged.

**Source and limits:** found by reading the code. `go build`, `go vet` and `go test -short ./...` passed at review
time. Not run: `-race`, integration tests, and the exploits themselves. Write a failing test first for each item, then
fix it.

**Priority key**

| Tier | Meaning |
|---|---|
| P0 | Security hole or secret exposure. Fix before the next release |
| P1 | Outage, crash or memory problem under load |
| P2 | Wrong behaviour, unsafe default, missing test, doc that lies |
| P3 | Small cleanups |

---

## P0 — security

### [x] TD-01 SQL filter and sort fields can be injected
- **Where:** [helpers.go:157-164](../lib/repository/sql/helpers.go) (`SanitizeColumnName`), `BuildWhereClause`, `BuildOrderByClause`
- **Problem:** The guard is a deny-list (`' ; " \ ( ) [ ] { }`). Spaces, `=`, `*`, `-` pass. Field `1=1 OR id` gives
  `WHERE tenant_id = $1 AND 1=1 OR id = $2`, which skips the tenant filter. `UNION SELECT *` is probably possible.
  Matters whenever a field name comes from a request.
- **Fix:** Allow only columns the entity has (`getOrderedColumns(r.entityType)`). Reject anything else with an error.
  Delete `SanitizeColumnName`.
- **Done when:** a test with field `1=1 OR id` returns an error, and a test with a real column still works.

### [x] TD-02 Bodies are logged by default, with no size limit
- **Where:** [logging.go:29-36](../lib/httpkit/middleware/logging.go), [logging.go:70-80](../lib/httpkit/middleware/logging.go)
- **Problem:** `opts == nil` logs every request and response body (passwords, tokens). `io.ReadAll` reads the whole
  request body, and `MaxBodyBytesForLogging` defaults to 0 (no limit). `SECURITY.md` says to leave this off.
- **Fix:** Default `LogRequestBody` and `LogResponseBody` to false. Read at most the limit with `io.LimitReader`
  (and restore the rest of the body).
- **Done when:** default options log no body, and a 10 MB body does not allocate 10 MB for logging.

### [x] TD-03 Route policy fails open when `default_protected` is missing
- **Where:** [policy.go:20-26](../lib/auth/policy.go), [config.go:196-208](../lib/auth/config.go)
- **Problem:** Zero value is `DefaultProtected: false`. `auth.Policy{Rules: ...}` or YAML without the key makes every
  unmatched route public. `withDefaults` cannot tell "unset" from "false".
- **Fix:** Invert to `default_public` (zero value = protected), or make the key required in `Validate`.
- **Done when:** an empty `Policy{}` protects every route, with a test.

### [x] TD-04 Client IP can be faked, and has the port when there is no proxy
- **Where:** [logging.go:121-132](../lib/httpkit/middleware/logging.go) (`clientIP`), used by `KeyByIP`, `KeyByUser`, `KeyByHeader`
- **Problem:** Trusts `X-Forwarded-For` / `X-Real-IP` from anyone, so a fake IP per request gets a fresh rate-limit
  bucket. Without a proxy it returns `RemoteAddr` (`ip:port`), so each new connection gets its own bucket.
- **Fix:** `net.SplitHostPort` on `RemoteAddr`. Read forwarded headers only when the peer is in a configured
  trusted-proxy list.
- **Done when:** tests cover: spoofed header ignored, header honoured from a trusted proxy, port stripped.

### [x] TD-05 DB passwords with special characters fail, and MySQL ignores `ssl_mode`
- **Where:** [config.go:50-88](../lib/sqlkit/config.go) (`DBConfig.DSN`)
- **Problem:** `url.QueryEscape` on the password, but the Postgres key=value DSN and the MySQL DSN take it literally.
  `p@ss word` is sent as `p%40ss+word`. The MySQL DSN has no TLS option, so `ssl_mode` is silently ignored.
  `SSLMode` has no default and no validation. `sqlkit` has no tests.
- **Fix:** Postgres: build with `url.URL` and `url.UserPassword`. MySQL: use `mysql.Config.FormatDSN()` and map
  `ssl_mode` to `tls=`. Validate `ssl_mode` against the allowed values.
- **Done when:** table test with passwords containing `@ # + space % :` for both drivers, and a test that MySQL gets a TLS setting.

---

## P1 — outage, crash, memory

### [ ] TD-06 JWKS outage: every request retries the fetch and blocks the others
- **Where:** [jwks.go:64-89](../lib/auth/jwks.go), [jwks.go:101-103](../lib/auth/jwks.go)
- **Problem:** A failed fetch never updates `fetchedAt`, so the cache stays stale. Each request takes the write lock
  and refetches (up to 10 s), one at a time. The 30 s cooldown only covers unknown `kid`s.
- **Fix:** Record the time of every attempt (success or failure). Apply the cooldown to stale refreshes too.
- **Done when:** a test with a failing endpoint shows one fetch per cooldown, and stale keys still validate.

### [ ] TD-07 JWKS outage returns 401 instead of 502
- **Where:** [jwt.go:59-68](../lib/auth/jwt.go) (`mapJWTError`)
- **Problem:** The 502 error from key lookup falls into the default branch and becomes `ErrInvalidToken` (401).
  Clients treat it as a bad session and log users out.
- **Fix:** If `errors.As` finds an `*errorz.Error` that is not a token error, return it unchanged.
- **Done when:** a test with an unreachable JWKS and no cached keys gets a 502.

### [ ] TD-08 Every response is copied into memory by Metrics and Tracing
- **Where:** [logging.go:141-167](../lib/httpkit/middleware/logging.go) (`responseCapture`), [metrics.go:44](../lib/httpkit/middleware/metrics.go), [tracing.go:56](../lib/httpkit/middleware/tracing.go)
- **Problem:** `Write` always appends to `buf`. Metrics and Tracing never read it, so big responses are held in RAM
  two or three times. The wrapper also hides `Flusher` and `Hijacker`, which breaks streaming (SSE).
- **Fix:** Make the buffer optional, only for logging. Metrics and Tracing keep just the status. Keep `Unwrap` so
  `http.ResponseController` works.
- **Done when:** a benchmark or test shows no body copy under Metrics/Tracing, and a streamed response still flushes.

### [ ] TD-09 Memory rate limiter's key cap is not a cap
- **Where:** [memory.go:83-90](../lib/ratelimit/memory.go)
- **Problem:** At `MaxKeys` it evicts only buckets idle 10 minutes, then inserts anyway. With TD-04 the map grows
  without limit. Each new key at the cap also scans the whole map under the global mutex.
- **Fix:** If still full after eviction, reject or evict the oldest entry. Avoid a full scan per request.
- **Done when:** a test sending 10x `MaxKeys` fresh keys keeps `len(buckets) <= MaxKeys`.

### [ ] TD-10 `List(ctx, nil)` panics
- **Where:** [sql_repository.go:207-229](../lib/repository/sql/sql_repository.go)
- **Problem:** `buildListQuery` handles nil, but `List` then reads `opts.SkipCount` and `opts.Filter` on the nil pointer.
- **Fix:** `if opts == nil { opts = &repository.ListOptions{} }` at the top of `List`.
- **Done when:** a test calls `List(ctx, nil)` and gets rows, not a panic.

---

## P2 — correctness, unsafe defaults, tests, docs

### [ ] TD-11 Bad filters are silently dropped and widen the result
- **Where:** [helpers.go:55](../lib/repository/sql/helpers.go), [helpers.go:78-86](../lib/repository/sql/helpers.go)
- **Problem:** Empty `IN`, unknown operator, or rejected field is skipped. `id IN ()` returns every row.
- **Fix:** Return an error for unknown operators and fields. Render an empty `IN` as `1=0`.

### [ ] TD-12 Write-through cache can store a stale row
- **Where:** [decorator.go:148](../lib/repository/cache/decorator.go), [base.go:76-81](../lib/repository/sql/base.go)
- **Problem:** After `Update`, the re-read goes to a follower. Replication lag caches the old row for the TTL (5 min).
  The package has no tests and no doc comments (breaks AGENTS.md).
- **Fix:** Read from the leader there, or just invalidate. Add a test and doc comments.

### [ ] TD-13 Failed SMTP `QUIT` is reported as a send failure
- **Where:** [smtp.go:192](../lib/mailer/smtp.go)
- **Problem:** After the server accepted the mail, a `Quit` error makes `Send` fail, and callers retry and duplicate.
- **Fix:** `_ = c.Quit(); return nil`.

### [ ] TD-14 Metrics `path` label defaults to the raw URL path
- **Where:** [metrics.go:31](../lib/httpkit/middleware/metrics.go), [metrics.go:60-64](../lib/httpkit/middleware/metrics.go)
- **Problem:** Each `/orders/42` or random 404 path is a new Prometheus series. Anyone can create unlimited series.
- **Fix:** Default to `r.Pattern` (read after the handler returns), falling back to `"unmatched"`.
- **Also:** `PathNormalizer` is called before `next.ServeHTTP`, so a normalizer that reads the router's matched
  pattern (chi `RoutePattern()`, `r.Pattern`) always sees an empty value. Call it after the handler returns, so a
  service can follow the README's advice. go-service-template TD-04 works around this until then.

### [ ] TD-15 Auth validation cache can grow without bound
- **Where:** [cache.go:77-81](../lib/auth/cache.go)
- **Problem:** At 10,000 entries it sweeps expired ones, then inserts anyway. With many live tokens the map keeps
  growing, and each insert scans the whole map under the write lock.
- **Fix:** If still full after the sweep, evict one entry or skip caching.

### [ ] TD-16 Issuer and audience are optional, and HS256 accepts any secret length
- **Where:** [jwt.go:30-38](../lib/auth/jwt.go), [config.go:136](../lib/auth/config.go)
- **Problem:** Without them, any token signed with the key is accepted. A 1-character HS256 secret passes `Validate`.
- **Fix:** Require `issuer` and `audience` in `validateLocal` with an explicit opt-out. Require at least 32 bytes
  for the HS256 secret. Update `SECURITY.md`.

### [ ] TD-17 Repository logs query arguments
- **Where:** [sql_repository.go:78-87](../lib/repository/sql/sql_repository.go)
- **Problem:** Debug logs print `args: %v`: emails, hashes, phone numbers. `tracesql` avoids arguments on purpose.
- **Fix:** Log the query and the argument count only.

### [ ] TD-18 `ScanRow` swallows bad data
- **Where:** [scan.go:63-66](../lib/repository/sql/scan.go), [scan.go:89-102](../lib/repository/sql/scan.go)
- **Problem:** An invalid UUID gives a zero UUID and no error. The type-mismatch branch returns `(nil, nil)`, so
  `GetByID` could return a nil entity and no error.
- **Fix:** Return the parse error. Return an error in the mismatch branch.

### [ ] TD-19 Repository claims multi-dialect support but only works on Postgres
- **Where:** [sql_repository.go:122-129](../lib/repository/sql/sql_repository.go), [sql_repository.go:175-181](../lib/repository/sql/sql_repository.go), [helpers.go:174](../lib/repository/sql/helpers.go)
- **Problem:** `RETURNING id` is hardcoded (fails on MySQL). MySQL `RowsAffected` is 0 for an unchanged row, so
  `Update` returns `ErrNotFound`. Unique-violation mapping is a TODO.
- **Fix:** Decide: Postgres-only (delete the other dialects, document it) or add real dialect handling and tests.

### [ ] TD-20 `sqlkit` housekeeping
- **Where:** [db.go:7](../lib/sqlkit/db.go), [db.go:41-82](../lib/sqlkit/db.go), [db.go:219](../lib/sqlkit/db.go)
- **Problem:** Uses stdlib `log` (AGENTS.md forbids it). `New` overwrites the caller's `cfg.Pool` and `cfg.Health`.
  Followers that fail at startup are never retried. Retries use `time.Sleep` and ignore `ctx`. `Follower()` takes a
  global mutex on every read.
- **Fix:** Optional logger. Copy the config. Pick followers with an atomic counter. Retry on a context-aware timer.

### [ ] TD-21 Authz identity source: no backoff on failure
- **Where:** [source_identity.go:91-109](../lib/authz/source_identity.go)
- **Problem:** Once the stale map is older than 10×TTL and identity is down, every request waits up to 5 s for a
  failing fetch, one at a time.
- **Fix:** Cache the failure for a few seconds and return it at once.

### [ ] TD-22 One bad Kafka message blocks its partition
- **Where:** [consumer.go:69-87](../lib/kafka/consumer.go)
- **Problem:** A handler error stops the loop with the message uncommitted, so it is redelivered and fails forever.
  A cancel during commit returns an error, not nil. One synchronous commit per message limits throughput.
- **Fix:** Optional dead-letter or retry-count hook. Document the poison-message behaviour. Treat a `ctx` error from
  commit as a clean stop.

### [ ] TD-23 `crypto`: no associated data, no key ID
- **Where:** [crypto.go:69-96](../lib/crypto/crypto.go)
- **Problem:** Nothing binds ciphertext to its row or column. Someone with table write access can copy ciphertext
  between rows or columns and it still decrypts. No key version in the output, so rotation needs a flag day.
- **Fix:** Add `EncryptWithAAD` / `DecryptWithAAD` taking a context string such as `"users.email:"+id`. Prefix the
  output with a one-byte key version. Do it before real data exists in this format.

### [ ] TD-24 Config: `${VAR}` substitution edits raw file text
- **Where:** [substitute.go:18-31](../lib/config/substitute.go), [config.go:80-82](../lib/config/config.go)
- **Problem:** A value with `#`, `: ` or a newline changes the YAML. An unset variable silently becomes `""`.
  `Load` with no files probably returns zeros, because Viper's `AutomaticEnv` does not fill unknown keys (from
  memory of Viper's behaviour, not run).
- **Fix:** Substitute inside parsed string values. Add a `${VAR:?}` "required" form. Fail when `Load` gets no files.

### [ ] TD-25 HTTP client has no timeout or size limit; request ID is unchecked
- **Where:** [client.go:23](../lib/httpkit/client/client.go), [client.go:46](../lib/httpkit/client/client.go), [requestid.go:29](../lib/httpkit/middleware/requestid.go), [correlation.go:23](../lib/httpkit/middleware/correlation.go)
- **Problem:** `http.DefaultClient` has no timeout, and `io.ReadAll` has no limit. The `Correlation` comment says the
  client forwards the ID, but it does not. Request and correlation IDs are accepted at any length or characters,
  echoed in headers, and written to logs. The client has no tests.
- **Fix:** Default client with a timeout and `io.LimitReader`. Forward `X-Correlation-Id` and `traceparent`, or fix
  the comment. Accept IDs only if at most 64 chars and `[A-Za-z0-9._-]`.

### [ ] TD-26 CI and supply chain
- **Where:** [ci.yml:16-26](../.github/workflows/ci.yml), [security.mk:16](../scripts/security.mk), [Makefile:49-61](../Makefile)
- **Problem:** `gofumpt` and `govulncheck` installed with `@latest`. Actions pinned by tag, not SHA. Go is `stable`.
  CI never runs `-race`. No Dependabot or Renovate. `make check` runs `lint-fix`, which rewrites files, so local and
  CI results can differ. No `LICENSE`.
- **Fix:** Pin tool versions and action SHAs. Add `go test -race -short ./...`. Add `.github/dependabot.yml`. Run
  `lint`, not `lint-fix`, in the gate. Add a `LICENSE`.

### [ ] TD-27 Docs that disagree with the code, and missing tests
- **Where:** [SECURITY.md](../SECURITY.md), [AGENTS.md](../AGENTS.md), [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md)
- **Problem:** `SECURITY.md` says "leave body logging off" (it is on by default) and "JWKS refetch at most every 30 s"
  (false when stale). The correlation-forwarding claim is false. `AGENTS.md` single-package examples use
  `./repository/...` but the path is `./lib/repository/...`. `sqlkit`, `httpkit/client` and `repository/cache` have
  no test files.
- **Fix:** Re-check each claim after TD-02, TD-06 and TD-25 land. Fix the paths. Add one test file for each of the
  three untested packages.

### [ ] TD-36 `handler.Handle` never records the error behind a 5xx
- **Where:** [handler.go:15-37](../lib/httpkit/handler/handler.go), [writer.go](../lib/httpkit/handler/writer.go)
- **Problem:** `Handle` writes a masked error envelope and logs nothing (`Recover` logs panics since v0.6.0, but a
  returned error does not panic). The logging middleware sees only the status. A service that wraps `Handle` with
  nothing else gets `status=500` in its logs and no cause. Found in go-service-template TD-06.
- **Fix:** Add an optional, nil-safe `handler.WithLogger(log)` option (libraries do not log by default, per
  AGENTS.md). With it set, log once at error level for status 500 or more: the error, method, path and the request
  context's IDs. Never log the response body.
- **Done when:** a test with a handler that returns a wrapped error finds one error log line with the cause, and a
  test without the option logs nothing.

### [ ] TD-37 `config.Load` ignores unknown keys
- **Where:** [config.go](../lib/config/config.go)
- **Problem:** A key that matches no struct field is dropped without a word. v0.6.0 renamed the auth policy key
  `default_protected` to `default_public`; a service that kept the old key still loads and the key does nothing
  (go-service-template TD-16). The same happens for a typo in any key.
- **Fix:** Add an option, for example `config.Strict()`, that sets mapstructure `ErrorUnused` and returns the unknown
  key names. Keep the default as is so existing services do not break, and document it in the package README.
- **Done when:** a test loads a file with an unknown key and gets an error naming it with `Strict()`, and loads
  cleanly without it.

---

## P3 — small items

- [ ] **TD-28 Policy path matching:** `Policy` matches the raw path (`//`, `..` not cleaned). Mostly safe behind
  `ServeMux`. Run `path.Clean` before matching. ([policy.go:31-38](../lib/auth/policy.go))
- [ ] **TD-29 Retry-After rounds down:** `int(RetryAfter.Seconds())` turns 100 ms into `Retry-After: 0`. Round up.
  ([ratelimit.go:130](../lib/httpkit/middleware/ratelimit.go))
- [ ] **TD-30 Redis limiter key has no prefix:** a header value could hit another app's key. Prefix with `rl:`.
  ([redis.go:79](../lib/ratelimit/redis.go))
- [ ] **TD-31 Redis client:** `NewClient` does not close the client when `Ping` fails, has no doc comment, and allows
  only two TLS cipher suites. ([client.go:82-174](../lib/redis/client.go))
- [ ] **TD-32 Tracer sample rate:** `sample_rate: 0` becomes 1.0 and samples everything. Use a pointer or an
  `enabled` flag. ([config.go:50-52](../lib/tracer/config.go))
- [ ] **TD-33 Shutdown hook:** in `lifecycle`, the hook runs with `context.Background()` and is not covered by the
  second-signal escape. ([lifecycle.go:123-128](../lib/lifecycle/lifecycle.go))
- [ ] **TD-34 `Success` with status 0:** `Success{Data: x}` makes `WriteHeader(0)` panic. Default to 200.
  ([handler.go:24-28](../lib/httpkit/handler/handler.go))
- [ ] **TD-35 Compose ports:** Redis and Kafka are published on all interfaces. Bind to `127.0.0.1:`.
  ([docker-compose.integration.yaml](../docker-compose.integration.yaml))
- [ ] **TD-38 Logging middleware has no skip list:** it writes one Info line per response, so health probes and
  metrics scrapes fill the log. Add `LoggingOptions.SkipPaths []string` (exact match on the path). Found in
  go-service-template TD-17. ([logging.go](../lib/httpkit/middleware/logging.go))
- [ ] **TD-39 Service-token exchange is copied per service:** every service that calls identity's `/v1/internal`
  needs the client-credentials call, a cache and a refresh margin. go-service-template has its own
  ([service_token.go](https://github.com/biairmal/go-service-template/blob/main/internal/app/service_token.go)) and
  feeds it to `authz.WithServiceToken`. Move one tested copy next to `authz` (for example
  `authz.NewClientCredentials(url, id, secret, opts...)`), with a timeout on its HTTP client.

### Lean (delete or merge, roughly -100 lines)

- [ ] Four copies of the "find the ID field" loop in [crud.go](../lib/repository/sql/crud.go) (`IsEntityIDZero`, `getEntityIDFieldInfo`, `SetEntityID`, `BuildInsertQuery`): merge into one helper.
- [ ] Deprecated `ReflectScan` in [scan.go:104-109](../lib/repository/sql/scan.go): delete.
- [ ] `generateRequestID` and `generateCorrelationID` are the same function: merge.
- [ ] `WriteBehindStrategy` is unimplemented and falls back to write-around: delete or implement.
- [ ] Extra SQL dialects, if TD-19 ends up Postgres-only.
