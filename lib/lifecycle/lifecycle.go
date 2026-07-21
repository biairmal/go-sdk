// Package lifecycle provides ordered, deadline-bounded graceful shutdown:
// trap a shutdown signal (or a canceled context), flip a readiness flag so
// load balancers stop routing traffic, wait out a drain delay for that to
// propagate, drain in-flight HTTP requests via http.Server.Shutdown, then
// close registered resources (DB, Redis, tracer flush, …) in registration
// order under their own deadline. A second shutdown signal at any point
// abandons the remaining budget and returns immediately.
//
// Example usage:
//
//	var ready atomic.Bool
//	ready.Store(true)
//	mux.HandleFunc("/readyz", httpkit.Readiness(func(ctx context.Context) error {
//		if !ready.Load() {
//			return errorz.ServiceUnavailable()
//		}
//		return nil
//	}))
//
//	srv := &http.Server{Addr: ":8080", Handler: handler}
//	go func() {
//		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
//			log.Error("listen", logger.F("error", err.Error()))
//		}
//	}()
//
//	err := lifecycle.Run(context.Background(), srv, lifecycle.DefaultConfig(),
//		lifecycle.WithReadiness(&ready),
//		lifecycle.WithLogger(log),
//		lifecycle.WithCloser("tracer", lifecycle.CloserFromTracer(tr)),
//		lifecycle.WithCloser("redis", lifecycle.CloserFromRedis(rdb)),
//		lifecycle.WithCloser("db", lifecycle.CloserFromDB(db)),
//	)
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
)

// ErrForcedShutdown indicates Run abandoned the remaining shutdown budget
// because a second shutdown signal arrived before graceful shutdown
// finished. It's joined into Run's returned error via errors.Join, so
// errors.Is(err, lifecycle.ErrForcedShutdown) reports whether shutdown was
// forced.
var ErrForcedShutdown = errors.New("lifecycle: forced shutdown: second signal received")

// namedCloser pairs a registered Closer with the name it was registered
// under, for logging and error attribution.
type namedCloser struct {
	name   string
	closer Closer
}

// runner holds the live, non-serializable state assembled from Options.
type runner struct {
	ready      *atomic.Bool
	signals    []os.Signal
	log        logger.Logger
	closers    []namedCloser
	shutdownFn func(context.Context) error
}

// newRunner builds a runner defaulted to os.Interrupt/syscall.SIGTERM, then
// applies opts in order.
func newRunner(opts []Option) *runner {
	r := &runner{signals: []os.Signal{os.Interrupt, syscall.SIGTERM}}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// Run blocks until a configured signal arrives or ctx is done, then drives
// graceful shutdown of srv plus every registered Closer: flip readiness →
// run the optional shutdown hook → wait DrainDelay → srv.Shutdown under
// ShutdownTimeout → run closers in registration order under CloserTimeout.
// A second shutdown signal received at any point during that sequence
// abandons the remaining budget, hard-closes srv, and returns immediately
// (see ErrForcedShutdown). Returns the joined result of every phase (nil if
// everything succeeded).
func Run(ctx context.Context, srv *http.Server, cfg Config, opts ...Option) error {
	if srv == nil {
		return errorz.BadRequest().WithMessage("lifecycle: srv must not be nil")
	}
	cfg = withDefaults(cfg)
	r := newRunner(opts)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, r.signals...)
	defer signal.Stop(sigCh)

	return r.run(ctx, srv, cfg, sigCh)
}

// run is Run's core, decoupled from the real OS signal channel so tests can
// drive it deterministically by sending values into a fake sigCh instead of
// relying on real OS signal delivery.
func (r *runner) run(ctx context.Context, srv *http.Server, cfg Config, sigCh chan os.Signal) error {
	select {
	case <-sigCh:
	case <-ctx.Done():
	}
	r.logInfo("lifecycle: shutdown triggered")

	if r.ready != nil {
		r.ready.Store(false)
	}

	var errs []error
	if r.shutdownFn != nil {
		if err := r.shutdownFn(context.Background()); err != nil {
			errs = append(errs, fmt.Errorf("lifecycle: shutdown hook: %w", err))
			r.logError("lifecycle: shutdown hook failed", err)
		}
	}

	if waitOrForced(sigCh, cfg.DrainDelay) {
		return r.forceExit(srv, errs)
	}

	ok, shutdownErr := runPhase(sigCh, func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})
	if !ok {
		return r.forceExit(srv, errs)
	}
	if shutdownErr != nil {
		errs = append(errs, fmt.Errorf("lifecycle: http shutdown: %w", shutdownErr))
	}

	ok, closerErr := runPhase(sigCh, func() error { return r.runClosers(cfg.CloserTimeout) })
	if !ok {
		return r.forceExit(srv, errs)
	}
	if closerErr != nil {
		errs = append(errs, closerErr)
	}
	return errors.Join(errs...)
}

// waitOrForced blocks for d (a no-op if d <= 0), returning true if sigCh
// fires first — i.e. a shutdown signal arrived during the wait.
func waitOrForced(sigCh <-chan os.Signal, d time.Duration) bool {
	if d <= 0 {
		return false
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return false
	case <-sigCh:
		return true
	}
}

// runPhase runs fn in a goroutine, returning (true, fn's error) once it
// completes, or (false, nil) immediately if sigCh fires first — fn keeps
// running in the background under its own deadline, but its eventual result
// is discarded.
func runPhase(sigCh <-chan os.Signal, fn func() error) (bool, error) {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return true, err
	case <-sigCh:
		return false, nil
	}
}

// forceExit is invoked when a second signal aborts shutdown mid-flight. It
// hard-closes srv (bypassing any remaining graceful drain), logs a warning,
// and returns the errors accumulated so far joined with ErrForcedShutdown.
func (r *runner) forceExit(srv *http.Server, errs []error) error {
	r.logWarn("lifecycle: second signal received, forcing shutdown")
	_ = srv.Close()
	errs = append(errs, ErrForcedShutdown)
	return errors.Join(errs...)
}

// runClosers runs every registered closer sequentially, in registration
// order, sharing one deadline (timeout) for the whole phase. A closer's own
// error is logged and joined, but doesn't stop the rest from running.
func (r *runner) runClosers(timeout time.Duration) error {
	if len(r.closers) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var errs []error
	for _, nc := range r.closers {
		start := time.Now()
		err := nc.closer.Close(ctx)
		dur := time.Since(start)
		if err != nil {
			errs = append(errs, fmt.Errorf("lifecycle: closer %q: %w", nc.name, err))
			r.logCloserError(nc.name, dur, err)
			continue
		}
		r.logCloserDone(nc.name, dur)
	}
	return errors.Join(errs...)
}

// logInfo logs msg at info level if a logger is set.
func (r *runner) logInfo(msg string) {
	if r.log == nil {
		return
	}
	r.log.Info(msg)
}

// logWarn logs msg at warn level if a logger is set.
func (r *runner) logWarn(msg string) {
	if r.log == nil {
		return
	}
	r.log.Warn(msg)
}

// logError logs msg with err at error level if a logger is set.
func (r *runner) logError(msg string, err error) {
	if r.log == nil {
		return
	}
	r.log.Error(msg, logger.F("error", err.Error()))
}

// logCloserDone logs a successful closer's duration if a logger is set.
func (r *runner) logCloserDone(name string, dur time.Duration) {
	if r.log == nil {
		return
	}
	r.log.Info("lifecycle: closer finished", logger.F("closer", name), logger.F("duration", dur.String()))
}

// logCloserError logs a failed closer's duration and error if a logger is set.
func (r *runner) logCloserError(name string, dur time.Duration, err error) {
	if r.log == nil {
		return
	}
	r.log.Error("lifecycle: closer failed",
		logger.F("closer", name), logger.F("duration", dur.String()), logger.F("error", err.Error()))
}
