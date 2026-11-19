package tracesql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

var driverSeq atomic.Int64

// openTraced registers the fake driver wrapped by WrapDriver and opens it,
// with spans going to an in-memory exporter through the global provider.
func openTraced(t *testing.T) (*sql.DB, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp)))

	name := fmt.Sprintf("fake-traced-%d", driverSeq.Add(1))
	sql.Register(name, WrapDriver(fakeDriver{}))
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, exp
}

func TestWrapDriver(t *testing.T) {
	const secret = "s3cret@example.com"
	tests := []struct {
		name      string
		hasParent bool
		wantSpans []string // each exported span: its db.query.text, or its name when it has none
	}{
		{"under a parent span", true, []string{
			"INSERT INTO note (body) VALUES ($1)", "SELECT body FROM note WHERE id = $1", "sql.conn.begin_tx", "sql.tx.commit",
		}},
		{"no parent span", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, exp := openTraced(t)
			ctx := context.Background()
			if tt.hasParent {
				var span trace.Span
				ctx, span = otel.Tracer("test").Start(ctx, "parent")
				defer span.End()
			}

			if err := db.PingContext(ctx); err != nil {
				t.Fatalf("Ping: %v", err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO note (body) VALUES ($1)", secret); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			rows, err := db.QueryContext(ctx, "SELECT body FROM note WHERE id = $1", secret)
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			for rows.Next() {
			}
			rows.Close()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("Begin: %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}

			var got []string
			for _, s := range exp.GetSpans() {
				if s.Name == "parent" {
					continue
				}
				attrs := map[string]string{}
				for _, kv := range s.Attributes {
					attrs[string(kv.Key)] = kv.Value.String()
					if strings.Contains(kv.Value.String(), secret) {
						t.Errorf("span %q attribute %s records an argument value", s.Name, kv.Key)
					}
				}
				if attrs["db.system.name"] != "postgresql" {
					t.Errorf("span %q db.system.name = %q", s.Name, attrs["db.system.name"])
				}
				if q, ok := attrs["db.query.text"]; ok {
					got = append(got, q)
				} else {
					got = append(got, s.Name)
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.wantSpans, "|") {
				t.Errorf("statement spans = %q, want %q", got, tt.wantSpans)
			}
		})
	}
}

// fakeDriver is the smallest driver with context-aware Exec, Query and Ping.
type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{}, nil }

type fakeConn struct{}

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (fakeConn) Close() error                        { return nil }
func (fakeConn) Begin() (driver.Tx, error)           { return fakeTx{}, nil }
func (fakeConn) Ping(context.Context) error          { return nil }
func (fakeConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

func (fakeConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &fakeRows{}, nil
}

type fakeTx struct{}

func (fakeTx) Commit() error   { return nil }
func (fakeTx) Rollback() error { return nil }

type fakeRows struct{ done bool }

func (*fakeRows) Columns() []string { return []string{"body"} }
func (*fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "hello"
	return nil
}
