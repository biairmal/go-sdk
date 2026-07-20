# Auth Package

`auth` provides monolith-first authentication: a `Validator` verifies bearer tokens, an `Issuer` mints them, and a config-driven route `Policy` decides which requests need authenticating. The same interfaces serve a monolith today (in-process JWT issue + validate) and split into services later by configuration alone.

## Overview

`auth.Validator` is the seam. In a monolith you issue and validate JWTs in one process (`NewHS256` / `NewRS256`); when you split out an identity service you either point `RS256` at its JWKS endpoint or switch to the `remote` validator — the calling code and the route policy do not change, only YAML does. Validation failures are returned as `*errorz.Error` with code `errorz.CodeUnauthorized`, so they flow through `httpkit` as clean 401s. On success, [`httpkit/middleware.Auth`](../httpkit/middleware) stores the validated `Claims` in the request context and publishes the subject via [`ctxkit`](../ctxkit/README.md), so `user_id` appears on every log line.

## Features

- **Interface-first**: `Validator`, `Claims`, and `Issuer` hide the backend; swap HS256, RS256, JWKS, remote, or a `ValidatorFunc` without touching call sites.
- **Local JWT**: `NewHS256` / `NewRS256` verify in-process. RS256 accepts a static PEM public key or a JWKS URL (`WithJWKSURL`).
- **JWKS**: keys are fetched by `kid`, cached with a TTL, refreshed on rotation, rate-limited on unknown kids, and served stale if a refresh fails.
- **Remote validation**: `NewRemote` calls an identity service with a fully-mappable JSON response (dotted-path field mapping) and configurable token forwarding (header, body, or query).
- **Token issuing**: `NewHS256Issuer` / `NewRS256Issuer` sign `sub/iss/aud/iat/exp` plus caller claims and roles.
- **Route policy**: a config-driven glob matcher (`*` = one path segment, `**` = zero or more) marks routes public or protected; first matching rule wins.
- **Validation cache**: `NewCached` wraps any `Validator` with a TTL cache of successful validations, keyed by a hash of the token and capped by the token's own `exp`.
- **Config-driven**: `FromConfig` / `IssuerFromConfig` build the whole stack from a `mapstructure`-tagged `Config`; transition is a config flip, not a code change.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/auth
```

### Basic usage (monolith: issue + validate)

```go
issuer, _ := auth.NewHS256Issuer("shared-secret",
    auth.WithIssuerName("orders-api"), auth.WithDefaultTTL(time.Hour))
token, _ := issuer.Issue("user-42", auth.WithRoles("admin"))

validator, _ := auth.NewHS256("shared-secret", auth.WithExpectedIssuer("orders-api"))
claims, err := validator.Validate(ctx, token)
if err != nil {
    // *errorz.Error with CodeUnauthorized → 401 at the HTTP edge
}
fmt.Println(claims.Subject(), claims.Roles())
```

### Config-driven setup

```go
v, err := auth.FromConfig(&cfg.Auth)      // selects HS256 / RS256 / JWKS / remote by cfg.Mode
iss, err := auth.IssuerFromConfig(&cfg.Auth) // builds the token issuer
```

### HTTP server middleware

```go
handler := middleware.Chain(mux,
    middleware.RequestID(),
    middleware.Tracing(tr),
    middleware.Logging(log, nil),
    middleware.Auth(v, middleware.WithPolicy(cfg.Auth.Policy())), // innermost; publishes user_id
)
```

Public routes (per the policy) pass through untouched; protected routes require a valid `Authorization: Bearer <token>`. Read the caller in handlers with `auth.ClaimsFromContext(ctx)` or `auth.SubjectFromContext(ctx)`.

### Route policy

```yaml
default_protected: true
rules:
  - pattern: /health
    public: true
  - pattern: /swagger/**      # ** matches /swagger and any subtree
    public: true
  - method: POST
    pattern: /api/v1/auth/login
    public: true
```

### Monolith → microservices transition

| Stage | `mode` | Validator | Network hop |
|---|---|---|---|
| Monolith | `local` | `NewHS256` / `NewRS256(WithPublicKey)` | No |
| Monolith, custom logic | `func` | `ValidatorFunc` | No |
| Split, key-based | `local` + `jwks_url` | `NewRS256(WithJWKSURL)` | Cached key fetch |
| Split, delegated | `remote` | `NewRemote(...)` | Yes |

Smoothest path is RS256 + JWKS: a static public key in the monolith, then point `jwks_url` at the identity service on split — the constructor never changes.

## Options

Validator options (`NewHS256`, `NewRS256`, `NewRemote`):

| Option | Description |
|--------|-------------|
| `WithExpectedIssuer(iss)` | Require the token's `iss` claim to equal `iss`. |
| `WithExpectedAudience(aud)` | Require the token's `aud` claim to contain `aud`. |
| `WithLeeway(d)` | Clock-skew tolerance for `exp`/`nbf`/`iat`. Default 0. |
| `WithPublicKey(pub)` | RS256 static verification key. |
| `WithJWKSURL(url)` | RS256 keys fetched by `kid` from a JWKS endpoint. |
| `WithHTTPClient(c)` | HTTP client for JWKS and remote requests. |

Issuer construction options (`NewHS256Issuer`, `NewRS256Issuer`):

| Option | Description |
|--------|-------------|
| `WithIssuerName(iss)` | The `iss` claim written on issued tokens. |
| `WithDefaultTTL(d)` | Default token lifetime. Default 15m. |
| `WithDefaultAudience(aud)` | Default `aud` claim. |

Issue-time options (`Issue`):

| Option | Description |
|--------|-------------|
| `WithTTL(d)` | Override lifetime for this token. |
| `WithAudience(aud)` | Override `aud` for this token. |
| `WithRoles(...)` | Set the `roles` claim. |
| `WithIssuedAt(t)` | Pin `iat`/`exp` basis (deterministic tests). |
| `WithExtraClaims(m)` | Add caller claims (registered claims always win). |

## Limitations

- **No refresh tokens.** Only access-token issue/validate is in scope for v1.
- **RSA only.** JWKS parsing and RS256 handle RSA keys; EC/EdDSA and X.509 certificate PEM blocks are unsupported.
- **Validation cache trades revocation latency.** A cached token is accepted until its cache entry expires even if revoked upstream; keep `cache_ttl` short relative to token lifetime.
- **Remote body forwarding is JSON.** `forward.in: body` sends `{"<name>": "<token>"}`, not RFC 7662 form-encoding.
- **JWKS refresh holds a write lock** during the fetch (a stdlib single-flight substitute): concurrent misses serialize behind one refresh.

## Dependencies

- [github.com/golang-jwt/jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) – JWT signing and verification. JWKS, the remote validator, the policy matcher, and PEM parsing use only the standard library.

## See also

- [ctxkit](../ctxkit/README.md) – `WithUserID`/`UserID` carry the authenticated subject through context and into logs.
- [httpkit](../httpkit/README.md) – `middleware.Auth` is the standard server-side integration point; `handler.StatusCodeFromError` maps auth errors to HTTP status.
- [errorz](../errorz/README.md) – token failures are `errorz.Unauthorized()` errors; compare with `errors.Is`.
