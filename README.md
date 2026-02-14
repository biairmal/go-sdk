# go-sdk

> 🚧 **Under development** — This package is under active development. APIs and behaviour may change.

Shared Go SDK for the Guest Management ecosystem. Provides configuration loading, structured errors, HTTP utilities (handler adapter, middleware, response envelope, health/readiness), logging, generic repository patterns, and SQL connection management. Consumed by applications such as [guest-management-be](../guest-management-be); all public APIs use standard library types where possible (e.g. `http.Handler`, `*sql.DB`) for easy integration with any router or driver.

---

## Overview

The SDK is a collection of libraries that handle cross-cutting concerns: config (Viper + .env + substitution), errors with codes and metadata ([errorz](errorz/README.md)), HTTP middleware and response envelope ([httpkit](httpkit/README.md)), structured logging ([logger](logger/README.md)), generic repository and SQL helpers ([repository](repository/README.md), [sqlkit](sqlkit/README.md)). Use the sub-packages you need; there is no requirement to use all of them.

---

## Features

- **Configuration** — Load JSON/YAML into structs with Viper; optional `.env` loading and `${VAR}` substitution in config files. See [config/README.md](config/README.md).
- **Structured errors** — Error type with codes, source system, metadata, and sentinels; maps to HTTP status in httpkit. See [errorz/README.md](errorz/README.md).
- **HTTP utilities** — Handler adapter (`func(*http.Request) (any, error)` → `http.Handler`), Recover/RequestID/Logging middleware, response envelope, health and readiness handlers, thin client. See [httpkit/README.md](httpkit/README.md).
- **Logging** — Unified logger interface; Zerolog backend and no-op for tests; levels, structured fields, context extraction, file rotation. See [logger/README.md](logger/README.md).
- **Repository** — Generic repository interfaces and SQL implementation; filtering, pagination, sorting; optional caching; mock for tests. See [repository/README.md](repository/README.md).
- **SQL connection** — Leader/follower support, health checks, retry, transaction injection; driver-agnostic over `database/sql`. See [sqlkit/README.md](sqlkit/README.md).

---

## Prerequisites

Before developing or depending on this package:

| Requirement   | Purpose |
| -------------- | ------- |
| **Go 1.25.1+** | Build and test. Check with `go version`. |
| **Make**       | Run format, lint, test, coverage, and tool installation. On Windows, use Git Bash, WSL, or [GnuWin32 Make](http://gnuwin32.sourceforge.net/packages/make.htm). |

Optional (installed via `make install-tools` when needed):

- **gofumpt** — Code formatter.
- **golangci-lint** — Linter.
- **govulncheck** — Vulnerability check for dependencies.

---

## How to develop

### First-time setup

Install development tools (formatter, linter, vulnerability checker):

```bash
make install-tools
```

### Running checks (CI)

Run all checks before committing. Stops at the first failure (fail-fast):

```bash
make check
```

Or:

```bash
make ci
```

This runs, in order: format, lint-fix, test-unit, coverage, vulncheck, deps-verify.

### Unit vs integration tests

- **Unit tests:** `make test` or `make test-unit` runs `go test -short ./...`. Use `testing.Short()` in tests to skip slow or integration-only code when running in short mode.
- **Integration tests:** `make test-integration` runs `go test ./...` (no `-short`). Alternatively, use a build tag (e.g. `//go:build integration`) and `go test -tags=integration ./...`; document the chosen convention in test files or this README.
- **Coverage:** `make coverage` writes `out/coverage.out` and `out/coverage.html`. Use `make coverage-view` to open the report (platform-dependent).

### Other targets

Run `make help` to see all targets and descriptions (formatter, linter, test, security, deps, build).

---

## Documentation references

| Document | Description |
| -------- | ----------- |
| [config/README.md](config/README.md) | Config loader: Viper, .env, substitution, usage. |
| [errorz/README.md](errorz/README.md) | Structured errors, codes, sentinels, limitations. |
| [httpkit/README.md](httpkit/README.md) | Handler, middleware, response envelope, health/readiness, client. |
| [logger/README.md](logger/README.md) | Logger interface, Zerolog backend, no-op, rotation. |
| [repository/README.md](repository/README.md) | Repository interfaces, SQL implementation, mock, options. |
| [sqlkit/README.md](sqlkit/README.md) | DB connection, leader/follower, health, transactions. |
