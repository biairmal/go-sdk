# Tracesql Package

`tracesql` wraps a `database/sql` driver so every statement shows up as a span under the operation that ran it.

## Overview

Without it, a trace shows a request and its service spans but not the SQL they ran. `WrapDriver` wraps any
`driver.Driver` with [otelsql](https://github.com/XSAM/otelsql), configured once here so every service traces SQL
the same way. Spans go to the global OpenTelemetry provider, which `tracer.NewOTel` installs. With tracing off the
global provider is a no-op, so the wrapper costs almost nothing.

## Features

- **Statement spans**: `Exec`, `Query`, `Prepare`, `Begin`, `Commit` and `Rollback` each get a span with
  `db.system.name: postgresql` and `db.query.text` (the SQL with its placeholders).
- **No argument values**: bound arguments are never recorded.
- **Quiet**: no spans for `Ping`, `Rows.Next`, `ResetSession` or connector `Connect`.
- **No orphan traces**: a statement whose context has no parent span gets no span, so health checks, pool
  maintenance and idle worker polls do not each start a one-span trace.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/tracer/tracesql
```

### Basic usage

Register the wrapped driver once (an `init` is the usual place) and open `sqlkit` or `sql.Open` against its name:

```go
import (
    "database/sql"

    "github.com/jackc/pgx/v5/stdlib"

    "github.com/biairmal/go-sdk/lib/tracer/tracesql"
)

func init() {
    sql.Register("pgx-traced", tracesql.WrapDriver(stdlib.GetDefaultDriver()))
}
```

Pass the request's context to every query (`QueryContext`, `ExecContext`); a context without a span produces no
SQL spans.

## Limitations

- **PostgreSQL only**: `db.system.name` is fixed to `postgresql`.
- **No SQL metrics**: otelsql's metrics are not exported, since the SDK does not install a global meter provider.

## Dependencies

- [github.com/XSAM/otelsql](https://pkg.go.dev/github.com/XSAM/otelsql) – the driver wrapper.
- [go.opentelemetry.io/otel](https://pkg.go.dev/go.opentelemetry.io/otel) – `/attribute`, `/trace`.

## See also

- [tracer](../README.md) – `NewOTel` installs the global provider these spans go to.
