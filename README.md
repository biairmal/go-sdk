# go-sdk

[![CI](https://github.com/biairmal/go-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/biairmal/go-sdk/actions/workflows/ci.yml)

The shared infrastructure layer for Go services: configuration, structured errors, HTTP plumbing, logging, SQL and
Redis access, authentication and permissions, observability, resilience, encryption, messaging and mail. It is a
set of independent packages, so a service imports only what it uses. Public APIs use standard-library types
(`http.Handler`, `*sql.DB`, `context.Context`), so any router or driver works.

[go-identity](https://github.com/biairmal/go-identity) and
[go-service-template](https://github.com/biairmal/go-service-template) are built on it. New to the codebase? Start
with [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

> The API is pre-1.0: a minor version (`v0.x.0`) may change exported APIs. Pin a version.

## Install and use

Needs Go 1.26+.

```bash
go get github.com/biairmal/go-sdk@latest
```

A JSON endpoint with the standard error envelope, panic recovery and request logging:

```go
package main

import (
    "net/http"

    "github.com/biairmal/go-sdk/lib/errorz"
    "github.com/biairmal/go-sdk/lib/httpkit"
    "github.com/biairmal/go-sdk/lib/httpkit/handler"
    "github.com/biairmal/go-sdk/lib/httpkit/middleware"
    "github.com/biairmal/go-sdk/lib/httpkit/response"
    "github.com/biairmal/go-sdk/lib/logger"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", httpkit.Health())
    mux.Handle("/notes/missing", handler.Handle(func(r *http.Request) (any, error) {
        return nil, errorz.NotFound().WithMessage("note not found") // 404 with the error envelope
    }))
    mux.Handle("/ping", handler.Handle(func(r *http.Request) (any, error) {
        return response.OK(map[string]string{"pong": "ok"}), nil
    }))

    h := middleware.Chain(mux, middleware.Recover(logger.NewNoOp()), middleware.Logging(logger.NewNoOp(), nil))
    _ = http.ListenAndServe(":8080", h)
}
```

Every package has its own README with usage, options and limitations. For a full service wired from these
packages, start from [go-service-template](https://github.com/biairmal/go-service-template).

## Packages

All packages live under `lib/`. Each row links to that package's README.

| Concern | Packages |
|---|---|
| Foundation | [errorz](lib/errorz/README.md) (structured errors), [logger](lib/logger/README.md), [config](lib/config/README.md) (YAML, `.env`, `${VAR}`), [ctxkit](lib/ctxkit/README.md) (request-scoped IDs), `common/dto` (paging DTOs) |
| HTTP | [httpkit](lib/httpkit/README.md) (handler adapter, middleware, response envelope, health, client), [validator](lib/validator/README.md), [serializer](lib/serializer/README.md) |
| Data | [sqlkit](lib/sqlkit/README.md) (leader/follower, transactions), [repository](lib/repository/README.md) (generic CRUD), `redis` (go-redis client) |
| Security | [auth](lib/auth/README.md) (JWT validate and issue, JWKS, route policy), [authz](lib/authz/README.md) (permissions), [crypto](lib/crypto/README.md) (field encryption, blind index), [ratelimit](lib/ratelimit/README.md) |
| Observability | [tracer](lib/tracer/README.md) (OpenTelemetry), [tracesql](lib/tracer/tracesql/README.md) (traced SQL driver), [metrics](lib/metrics/README.md) (Prometheus) |
| Resilience and lifecycle | [circuitbreaker](lib/circuitbreaker/README.md), [lifecycle](lib/lifecycle/README.md) (graceful shutdown) |
| Messaging and mail | [kafka](lib/kafka/README.md), [queue](lib/queue/README.md) (publish and subscribe), [mailer](lib/mailer/README.md) (SMTP, templates) |

**Mocks** for the exported interfaces are a separate module, so gomock never becomes a dependency of your build:

```bash
go get github.com/biairmal/go-sdk/mocks@latest   # test dependency only; see mocks/README.md
```

How to test against the SDK (fakes first, then mocks, miniredis, sqlmock) is in [docs/TESTING.md](docs/TESTING.md).

## Development

You need Go 1.26+ and Make (on Windows, Git Bash or WSL).

```bash
make install-tools      # gofumpt, golangci-lint, govulncheck
make check              # format, lint-fix, unit tests, coverage, vulncheck, deps-verify
make test-integration   # Redis and Kafka tests; see CONTRIBUTING.md for the services they need
make help               # every target
```

| Path | Holds |
|---|---|
| `lib/` | The packages, one directory each, each with a README |
| `mocks/` | Generated gomock mocks, a separate Go module |
| `scripts/` | Makefile fragments and the Kafka test certificate script |
| `docs/` | [Architecture](docs/ARCHITECTURE.md), [patterns](docs/PATTERNS.md), [testing](docs/TESTING.md), [new-package checklist](docs/NEW_PACKAGE_CHECKLIST.md), [package README template](docs/README_TEMPLATE.md), [development plan](docs/DEVELOPMENT_PLAN.md) |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the git conventions, PR process and Definition of Done, and
[AGENTS.md](AGENTS.md) for the code rules. Both apply to humans and AI agents alike.

## Security

Report vulnerabilities privately, never in a public issue. [SECURITY.md](SECURITY.md) has how to report and the
rules for using the security packages safely.
