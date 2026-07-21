# Mocks Module

Generated [gomock](https://github.com/uber-go/mock) mocks for the `go-sdk` interfaces, shipped as a **separate Go
module** (`github.com/biairmal/go-sdk/mocks`) so that `go.uber.org/mock` never becomes a dependency of the main SDK
module. Consuming microservices add it as a **test dependency** when they need call/argument verification.

## Overview

The mocks live in their own module (own `go.mod`, `replace github.com/biairmal/go-sdk => ../`) for one reason:
keeping the main SDK dependency-light. A mock in a normal SDK subpackage would make gomock a direct `require` of
`go-sdk`, dragging it into every consumer's build. Isolating it here avoids that while still versioning the mocks
alongside the interfaces they mock.

Mocks are **generated, not hand-written** — regenerate them whenever a mocked interface changes (`make mocks` from
the repo root). For dependencies where a working stand-in beats call verification, prefer the SDK's fakes instead
(see [docs/TESTING.md](../docs/TESTING.md)).

## Layout

One subpackage per source package (mirroring the SDK), each with its own package name distinct from the real one.
This keeps mock type names from colliding across packages (e.g. a future `auth.Validator` and `validator.Validator`
would both be `MockValidator` — separate packages keep them apart) and means importing one mock never drags in
another package's dependencies (importing `mocks/logger` does **not** pull in `go-redis`).

| Import path | Package | Constructors |
|---|---|---|
| `github.com/biairmal/go-sdk/mocks/logger` | `mocklogger` | `NewMockLogger(ctrl)` |
| `github.com/biairmal/go-sdk/mocks/redis` | `mockredis` | `NewMockClient(ctrl)`, `NewMockPipeliner(ctrl)` |
| `github.com/biairmal/go-sdk/mocks/repository` | `mockrepository` | `NewMockRepository[E,ID](ctrl)`, `NewMockReadRepository[E,ID]`, `NewMockWriteRepository[E,ID]`, `NewMockTransactionalRepository[E,ID]` |

## Usage

### Add as a test dependency

```bash
go get -t github.com/biairmal/go-sdk/mocks/repository   # or /logger, /redis — import only what you need
```

### Use in a test

```go
package usecase_test

import (
    "context"
    "testing"

    mockrepository "github.com/biairmal/go-sdk/mocks/repository"
    "go.uber.org/mock/gomock"
)

func TestCreateUser(t *testing.T) {
    ctrl := gomock.NewController(t) // auto-verifies expectations at test end (Go 1.14+)

    repo := mockrepository.NewMockRepository[User, string](ctrl)
    repo.EXPECT().
        Create(gomock.Any(), gomock.Any()).
        Return(nil).
        Times(1)

    svc := NewUserService(repo)
    if err := svc.CreateUser(context.Background(), "alice"); err != nil {
        t.Fatalf("CreateUser: %v", err)
    }
}
```

## Regenerating

The `//go:generate` directives live next to each interface in the main module (`lib/logger/logger.go`,
`lib/redis/client.go`, `lib/repository/repository.go`). Regenerate all mocks and re-tidy this module in one step from
the repo root:

```bash
make mocks
```

This runs `go generate` on the source packages (each directive writes into `./mocks/`) and then `go mod tidy` here.
Commit the regenerated files alongside the interface change.

## Limitations

- **Prefer fakes for the common case.** Mocks are for asserting *how* a dependency was called. When a test just
  needs a working dependency, use the SDK fakes (`logger.NewNoOp()`, etc.) or `miniredis` / `go-sqlmock` — see
  [docs/TESTING.md](../docs/TESTING.md).
- **Regenerate on interface changes.** These files are generated; edits are overwritten by `make mocks`.

## Dependencies

- [go.uber.org/mock](https://github.com/uber-go/mock) – the maintained gomock fork (mockgen + gomock runtime).

## See also

- [docs/TESTING.md](../docs/TESTING.md) – the full testing strategy (fakes vs mocks, miniredis, go-sqlmock).
- [repository](../repository/README.md), [redis](../redis), [logger](../logger/README.md) – the mocked interfaces.
