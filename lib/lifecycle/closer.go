package lifecycle

import "context"

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/lifecycle/mock_lifecycle.go -package=mocklifecycle github.com/biairmal/go-sdk/lib/lifecycle Closer

// Closer releases a resource during the shutdown sequence's closer phase.
// Implementations should respect ctx's deadline (Config.CloserTimeout) and
// return promptly once it expires.
type Closer interface {
	// Close releases the resource. A non-nil error is logged (if a logger is
	// set) and joined into Run's returned error; it does not stop other
	// registered closers from running.
	Close(ctx context.Context) error
}

// CloserFunc adapts a plain func to Closer.
type CloserFunc func(ctx context.Context) error

// Close calls f.
func (f CloserFunc) Close(ctx context.Context) error { return f(ctx) }

// CloserFromTracer adapts any value with a Shutdown(ctx) error method — e.g.
// a tracer.Tracer — to Closer. It's defined structurally (rather than taking
// tracer.Tracer directly) so lifecycle doesn't need to import the tracer
// package; any type with a matching Shutdown method works. Returns nil for a
// nil t (a literal untyped nil; a nil value of a concrete pointer type
// wrapped in the interface parameter is not caught — Go's usual typed-nil
// caveat).
func CloserFromTracer(t interface{ Shutdown(context.Context) error }) Closer {
	if t == nil {
		return nil
	}
	return CloserFunc(t.Shutdown)
}

// CloserFromDB adapts any value with a Close() error method — e.g. a
// *sqlkit.DB — to Closer. Close() takes no context, so the underlying call
// cannot be preempted by ctx's deadline; it always runs to completion. See
// CloserFromTracer's doc comment for the same typed-nil caveat.
func CloserFromDB(db interface{ Close() error }) Closer {
	if db == nil {
		return nil
	}
	return CloserFunc(func(_ context.Context) error { return db.Close() })
}

// CloserFromRedis adapts any value with a Close() error method — e.g. a
// redis.Client — to Closer. Same context-deadline and typed-nil caveats as
// CloserFromDB.
func CloserFromRedis(c interface{ Close() error }) Closer {
	if c == nil {
		return nil
	}
	return CloserFunc(func(_ context.Context) error { return c.Close() })
}
