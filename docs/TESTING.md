# Testing with go-sdk

How a microservice that consumes `go-sdk` unit-tests its own code against the SDK's seams. The short version:
**reach for a fake first; use a mock only when the test must assert *how* a dependency was called.**

## Two things get "mocked" — keep them separate

1. **The SDK's interfaces** (`logger.Logger`, `redis.Client`, `repository.Repository[T,ID]`, and the planned
   `auth.Validator` / `tracer.Tracer` / `metrics.Recorder`). Your service swaps these out when testing code that
   depends on them. This doc is about these.
2. **Your service's own interfaces** (its repositories, its services). Those are yours to mock — the SDK has nothing
   to do with them. The same tools below apply.

## Fakes first, mocks second

A **fake** is a real (usually in-memory) implementation you can hand to code under test. A **mock** records and
verifies calls. Fakes read cleaner, need no framework, and handle generics gracefully — so they're the default.
Reach for a mock only when the assertion *is* "was `Set` called once with key `X`".

| SDK dependency | Default in tests | Why |
|---|---|---|
| `logger.Logger` | `logger.NewNoOp()` (built-in fake) | You almost never assert on logs |
| `tracer.Tracer` / `metrics.Recorder` | `NewNoOp()` (planned built-in fakes) | Spans/metrics rarely asserted in unit tests |
| `auth.Validator` | `auth.ValidatorFunc(func(...){...})` (planned) | A one-line inline fake |
| `redis.Client` | [`miniredis`](https://github.com/alicebob/miniredis) | A real-ish in-memory Redis; behaves like the real thing |
| `sqlkit.DB` / `*sql.DB` | [`go-sqlmock`](https://github.com/DATA-DOG/go-sqlmock), or sqlite / testcontainers | It wraps a concrete `*sql.DB` — mock at the driver, or use a real DB |
| `repository.Repository[T,ID]` | in-memory fake, **or** `mocks.NewMockRepository[T,ID]` for call verification | Generic; a map-backed fake is usually nicer than a mock |

When you *do* need call verification, use the generated mocks in the separate
[`github.com/biairmal/go-sdk/mocks`](../mocks/README.md) module.

## Tooling

- **[go.uber.org/mock](https://github.com/uber-go/mock)** (`mockgen` + `gomock`) — the maintained gomock fork. Use
  this, **not** the archived `github.com/golang/mock`.
- **[alicebob/miniredis](https://github.com/alicebob/miniredis)** — in-memory Redis for `redis.Client` tests.
- **[DATA-DOG/go-sqlmock](https://github.com/DATA-DOG/go-sqlmock)** — mock the `database/sql` driver behind
  `sqlkit`.

## Using the generated SDK mocks

The mocks live in a **separate module**, one subpackage per source package, so gomock never leaks into the main
SDK's (or your production) dependency graph and importing one mock never drags in another package's deps. Add only
the subpackage you need as a test dependency:

```bash
go get -t github.com/biairmal/go-sdk/mocks/repository   # or /logger, /redis
```

```go
import (
    mockrepository "github.com/biairmal/go-sdk/mocks/repository"
    "go.uber.org/mock/gomock"
)

func TestPlaceOrder(t *testing.T) {
    ctrl := gomock.NewController(t) // auto-verifies at test end

    repo := mockrepository.NewMockRepository[Order, string](ctrl)
    repo.EXPECT().GetByID(gomock.Any(), "ord-1").Return(&Order{ID: "ord-1"}, nil)

    svc := NewOrderService(repo)
    // ... exercise svc, assert behaviour
}
```

| Import path (package) | Constructors |
|---|---|
| `mocks/logger` (`mocklogger`) | `NewMockLogger` |
| `mocks/redis` (`mockredis`) | `NewMockClient`, `NewMockPipeliner` |
| `mocks/repository` (`mockrepository`) | `NewMockRepository[E,ID]`, `NewMockReadRepository[E,ID]`, `NewMockWriteRepository[E,ID]`, `NewMockTransactionalRepository[E,ID]` |

## Example: `redis.Client` with miniredis (fake, preferred)

```go
func TestCache(t *testing.T) {
    mr, err := miniredis.Run()
    if err != nil { t.Fatal(err) }
    defer mr.Close()

    client, err := redis.NewClient(&redis.Config{Mode: redis.ModeStandalone, Address: mr.Addr()})
    if err != nil { t.Fatal(err) }

    // client talks to the in-memory server — real behaviour, no mocking framework.
}
```

## Example: `sqlkit` with go-sqlmock

```go
func TestRepo(t *testing.T) {
    db, sqlMock, err := sqlmock.New()
    if err != nil { t.Fatal(err) }
    defer db.Close()

    sqlMock.ExpectQuery("SELECT .* FROM users").
        WithArgs("u-1").
        WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("u-1", "alice"))

    // wrap db with your repository and assert queries.
}
```

## Regenerating the SDK mocks

The `//go:generate` directives live next to each interface in the main module. From the repo root:

```bash
make mocks   # go generate on logger/redis/repository → writes into ./mocks → go mod tidy
```

Commit regenerated files with the interface change that prompted them.

## See also

- [mocks/README.md](../mocks/README.md) – the mocks module: layout, constructors, regeneration.
- [AGENTS.md](../AGENTS.md) – authoring rules and the Definition of Done.
