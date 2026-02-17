# Repository Package

Generic, interface-based repository patterns for database operations with a concrete SQL implementation. Uses Go generics for type-safe CRUD, filtering, pagination, and sorting.

## Overview

The repository package defines generic interfaces (`Repository[TEntity, TID]`, `ReadRepository`, `WriteRepository`, `TransactionalRepository`) and provides a reflection-based SQL implementation in the `sql` subpackage. The SQL implementation uses struct tags (`db`) for column mapping, integrates with [sqlkit](https://pkg.go.dev/github.com/biairmal/go-sdk/sqlkit) for leader/follower and context-injected transactions, and supports multiple dialects (Postgres, MySQL, Oracle).

## Features

- **Type-safe generic interfaces** using `Repository[TEntity, TID]` with comparable ID types
- **SQL repository implementation** (`repository/sql`) with reflection and `db` struct tags; leader/follower and transaction support via sqlkit
- **Flexible filtering** via `Filter` with conditions (eq, ne, gt, gte, lt, lte, like, in, is_null, is_not_null)
- **Pagination and sorting** via `ListOptions` (offset/limit, multiple sorts)
- **Optional total count** via `SkipCount` in `ListOptions`
- **Extensible design** for custom repository implementations and additional dialects

## Package Structure

```
repository/
├── repository.go   # Core interfaces (Repository, ReadRepository, WriteRepository, TransactionalRepository)
├── options.go      # ListOptions, Filter, FilterCondition, Pagination, Sort
├── errors.go       # ErrNotFound, ErrAlreadyExists, etc.; IsNotFound, IsAlreadyExists, IsConflict
├── sql/
│   ├── sql_repository.go  # SQLRepository, NewSQLRepository, options
│   ├── base.go            # BaseRepository, GetConnection, GetReadConnection
│   ├── crud.go            # Reflection helpers (INSERT/UPDATE build, ID handling)
│   ├── dialect.go         # Dialect interface; Postgres, MySQL, Oracle
│   ├── helpers.go         # BuildWhereClause, BuildOrderByClause, BuildPaginationClause, SanitizeColumnName, ConvertSQLError
│   └── scan.go            # ScanRow[T], NullTime
└── cache/
    ├── decorator.go
    ├── strategy.go
    └── key.go
```

---

## Core Interfaces

### Repository[TEntity, TID]

Generic repository interface for full CRUD and list operations. `TID` must be comparable (e.g. `int64`, `string`, `uuid.UUID`).

```go
type Repository[TEntity any, TID comparable] interface {
    Create(ctx context.Context, entity *TEntity) error
    GetByID(ctx context.Context, id TID) (*TEntity, error)
    Update(ctx context.Context, id TID, entity *TEntity) error
    Delete(ctx context.Context, id TID) error
    List(ctx context.Context, opts *ListOptions) ([]*TEntity, int64, error)  // items, total count, error
    Count(ctx context.Context, filter Filter) (int64, error)
    Exists(ctx context.Context, id TID) (bool, error)
}
```

- **List** returns `(items, total, error)`. The total is the number of entities matching the filter (excluding pagination). Use `ListOptions.SkipCount` to skip the count query when not needed.
- **GetByID** returns `repository.ErrNotFound` when the entity does not exist.
- **Update** and **Delete** return `repository.ErrNotFound` when no rows are affected.

### ReadRepository[TEntity, TID]

Read-only subset: `GetByID`, `List`, `Count`, `Exists`. Use for read-only services or follower-only access.

### WriteRepository[TEntity, TID]

Write-only subset: `Create`, `Update`, `Delete`. Use for command-side or write-only services.

### TransactionalRepository[TEntity, TID]

Extends `Repository` with `WithTx(tx *sql.Tx) Repository[TEntity, TID]` for binding to an existing transaction. The SQL implementation in this package uses context-based transaction injection (sqlkit) instead of `WithTx`; see [repository/sql](#repository-sql-package) below.

---

## Options

### ListOptions

```go
type ListOptions struct {
    Pagination Pagination  // Limit, Offset, Cursor
    Filter     Filter     // Filtering criteria (conditions combined with AND)
    Sorts      []Sort     // Sort by multiple columns (order preserved)
    SkipCount  bool       // If true, List does not run count query; total is 0
}
```

Pass a non-nil `*ListOptions`. For no filtering, sorting, or pagination use `&repository.ListOptions{}`. Set `SkipCount: true` when the total count is not needed to avoid the extra `COUNT` query.

### Filter and FilterCondition

```go
type Filter struct {
    Conditions []FilterCondition
}

type FilterCondition struct {
    Field    string         // Column name
    Operator FilterOperator // Operator
    Value    any            // Value for single-value operators
    Values   []any          // For operator "in"
}
```

**FilterOperator constants:** `FilterOperatorEq`, `FilterOperatorNe`, `FilterOperatorGt`, `FilterOperatorGte`, `FilterOperatorLt`, `FilterOperatorLte`, `FilterOperatorLike`, `FilterOperatorIn`, `FilterOperatorIsNull`, `FilterOperatorIsNotNull`.

All conditions in `Conditions` are combined with **AND**. For `in`, use `Values`; for others use `Value`.

### Pagination

```go
type Pagination struct {
    Limit  int    // Page size (defaulted in SQL impl: 20 if <= 0, capped at 100)
    Offset int    // Offset (defaulted to 0 if < 0)
    Cursor string // Reserved for cursor-based pagination
}
```

### Sort

```go
type Sort struct {
    Field     string       // Column name
    Direction SortDirection // "ASC" or "DESC"
}

const SortAsc, SortDesc SortDirection = "ASC", "DESC"
```

---

## Error Handling

Standard errors and helpers:

| Variable / Function   | Use |
|----------------------|-----|
| `repository.ErrNotFound`      | Entity not found (GetByID, Update, Delete) |
| `repository.ErrAlreadyExists`  | Entity already exists |
| `repository.ErrInvalidID`      | Invalid ID format |
| `repository.ErrInvalidEntity`  | Entity validation failed |
| `repository.ErrConflict`       | Update conflict |
| `repository.ErrConnection`     | Database connection error |
| `repository.IsNotFound(err)`  | Returns true if err is ErrNotFound |
| `repository.IsAlreadyExists(err)` | Returns true if err is ErrAlreadyExists |
| `repository.IsConflict(err)`  | Returns true if err is ErrConflict |

---

## Repository SQL Package

The `github.com/biairmal/go-sdk/repository/sql` package provides a generic CRUD implementation that satisfies `repository.Repository[TEntity, TID]`. It uses reflection and the `db` struct tag for column mapping, and integrates with `*sqlkit.DB` for leader/follower and context-injected transactions.

### Dependencies

- **sqlkit**: For `*sqlkit.DB`, `Leader()`, `Follower()`, and context transaction injection (`sqlkit.ExtractTx`, `InjectTx`).
- **logger** (optional): `github.com/biairmal/go-sdk/logger` for optional query logging; pass `nil` to disable.

### Entity Requirements

- `TEntity` must be a **struct type** (enforced at construction; panics otherwise).
- Exported fields that map to columns must use the **`db` struct tag** with the column name (e.g. `db:"id"`, `db:"created_at"`). Use `db:"-"` to omit a field.
- Column names in tags are matched **case-insensitively** when scanning and when resolving the ID column.
- Supported field types for scanning include: common primitives, `time.Time`, `*time.Time`, `uuid.UUID`, `*uuid.UUID`. For nullable time, the package provides `sql.NullTime` (Time + Valid) implementing `sql.Scanner`.

### Creating a SQL Repository

```go
import (
    "github.com/biairmal/go-sdk/logger"
    "github.com/biairmal/go-sdk/repository"
    "github.com/biairmal/go-sdk/repository/sql"
    "github.com/biairmal/go-sdk/sqlkit"
)

type User struct {
    ID        int64     `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at"`
}

repo := sql.NewSQLRepository[User, int64](
    log,        // logger.Logger; may be nil to disable query logging
    db,         // *sqlkit.DB
    "users",    // table name
    sql.WithDialect(sql.Postgres{}),
    sql.WithSelectColumns[User, int64]([]string{"id", "name", "email"}),
    sql.WithIDColumn[User, int64]("id"), // optional; default is "id"
)
// repo implements repository.Repository[User, int64]
```

### SQL Repository Options

| Option | Description |
|--------|--------------|
| `sql.WithDialect(d Dialect)` | SQL dialect for placeholders and pagination. Built-in: `sql.Postgres{}`, `sql.MySQL{}`, `sql.Oracle{}`. Default: Postgres. |
| `sql.WithSelectColumns[TEntity, TID](columns []string)` | Columns to SELECT in GetByID and List. If empty, `*` is used. |
| `sql.WithIDColumn[TEntity, TID](column string)` | Name of the ID column; default `"id"`. |

### Read vs Write Connection

- **BaseRepository** (embedded in SQLRepository) provides:
  - **GetConnection(ctx)** – for write operations (Create, Update, Delete). If a transaction is present in the context (`sqlkit.ExtractTx(ctx)`), that transaction is used; otherwise `db.Leader()`.
  - **GetReadConnection(ctx)** – for read operations (GetByID, List, Count, Exists). If a transaction is present, that transaction is used; otherwise `db.Follower()`.

So when the service runs code inside `sqlkit.WithTransaction(ctx, fn)`, the same context is passed to the repository; the repository then uses the injected transaction for both reads and writes within that transaction.

### Create Behaviour

- If the entity’s ID field (matching the configured ID column) is **zero** (e.g. 0, nil, `uuid.Nil`, empty string):
  - The ID column is **omitted** from the `INSERT` so the database can set it (e.g. `DEFAULT`, `SERIAL`, `AUTO_INCREMENT`).
  - The generated ID is **written back** to the entity:
    - For **int64** (or `*int64`) ID: uses `Result.LastInsertId()` when the dialect supports it (e.g. MySQL); otherwise the implementation uses `RETURNING id` (e.g. Postgres) and scans into the entity.
    - For **uuid.UUID**, **string**, or other types: the implementation uses `INSERT ... RETURNING <id_column>` and scans the returned value into the entity.
- If the entity’s ID is **non-zero**, the row is inserted with that ID (no write-back).

### GetByID, List, Count, Exists

- Use the **read** connection (follower when not in a transaction).
- **GetByID**: returns `repository.ErrNotFound` when no row is found.
- **List**: returns `(items, total, error)`. If `opts.SkipCount` is true, the count query is skipped and `total` is 0. Filter, sort, and pagination are applied as in [Options](#options). Defaults: `Pagination.Limit` 20 if ≤ 0, max 100; `Offset` ≥ 0.
- **Count**: returns the number of rows matching the filter.
- **Exists**: returns whether a row with the given ID exists.

### Update and Delete

- Use the **write** connection (leader or transaction).
- Return **repository.ErrNotFound** when `RowsAffected() == 0`.
- **Update**: all struct fields with `db` tags (except the ID column) are included in `SET`. The ID column is only used in the `WHERE` clause.

### Dialects

The `Dialect` interface provides:

- **Placeholder(index int) string** – e.g. Postgres `$1`, `$2`; MySQL `?`; Oracle `:1`, `:2`.
- **PaginationClause(limitArgIndex, offsetArgIndex int) string** – e.g. `LIMIT $1 OFFSET $2` (Postgres), `LIMIT ? OFFSET ?` (MySQL), `OFFSET :2 ROWS FETCH NEXT :1 ROWS ONLY` (Oracle 12c+).

Built-in types: `sql.Postgres{}`, `sql.MySQL{}`, `sql.Oracle{}`. Default dialect is Postgres.

### Filter Operators (SQL)

The SQL implementation builds `WHERE` clauses from `repository.Filter`. Column names in conditions are sanitised (unsafe characters rejected). Supported operators:

- **eq**, **ne**, **gt**, **gte**, **lt**, **lte**, **like** – use `Value`.
- **in** – use `Values` (slice).
- **is_null**, **is_not_null** – no value.

Conditions are combined with AND. Only these operator strings are accepted; others are ignored.

### Scanning Rows

- **ScanRow[T](rows *sql.Rows) (*T, error)** – maps one row into `*T` using the `db` tag. Call after `rows.Next()`. Supports primitives, `time.Time`, `*time.Time`, `uuid.UUID`, `*uuid.UUID`. Column names are matched case-insensitively.
- **NullTime** – struct with `Time` and `Valid`; implements `sql.Scanner` for nullable time columns.

### Error Conversion

**ConvertSQLError(err error)** – maps `sql.ErrNoRows` to `repository.ErrNotFound`; other errors are returned as-is. Used internally; SQL-specific errors (e.g. duplicate key) are not yet mapped to `ErrAlreadyExists`.

### Transactions

Use **sqlkit** in the service layer; do not pass a nil context. Example:

```go
err := db.WithTransaction(ctx, func(txCtx context.Context) error {
    if err := userRepo.Create(txCtx, user); err != nil {
        return err
    }
    return walletRepo.Create(txCtx, wallet)
})
```

Repositories that use `GetConnection` / `GetReadConnection` will see the transaction in the context and use it for the whole callback.

---

## Quick Start

### Basic usage with SQL repository

```go
package main

import (
    "context"
    "github.com/biairmal/go-sdk/logger"
    "github.com/biairmal/go-sdk/repository"
    "github.com/biairmal/go-sdk/repository/sql"
    "github.com/biairmal/go-sdk/sqlkit"
)

type User struct {
    ID        int64     `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at"`
}

func main() {
    ctx := context.Background()
    // db := sqlkit.New(ctx, &cfg) ...

    repo := sql.NewSQLRepository[User, int64](nil, db, "users", sql.WithDialect(sql.Postgres{}))

    user := &User{Name: "Jane", Email: "jane@example.com"}
    if err := repo.Create(ctx, user); err != nil {
        panic(err)
    }
    // user.ID is set if DB generated it

    u, err := repo.GetByID(ctx, user.ID)
    if err != nil {
        if repository.IsNotFound(err) {
            // handle not found
        }
        panic(err)
    }
}
```

### List with filter and pagination

```go
opts := &repository.ListOptions{
    Filter: repository.Filter{
        Conditions: []repository.FilterCondition{
            {Field: "status", Operator: repository.FilterOperatorEq, Value: "active"},
        },
    },
    Sorts: []repository.Sort{
        {Field: "created_at", Direction: repository.SortDesc},
    },
    Pagination: repository.Pagination{Limit: 20, Offset: 0},
    SkipCount:  false,
}
users, total, err := repo.List(ctx, opts)
```

### With transactions (sqlkit)

```go
err := db.WithTransaction(ctx, func(txCtx context.Context) error {
    user := &User{Name: req.Name, Email: req.Email}
    if err := userRepo.Create(txCtx, user); err != nil {
        return err
    }
    wallet := &Wallet{UserID: user.ID, Balance: 0}
    return walletRepo.Create(txCtx, wallet)
})
```

---

## Security Considerations

1. **Parameterised queries**: The SQL implementation uses placeholders only; filter values are passed as arguments. Do not interpolate user input into table or column names.
2. **Column names**: Filter/sort column names are sanitised (dangerous characters rejected). Prefer whitelisting column names in higher layers where possible.
3. **Authorization**: Repositories perform data access only; enforce authorization in the service or API layer.

## Limitations

- **SQL repository**: Requires struct entities with `db` tags; no query builder or raw SQL API. Complex queries need custom repositories or other tools (e.g. sqlc).
- **List opts**: Pass a non-nil `*ListOptions`; use `&repository.ListOptions{}` for no filter/sort/pagination.
- **Error mapping**: Duplicate key and other DB-specific errors are not yet mapped to `repository.ErrAlreadyExists` or `ErrConflict`.
- **Cache**: The `cache` subpackage is present but not covered here; caching decorators may require additional dependencies.

## See Also

- [SQLKit Package](../sqlkit/README.md) – Database connection management, leader/follower, and transaction injection.
