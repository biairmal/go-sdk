package lifecycle

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

// fastCloser records that it ran, in order, into a shared, mutex-guarded log.
type fastCloser struct {
	name string
	log  *callLog
	err  error
}

func (c *fastCloser) Close(_ context.Context) error {
	c.log.record(c.name)
	return c.err
}

// callLog is a concurrency-safe append-only list of closer names, used to
// assert registration-order execution.
type callLog struct {
	mu    sync.Mutex
	names []string
}

func (l *callLog) record(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.names = append(l.names, name)
}

func (l *callLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.names))
	copy(out, l.names)
	return out
}

// blockingCloser blocks until ctx is done (or blockCh is signaled), notifying
// startedCh once it has begun, so tests can synchronize on "the closer phase
// is now in progress" without racing on timing.
type blockingCloser struct {
	startedCh chan struct{}
}

func (c *blockingCloser) Close(ctx context.Context) error {
	close(c.startedCh)
	<-ctx.Done()
	return ctx.Err()
}

func newTestServer() *http.Server {
	return &http.Server{Addr: "127.0.0.1:0"}
}

func TestRun_HappyPath(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)

	log := &callLog{}
	sigCh := make(chan os.Signal, 2)
	sigCh <- syscall.SIGINT

	r := newRunner([]Option{
		WithReadiness(&ready),
		WithCloser("tracer", &fastCloser{name: "tracer", log: log}),
		WithCloser("redis", &fastCloser{name: "redis", log: log}),
		WithCloser("db", &fastCloser{name: "db", log: log, err: errBoom}),
	})
	cfg := withDefaults(Config{DrainDelay: time.Millisecond, ShutdownTimeout: time.Second, CloserTimeout: time.Second})

	err := r.run(context.Background(), newTestServer(), cfg, sigCh)

	if !errors.Is(err, errBoom) {
		t.Fatalf("run() error = %v, want it to wrap errBoom", err)
	}
	if errors.Is(err, ErrForcedShutdown) {
		t.Fatalf("run() error = %v, want no ErrForcedShutdown on a clean single-signal shutdown", err)
	}
	if ready.Load() {
		t.Error("readiness flag still true after shutdown, want false")
	}
	want := []string{"tracer", "redis", "db"}
	got := log.snapshot()
	if len(got) != len(want) {
		t.Fatalf("closer call order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("closer call order = %v, want %v", got, want)
		}
	}
}

func TestRun_TriggeredByContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := newRunner(nil)
	cfg := withDefaults(Config{DrainDelay: time.Millisecond, ShutdownTimeout: time.Second, CloserTimeout: time.Second})
	sigCh := make(chan os.Signal, 2)

	err := r.run(ctx, newTestServer(), cfg, sigCh)
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}
}

func TestRun_ShutdownHookErrorIsJoinedButDoesNotStopShutdown(t *testing.T) {
	log := &callLog{}
	sigCh := make(chan os.Signal, 2)
	sigCh <- syscall.SIGINT

	hookErr := errors.New("hook failed")
	r := newRunner([]Option{
		WithShutdownFunc(func(context.Context) error { return hookErr }),
		WithCloser("db", &fastCloser{name: "db", log: log}),
	})
	cfg := withDefaults(Config{DrainDelay: time.Millisecond, ShutdownTimeout: time.Second, CloserTimeout: time.Second})

	err := r.run(context.Background(), newTestServer(), cfg, sigCh)
	if !errors.Is(err, hookErr) {
		t.Fatalf("run() error = %v, want it to wrap hookErr", err)
	}
	if got := log.snapshot(); len(got) != 1 || got[0] != "db" {
		t.Fatalf("closer calls = %v, want [db] (hook failure must not skip closers)", got)
	}
}

func TestRun_SecondSignalDuringCloserPhaseForcesExit(t *testing.T) {
	startedCh := make(chan struct{})
	sigCh := make(chan os.Signal, 2)
	sigCh <- syscall.SIGINT

	r := newRunner([]Option{
		WithCloser("slow", &blockingCloser{startedCh: startedCh}),
	})
	// Small CloserTimeout so the leaked background goroutine (still blocked
	// on ctx.Done() after run() returns early) unblocks quickly.
	cfg := withDefaults(Config{
		DrainDelay: time.Millisecond, ShutdownTimeout: time.Second, CloserTimeout: 200 * time.Millisecond,
	})

	resultCh := make(chan error, 1)
	go func() { resultCh <- r.run(context.Background(), newTestServer(), cfg, sigCh) }()

	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("closer phase never started")
	}
	sigCh <- syscall.SIGINT

	select {
	case err := <-resultCh:
		if !errors.Is(err, ErrForcedShutdown) {
			t.Fatalf("run() error = %v, want ErrForcedShutdown", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run() did not return promptly after a second signal")
	}
}

func TestRun_SecondSignalDuringDrainDelayForcesExit(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	sigCh <- syscall.SIGINT
	sigCh <- syscall.SIGINT

	r := newRunner(nil)
	cfg := withDefaults(Config{DrainDelay: time.Hour, ShutdownTimeout: time.Second, CloserTimeout: time.Second})

	resultCh := make(chan error, 1)
	go func() { resultCh <- r.run(context.Background(), newTestServer(), cfg, sigCh) }()

	select {
	case err := <-resultCh:
		if !errors.Is(err, ErrForcedShutdown) {
			t.Fatalf("run() error = %v, want ErrForcedShutdown", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run() did not return promptly after a second signal during the drain delay")
	}
}

func TestWithCloser_NilClosersAreIgnored(t *testing.T) {
	r := newRunner([]Option{WithCloser("nil", nil)})
	if len(r.closers) != 0 {
		t.Fatalf("closers = %v, want none registered for a nil Closer", r.closers)
	}
}

func TestWithSignals_EmptyIsNoOp(t *testing.T) {
	def := newRunner(nil).signals
	r := newRunner([]Option{WithSignals()})
	if len(r.signals) != len(def) {
		t.Fatalf("signals = %v, want default %v preserved on an empty WithSignals call", r.signals, def)
	}
}

func TestWithReadiness_NilFlagSkipsFlip(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	sigCh <- syscall.SIGINT
	r := newRunner(nil)
	cfg := withDefaults(Config{DrainDelay: time.Millisecond, ShutdownTimeout: time.Second, CloserTimeout: time.Second})

	if err := r.run(context.Background(), newTestServer(), cfg, sigCh); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}
}

func TestCloserAdapters(t *testing.T) {
	t.Run("CloserFromTracer wraps Shutdown", func(t *testing.T) {
		called := false
		tr := fakeShutdowner{fn: func(context.Context) error { called = true; return nil }}
		c := CloserFromTracer(tr)
		if err := c.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}
		if !called {
			t.Error("underlying Shutdown was not called")
		}
	})

	t.Run("CloserFromTracer nil returns nil Closer", func(t *testing.T) {
		if c := CloserFromTracer(nil); c != nil {
			t.Errorf("CloserFromTracer(nil) = %v, want nil", c)
		}
	})

	t.Run("CloserFromDB wraps Close and ignores ctx", func(t *testing.T) {
		called := false
		db := fakeCloser{fn: func() error { called = true; return nil }}
		c := CloserFromDB(db)
		if err := c.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}
		if !called {
			t.Error("underlying Close was not called")
		}
	})

	t.Run("CloserFromDB nil returns nil Closer", func(t *testing.T) {
		if c := CloserFromDB(nil); c != nil {
			t.Errorf("CloserFromDB(nil) = %v, want nil", c)
		}
	})

	t.Run("CloserFromRedis wraps Close", func(t *testing.T) {
		called := false
		rdb := fakeCloser{fn: func() error { called = true; return errBoom }}
		c := CloserFromRedis(rdb)
		if err := c.Close(context.Background()); !errors.Is(err, errBoom) {
			t.Fatalf("Close() error = %v, want errBoom", err)
		}
		if !called {
			t.Error("underlying Close was not called")
		}
	})

	t.Run("CloserFromRedis nil returns nil Closer", func(t *testing.T) {
		if c := CloserFromRedis(nil); c != nil {
			t.Errorf("CloserFromRedis(nil) = %v, want nil", c)
		}
	})
}

type fakeShutdowner struct {
	fn func(context.Context) error
}

func (f fakeShutdowner) Shutdown(ctx context.Context) error { return f.fn(ctx) }

type fakeCloser struct {
	fn func() error
}

func (f fakeCloser) Close() error { return f.fn() }

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "valid config", cfg: DefaultConfig()},
		{
			name:    "negative drain_delay",
			cfg:     Config{DrainDelay: -time.Second, ShutdownTimeout: time.Second, CloserTimeout: time.Second},
			wantErr: true,
		},
		{
			name:    "non-positive shutdown_timeout",
			cfg:     Config{DrainDelay: 0, ShutdownTimeout: 0, CloserTimeout: time.Second},
			wantErr: true,
		},
		{
			name:    "non-positive closer_timeout",
			cfg:     Config{DrainDelay: 0, ShutdownTimeout: time.Second, CloserTimeout: 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig_IsValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() = %v, want nil", err)
	}
}

func TestWithDefaults_ZeroConfigUsesDefaults(t *testing.T) {
	got := withDefaults(Config{})
	if got != DefaultConfig() {
		t.Fatalf("withDefaults(Config{}) = %+v, want %+v", got, DefaultConfig())
	}
}
