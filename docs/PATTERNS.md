# Patterns

Copy-paste templates for the conventions required by [../AGENTS.md](../AGENTS.md). Each snippet is modelled on real code in this repository — follow these shapes rather than inventing new ones.

## Constructors & functional options

Constructors are named `NewX`, take `context`-free configuration, and accept variadic `WithX` options that mutate the receiver. Options must be nil/zero-safe.

Modelled on [`repository/sql/sql_repository.go`](../repository/sql/sql_repository.go):

```go
type SQLRepositoryOption[TEntity any, TID comparable] func(*SQLRepository[TEntity, TID])

func NewSQLRepository[TEntity any, TID comparable](
    log logger.Logger,            // optional, nil-safe
    db *sqlkit.DB,
    tableName string,
    opts ...SQLRepositoryOption[TEntity, TID],
) repository.Repository[TEntity, TID] {   // return the interface, not the concrete type
    repo := &SQLRepository[TEntity, TID]{ /* defaults */ }
    for _, opt := range opts {
        opt(repo)
    }
    return repo
}

// WithDialect sets the SQL dialect for placeholders and pagination.
func WithDialect[TEntity any, TID comparable](d Dialect) SQLRepositoryOption[TEntity, TID] {
    return func(r *SQLRepository[TEntity, TID]) {
        if d != nil {            // nil-safe: ignore zero values
            r.dialect = d
        }
    }
}
```

## Sentinel errors (`errorz`)

Sentinels are package-level values prefixed `Err…`; constructors return a fresh `*Error` that wraps the sentinel so callers can use `errors.Is`. `errorz` is the [ecosystem-wide error type](ARCHITECTURE.md#errors-errorz-as-the-ecosystem-wide-error-type) — use it across all layers and SDK packages, not just the HTTP edge.

Modelled on [`errorz/error.go`](../errorz/error.go):

```go
// Sentinel — used for comparison, never type-asserted directly.
var ErrNotFound = sentinelError{code: CodeNotFound, msg: "not found"}

type sentinelError struct{ code, msg string }

func (e sentinelError) Error() string { return e.msg }

// Constructor — returns a new *Error wrapping the sentinel.
func NotFound() *Error {
    return &Error{Code: CodeNotFound, Message: "not found", Err: ErrNotFound, SourceSystem: DefaultSourceSystem}
}
```

Produce and consume it from any layer. Two idioms, both valid:

```go
row := conn.QueryRowContext(ctx, query, id)
if err := row.Scan(&u.ID, &u.Name); err != nil {
    // 1. Sentinel-carrying constructor — keeps the ErrNotFound sentinel + CodeNotFound,
    //    so errors.Is(.., errorz.ErrNotFound) works and httpkit maps it to 404.
    if errors.Is(err, sql.ErrNoRows) {
        return nil, errorz.NotFound().WithMessage("user not found")
    }
    // 2. Wrap the real cause and attach a code — preserves the underlying chain.
    return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("query user")
}

// Callers compare by sentinel, regardless of which layer produced the error.
if errors.Is(err, errorz.ErrNotFound) {                 // ✅ sentinel comparison
    // ...
}
```

Use the **constructor** (`NotFound()`, `BadRequest()`, …) when you want a known code + sentinel; use **`Wrap(err)`** when you're carrying a deeper cause and adding a code with `WithCode`. `httpkit` maps the code to an HTTP status at the edge — that mapping is the *only* HTTP-specific step; the error itself is free to originate deep in the stack.

## Table-driven tests

Same-package, `[]struct{ name … }` iterated with `t.Run`. Modelled on [`errorz/error__test.go`](../errorz/error__test.go):

```go
func TestNew(t *testing.T) {
    tests := []struct {
        name        string
        message     string
        wantMessage string
        wantCode    string
    }{
        {name: "creates error with message", message: "test error", wantMessage: "test error"},
        {name: "creates error with empty message", message: "", wantMessage: ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := New(tt.message)
            if got.Message != tt.wantMessage {
                t.Errorf("New().Message = %v, want %v", got.Message, tt.wantMessage)
            }
        })
    }
}
```

Integration tests that need a live service go in `*_integration_test.go` and skip under `-short`:

```go
func TestClient_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("integration test: requires a live Redis")
    }
    // ... talk to the real service
}
```

## Context & transactions

I/O functions take `context.Context` first. Transactions ride on the context so callers can compose them without changing signatures.

Modelled on [`sqlkit/transaction.go`](../sqlkit/transaction.go) and [`sqlkit/db.go`](../sqlkit/db.go):

```go
// Inject a transaction for the duration of a request/unit of work.
ctx = sqlkit.InjectTx(ctx, tx)

// Inside the data layer, transparently use it if present.
if tx, ok := sqlkit.ExtractTx(ctx); ok {
    return tx.ExecContext(ctx, query, args...)
}
return db.Leader().ExecContext(ctx, query, args...)   // writes → Leader
// reads → db.Follower() (round-robin, health fallback to Leader)
```

## Optional, nil-safe logger

Accept `logger.Logger` only when there's real internal state to trace, and guard every call. Modelled on [`repository/sql/sql_repository.go`](../repository/sql/sql_repository.go):

```go
func (r *SQLRepository[TEntity, TID]) logQuery(ctx context.Context, query string, args []any) {
    if r.log == nil {            // nil = silent, never panic
        return
    }
    r.log.DebugfWithContext(ctx, "query: %s args: %v", query, args)
}
```
