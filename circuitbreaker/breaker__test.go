package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/errorz"
)

const sec = time.Second

var errBoom = errors.New("boom")

// manualClock is a test-only time source controlled explicitly via Advance,
// so OpenTimeout expiry can be tested without sleeping.
type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func newManualClock() *manualClock { return &manualClock{now: time.Unix(0, 0)} }

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func TestBreaker_OpenRejectsWithoutCallingFn(t *testing.T) {
	b := NewBreaker(Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1})
	ctx := context.Background()
	_ = b.Execute(ctx, func(context.Context) error { return errBoom })

	var called bool
	err := b.Execute(ctx, func(context.Context) error { called = true; return nil })
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("Execute() error = %v, want ErrOpen", err)
	}
	if called {
		t.Error("Execute() invoked fn while Open, want rejected before calling it")
	}
}

func TestBreaker_TripsOnConsecutiveFailures(t *testing.T) {
	b := NewBreaker(Config{FailureThreshold: 3, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1})
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_ = b.Execute(ctx, func(context.Context) error { return errBoom })
		if got := b.State(); got != StateClosed {
			t.Fatalf("State() after %d failure(s) = %v, want Closed", i+1, got)
		}
	}

	err := b.Execute(ctx, func(context.Context) error { return errBoom })
	if !errors.Is(err, errBoom) {
		t.Fatalf("Execute() error on the tripping call = %v, want errBoom (fn's own error, not ErrOpen)", err)
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("State() after 3rd consecutive failure = %v, want Open", got)
	}
}

func TestBreaker_SuccessResetsConsecutiveFailures(t *testing.T) {
	b := NewBreaker(Config{FailureThreshold: 2, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1})
	ctx := context.Background()

	_ = b.Execute(ctx, func(context.Context) error { return errBoom })
	_ = b.Execute(ctx, func(context.Context) error { return nil })
	_ = b.Execute(ctx, func(context.Context) error { return errBoom })

	if got := b.State(); got != StateClosed {
		t.Fatalf("State() = %v, want Closed (a success should reset the consecutive-failure streak)", got)
	}
}

func TestBreaker_TripsOnFailureRatio(t *testing.T) {
	// FailureThreshold=4 keeps the consecutive rule from firing on its own
	// (failures never run more than 1 deep) and doubles as the minimum
	// sample size before the ratio rule is evaluated.
	b := NewBreaker(Config{FailureThreshold: 4, FailureRatio: 0.5, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1})
	ctx := context.Background()

	outcomes := []bool{true, false, true, false} // 2/4 = 50% failure rate
	for i, ok := range outcomes {
		var callErr error
		if !ok {
			callErr = errBoom
		}
		_ = b.Execute(ctx, func(context.Context) error { return callErr })

		wantOpen := i == len(outcomes)-1
		if got := b.State() == StateOpen; got != wantOpen {
			t.Fatalf("after call %d, State() open = %v, want %v", i+1, got, wantOpen)
		}
	}
}

func TestBreaker_OpenTransitionsToHalfOpenAfterTimeout(t *testing.T) {
	clock := newManualClock()
	b := NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: sec, HalfOpenMaxCalls: 1},
		WithClock(clock.Now),
	)
	ctx := context.Background()
	_ = b.Execute(ctx, func(context.Context) error { return errBoom })

	if err := b.Execute(ctx, func(context.Context) error { return nil }); !errors.Is(err, ErrOpen) {
		t.Fatalf("Execute() before OpenTimeout elapsed = %v, want ErrOpen", err)
	}

	clock.Advance(sec)

	var probed bool
	if err := b.Execute(ctx, func(context.Context) error { probed = true; return nil }); err != nil {
		t.Fatalf("Execute() after OpenTimeout elapsed = %v, want nil", err)
	}
	if !probed {
		t.Error("Execute() after OpenTimeout elapsed did not call fn as a Half-Open probe")
	}
}

func TestBreaker_HalfOpenClosesAfterEnoughSuccesses(t *testing.T) {
	clock := newManualClock()
	b := NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: sec, HalfOpenMaxCalls: 2},
		WithClock(clock.Now),
	)
	ctx := context.Background()
	_ = b.Execute(ctx, func(context.Context) error { return errBoom })
	clock.Advance(sec)

	for i := 0; i < 2; i++ {
		if err := b.Execute(ctx, func(context.Context) error { return nil }); err != nil {
			t.Fatalf("probe %d Execute() error = %v, want nil", i+1, err)
		}
	}

	if got := b.State(); got != StateClosed {
		t.Fatalf("State() after 2 successful probes = %v, want Closed", got)
	}
}

func TestBreaker_HalfOpenReopensOnFailure(t *testing.T) {
	clock := newManualClock()
	b := NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: sec, HalfOpenMaxCalls: 2},
		WithClock(clock.Now),
	)
	ctx := context.Background()
	_ = b.Execute(ctx, func(context.Context) error { return errBoom })
	clock.Advance(sec)

	err := b.Execute(ctx, func(context.Context) error { return errBoom })
	if !errors.Is(err, errBoom) {
		t.Fatalf("probe Execute() error = %v, want errBoom", err)
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("State() after failed probe = %v, want Open", got)
	}

	if err := b.Execute(ctx, func(context.Context) error { return nil }); !errors.Is(err, ErrOpen) {
		t.Fatalf("Execute() immediately after reopening = %v, want ErrOpen", err)
	}
}

func TestBreaker_HalfOpenLimitsConcurrentProbes(t *testing.T) {
	clock := newManualClock()
	b := NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: sec, HalfOpenMaxCalls: 1},
		WithClock(clock.Now),
	)
	ctx := context.Background()
	_ = b.Execute(ctx, func(context.Context) error { return errBoom })
	clock.Advance(sec)

	started := make(chan struct{})
	release := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Execute(ctx, func(context.Context) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started // first probe now holds the only Half-Open slot

	var fn2Called bool
	err := b.Execute(ctx, func(context.Context) error { fn2Called = true; return nil })
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("second concurrent probe error = %v, want ErrOpen", err)
	}
	if fn2Called {
		t.Error("second concurrent probe invoked fn, want rejected before calling it")
	}

	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("first probe Execute() error = %v, want nil", err)
	}
	if got := b.State(); got != StateClosed {
		t.Fatalf("State() after the only probe succeeded = %v, want Closed", got)
	}
}

func TestBreaker_HalfOpenStaleProbeResultIgnored(t *testing.T) {
	clock := newManualClock()
	b := NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: sec, HalfOpenMaxCalls: 2},
		WithClock(clock.Now),
	)
	ctx := context.Background()
	_ = b.Execute(ctx, func(context.Context) error { return errBoom })
	clock.Advance(sec)

	startedA := make(chan struct{})
	releaseA := make(chan struct{})
	errA := make(chan error, 1)
	go func() {
		errA <- b.Execute(ctx, func(context.Context) error {
			close(startedA)
			<-releaseA
			return nil // succeeds, but only after the breaker has already reopened below
		})
	}()
	<-startedA

	if err := b.Execute(ctx, func(context.Context) error { return errBoom }); !errors.Is(err, errBoom) {
		t.Fatalf("second probe Execute() error = %v, want errBoom", err)
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("State() after failed probe = %v, want Open", got)
	}

	close(releaseA)
	if err := <-errA; err != nil {
		t.Fatalf("stale probe Execute() error = %v, want nil", err)
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("State() after stale probe's success arrived = %v, want still Open", got)
	}
}

func TestBreaker_PanicIsRecordedAndRepanicked(t *testing.T) {
	b := NewBreaker(Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1})
	ctx := context.Background()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Execute() did not re-panic")
			}
		}()
		_ = b.Execute(ctx, func(context.Context) error { panic("boom") })
	}()

	if got := b.State(); got != StateOpen {
		t.Fatalf("State() after a panicking call = %v, want Open (a panic must count as a failure)", got)
	}
}

func TestBreaker_WithOnStateChangeCallback(t *testing.T) {
	var mu sync.Mutex
	var got []transition

	b := NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1},
		WithOnStateChange(func(from, to State) {
			mu.Lock()
			got = append(got, transition{from: from, to: to})
			mu.Unlock()
		}),
	)
	_ = b.Execute(context.Background(), func(context.Context) error { return errBoom })

	mu.Lock()
	defer mu.Unlock()
	want := []transition{{from: StateClosed, to: StateOpen}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("transitions = %+v, want %+v", got, want)
	}
}

func TestBreaker_OnStateChangeCanCallStateWithoutDeadlock(t *testing.T) {
	done := make(chan State, 1)
	var b Breaker
	b = NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1},
		WithOnStateChange(func(_, _ State) { done <- b.State() }),
	)
	_ = b.Execute(context.Background(), func(context.Context) error { return errBoom })

	select {
	case got := <-done:
		if got != StateOpen {
			t.Fatalf("State() called from within the callback = %v, want Open", got)
		}
	case <-time.After(sec):
		t.Fatal("WithOnStateChange callback calling State() deadlocked")
	}
}

func TestBreaker_WithIsSuccessfulCustomClassifier(t *testing.T) {
	b := NewBreaker(
		Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1},
		WithIsSuccessful(func(err error) bool { return err == nil || errors.Is(err, context.Canceled) }),
	)
	ctx := context.Background()

	err := b.Execute(ctx, func(context.Context) error { return context.Canceled })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
	if got := b.State(); got != StateClosed {
		t.Fatalf("State() after a call classified as success = %v, want Closed", got)
	}
}

func TestErrOpen_WrapsServiceUnavailable(t *testing.T) {
	b := NewBreaker(Config{FailureThreshold: 1, FailureRatio: 1, OpenTimeout: time.Minute, HalfOpenMaxCalls: 1})
	_ = b.Execute(context.Background(), func(context.Context) error { return errBoom })

	err := b.Execute(context.Background(), func(context.Context) error { return nil })
	if !errors.Is(err, ErrOpen) {
		t.Fatal("errors.Is(err, ErrOpen) = false, want true")
	}
	if !errors.Is(err, errorz.ErrServiceUnavailable) {
		t.Fatal("errors.Is(err, errorz.ErrServiceUnavailable) = false, want true")
	}
	var ez *errorz.Error
	if !errors.As(err, &ez) {
		t.Fatal("errors.As(err, *errorz.Error) = false, want true")
	}
	if ez.Code != errorz.CodeServiceUnavailable {
		t.Errorf("code = %q, want %q", ez.Code, errorz.CodeServiceUnavailable)
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		name string
		s    State
		want string
	}{
		{"closed", StateClosed, "closed"},
		{"open", StateOpen, "open"},
		{"half_open", StateHalfOpen, "half_open"},
		{"unknown", State(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDo(t *testing.T) {
	b := NewBreaker(DefaultConfig())
	ctx := context.Background()

	v, err := Do(ctx, b, func(context.Context) (int, error) { return 42, nil })
	if err != nil || v != 42 {
		t.Fatalf("Do() = (%d, %v), want (42, nil)", v, err)
	}

	v, err = Do(ctx, b, func(context.Context) (int, error) { return 7, errBoom })
	if !errors.Is(err, errBoom) || v != 0 {
		t.Fatalf("Do() = (%d, %v), want (0, errBoom)", v, err)
	}
}

func TestBreaker_ConcurrentUse(t *testing.T) {
	// Not skipped under testing.Short(): it's fast, and make test-race (which
	// is what this test exists for) also passes -short.
	cfg := Config{FailureThreshold: 5, FailureRatio: 0.5, OpenTimeout: 10 * time.Millisecond, HalfOpenMaxCalls: 2}
	b := NewBreaker(cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = b.Execute(ctx, func(context.Context) error {
					if (i+j)%3 == 0 {
						return errBoom
					}
					return nil
				})
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Execute calls did not complete, possible deadlock")
	}
}
