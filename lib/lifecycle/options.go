package lifecycle

import (
	"context"
	"os"
	"sync/atomic"

	"github.com/biairmal/go-sdk/lib/logger"
)

// Option configures Run with optional, non-serializable behavior.
type Option func(*runner)

// WithReadiness wires a readiness flag that Run flips to false as soon as a
// shutdown signal is received, before the shutdown hook and drain delay —
// pair it with httpkit.Readiness so load balancers see 503 and stop routing
// new traffic. Nil-safe: without this option Run skips the readiness flip.
func WithReadiness(ready *atomic.Bool) Option {
	return func(r *runner) {
		r.ready = ready
	}
}

// WithSignals overrides the OS signals that trigger shutdown. Default:
// os.Interrupt, syscall.SIGTERM. Called with no signals, it's a no-op (keeps
// the default).
func WithSignals(sig ...os.Signal) Option {
	return func(r *runner) {
		if len(sig) > 0 {
			r.signals = sig
		}
	}
}

// WithLogger sets an optional logger for shutdown progress: signal
// received, shutdown-hook failure, per-closer duration/outcome, and the
// forced-exit warning. Nil-safe: without it Run stays silent.
func WithLogger(log logger.Logger) Option {
	return func(r *runner) {
		r.log = log
	}
}

// WithCloser registers a named Closer to run during the closer phase, after
// the HTTP server has drained. Closers run sequentially in registration
// order — register least-recoverable first (e.g. tracer flush, so it
// captures spans from requests that just finished draining, then caches,
// then the database last, since other closers may still need it) so the
// example lifecycle.Run wiring in the package doc reads top-to-bottom as
// the actual close order. Repeated calls append; a nil closer is ignored.
func WithCloser(name string, c Closer) Option {
	return func(r *runner) {
		if c == nil {
			return
		}
		r.closers = append(r.closers, namedCloser{name: name, closer: c})
	}
}

// WithShutdownFunc registers a hook run once, synchronously, right after the
// readiness flip and before the drain delay — for custom pre-shutdown logic
// that doesn't fit the Closer model (e.g. stopping a background worker pool
// from picking up new jobs). It is not bounded by ShutdownTimeout or
// CloserTimeout, so keep it fast. A non-nil error is logged (if a logger is
// set) and joined into Run's returned error, but does not stop the rest of
// shutdown. Nil-safe.
func WithShutdownFunc(fn func(ctx context.Context) error) Option {
	return func(r *runner) {
		r.shutdownFn = fn
	}
}
