# New package checklist

Follow these steps in order when adding a sub-package to the SDK. The rules referenced here are defined in [../AGENTS.md](../AGENTS.md); templates are in [PATTERNS.md](PATTERNS.md).

## 1. Decide it belongs here

- Is it a **cross-cutting concern** reusable by multiple consumers? If it's app-specific, it belongs in the consumer, not the SDK.
- Can it stand alone without importing sibling SDK packages? Keep it [independent](ARCHITECTURE.md#modular-no-entry-point). If it needs shared types, prefer a small leaf package (e.g. `lib/common/dto`).

## 2. Create the package

```
lib/<package>/
  doc.go            # package-level doc comment (or top of the main file)
  <package>.go      # implementation
  <package>_test.go # table-driven tests
  README.md         # what it does + usage
```

- File names are `lower_with_underscores.go`.
- Write a **package doc comment** describing the package's purpose.

## 3. Design the API

- **Interface before implementation** where it aids testing — define the contract, return it from the constructor (see `repository.Repository`, `redis.Client`).
- Constructor named **`NewX`**. **Configuration goes in an exported `mapstructure`-tagged `Config` struct** (with `DefaultConfig()` and `Validate()`), so consumers can embed it and load from YAML via `config.Load` — see the [Configuration convention](../AGENTS.md#configuration). Reserve **functional `WithX` options** for optional deps / non-serializable behavior (loggers, clients, callbacks); required deps are positional.
- Exported signatures use **stdlib types** (`context.Context`, `http.Handler`, `*sql.DB`) — don't leak third-party types unless the package's purpose is wrapping one.
- I/O functions take **`context.Context` first**.

## 4. Errors & logging

- Return **`*errorz.Error`** with an appropriate code, wrapping the underlying cause to preserve the chain. `errorz` is the ecosystem-wide error type and is a permitted foundational dependency. See [ARCHITECTURE.md](ARCHITECTURE.md#errors-errorz-as-the-ecosystem-wide-error-type) and the [error patterns](PATTERNS.md#sentinel-errors-errorz).
- Add a `logger.Logger` parameter **only** if the package has internal state worth tracing, and keep it **optional and nil-safe** ([template](PATTERNS.md#optional-nil-safe-logger)). Never use stdlib `log`.

## 5. Tests

- **Table-driven** unit tests ([template](PATTERNS.md#table-driven-tests)).
- Guard slow/integration-only paths with `testing.Short()`; put live-service tests in **`*_integration_test.go`**.

### Mocks (if the package exports an interface consumers depend on)

Consumers unit-test against SDK interfaces. For each new **exported interface** (e.g. `auth.Validator`,
`tracer.Tracer`, `metrics.Recorder`), add a mock so services don't each re-generate one. See
[TESTING.md](TESTING.md) for the full strategy and the fakes-vs-mocks split.

- Add a **`//go:generate`** directive immediately above the interface (blank line before its doc comment), writing
  into the interface's own subpackage in the separate `mocks` module. Use a **distinct package name** (`mock<pkg>`)
  so mock type names never collide across packages and importing one mock doesn't pull in another's deps:

  ```go
  //go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/<pkg>/mock_<pkg>.go -package=mock<pkg> github.com/biairmal/go-sdk/lib/<pkg> Iface1,Iface2
  ```

- Run **`make mocks`** from the repo root (regenerates via `go generate` + tidies the `mocks` module). Commit the
  generated files with the interface change.
- Prefer a **hand-written fake / `NoOp`** in the package itself (like `logger.NewNoOp()`) when a working stand-in is
  more useful to consumers than call verification — ship it *and* the mock.

## 6. Documentation

- Write the package **`README.md`** following [README_TEMPLATE.md](README_TEMPLATE.md) (copy the skeleton at the bottom of that file).
- **Add a row to the [package map](../AGENTS.md#package-map)** in `AGENTS.md`.
- Add a quick link in the root [README.md](../README.md) documentation table if it's a top-level package.

## 7. Verify

```bash
make check          # MUST pass: format → lint-fix → test-unit → coverage → vulncheck → deps-verify
```

Then walk the [Definition of Done](../AGENTS.md#definition-of-done).
