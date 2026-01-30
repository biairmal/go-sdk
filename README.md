# go-sdk

Go SDK for Guest Management.

## Development

### Prerequisites

- Go 1.25 or later
- Make

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

Or equivalently:

```bash
make ci
```

This runs, in order: format-check, lint, unit tests, coverage, vulncheck, deps-verify.

### Unit vs integration tests

- **Unit tests:** `make test` or `make test-unit` runs `go test -short ./...`. Use `testing.Short()` in tests to skip slow or integration-only code when running in short mode.
- **Integration tests:** `make test-integration` runs `go test ./...` (no `-short`). Alternatively, use a build tag (e.g. `//go:build integration`) and `go test -tags=integration ./...`; document the chosen convention in test files or this README.
- **Coverage:** `make coverage` writes `out/coverage.out` and `out/coverage.html`. Use `make coverage-view` to open the report (platform-dependent).

### Other targets

Run `make help` to see all targets and descriptions (formatter, linter, test, security, deps, build).
