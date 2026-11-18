// Package tracesql wraps a database/sql driver so every statement a service
// runs becomes a span under the operation that ran it. Spans go to the global
// OpenTelemetry provider, which tracer.NewOTel installs; until then (or with
// tracing off) the wrapper is close to free.
//
// Example usage:
//
//	sql.Register("pgx-traced", tracesql.WrapDriver(stdlib.GetDefaultDriver()))
//	db, err := sql.Open("pgx-traced", dsn)
package tracesql

import (
	"context"
	"database/sql/driver"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// WrapDriver returns d with tracing. Each statement span carries
// db.system.name=postgresql and db.query.text (the SQL with its placeholders);
// argument values are never recorded. Begin, Commit and Rollback are traced;
// Ping, Rows.Next, ResetSession and connector Connect are not.
//
// A statement whose context has no valid parent span gets no span, so health
// checks, pool maintenance and idle worker polls do not each start a one-span
// trace.
func WrapDriver(d driver.Driver) driver.Driver {
	return otelsql.WrapDriver(d,
		otelsql.WithAttributes(attribute.String("db.system.name", "postgresql")),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			OmitRows:             true,
			OmitConnResetSession: true,
			OmitConnectorConnect: true,
			SpanFilter:           hasParent,
		}),
	)
}

// hasParent reports whether ctx carries a span to hang the statement span under.
func hasParent(ctx context.Context, _ otelsql.Method, _ string, _ []driver.NamedValue) bool {
	return trace.SpanContextFromContext(ctx).IsValid()
}
