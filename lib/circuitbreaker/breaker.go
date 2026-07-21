// Package circuitbreaker protects calls to unreliable dependencies with a
// lean, dependency-free closed→open→half-open state machine: Breaker.Execute
// (or the generic Do helper, for calls that return a value) runs a call
// while Closed, trips to Open once failures cross a configured threshold or
// ratio, rejects calls immediately with ErrOpen while Open, and after
// OpenTimeout admits a bounded number of Half-Open probes to test recovery
// before closing again.
//
// Example usage:
//
//	b := circuitbreaker.NewBreaker(circuitbreaker.DefaultConfig())
//	err := b.Execute(ctx, func(ctx context.Context) error {
//		return callFlakyDependency(ctx)
//	})
//	if errors.Is(err, circuitbreaker.ErrOpen) {
//		// dependency looks unhealthy; fall back or fail fast without retrying
//	}
package circuitbreaker

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/circuitbreaker/mock_circuitbreaker.go -package=mockcircuitbreaker github.com/biairmal/go-sdk/lib/circuitbreaker Breaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// State is a Breaker's current lifecycle stage.
type State int

const (
	// StateClosed lets calls through and counts failures toward the trip
	// conditions (FailureThreshold, FailureRatio).
	StateClosed State = iota
	// StateOpen rejects every call immediately with ErrOpen until
	// OpenTimeout elapses.
	StateOpen
	// StateHalfOpen admits up to HalfOpenMaxCalls concurrent probe calls to
	// test whether the dependency has recovered.
	StateHalfOpen
)

// String returns the state's name, suitable for log fields and metric
// labels (e.g. passed straight to metrics.Labels).
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// ErrOpen indicates Execute rejected a call without invoking fn, either
// because the breaker is Open or because it is Half-Open with no free probe
// slot. It wraps errorz.ErrServiceUnavailable so both
// errors.Is(err, circuitbreaker.ErrOpen) and
// errors.Is(err, errorz.ErrServiceUnavailable) hold, and the *errorz.Error
// Execute actually returns resolves to a clean 503 at the httpkit edge via
// StatusCodeFromError.
var ErrOpen = fmt.Errorf("circuitbreaker: breaker open: %w", errorz.ErrServiceUnavailable)

// wrapOpen builds the *errorz.Error returned to callers when Execute rejects
// a call because the breaker is open.
func wrapOpen() error {
	return errorz.Wrap(ErrOpen).WithCode(errorz.CodeServiceUnavailable).WithMessage(ErrOpen.Error())
}

// Breaker guards calls to an unreliable dependency with a closed→open→
// half-open state machine: while Closed it lets calls through and counts
// failures; once FailureThreshold or FailureRatio trips, it moves to Open
// and rejects every call immediately with ErrOpen until OpenTimeout elapses;
// it then moves to Half-Open, admitting a bounded number of probe calls to
// test recovery — a single probe failure reopens it, while enough
// consecutive probe successes close it. Implementations must be safe for
// concurrent use.
type Breaker interface {
	// Execute runs fn if the breaker currently admits calls, and records the
	// outcome against the state machine. It returns ErrOpen without calling
	// fn when the breaker is Open, or Half-Open with no free probe slot.
	// Otherwise it returns whatever fn returns. A panic from fn is recorded
	// as a failure and then re-raised — Execute never swallows a panic.
	Execute(ctx context.Context, fn func(ctx context.Context) error) error
	// State returns the breaker's current state, resolving a pending
	// Open→Half-Open transition (OpenTimeout has elapsed) as a side effect,
	// the same as Execute would.
	State() State
}

// Do runs fn through b and returns its value alongside the error, for calls
// that produce a result in addition to an error. Go doesn't support generic
// methods, so this is a free function wrapping Execute rather than a method
// on Breaker. On any error (including ErrOpen) the returned value is T's
// zero value.
func Do[T any](ctx context.Context, b Breaker, fn func(ctx context.Context) (T, error)) (T, error) {
	var result T
	err := b.Execute(ctx, func(ctx context.Context) error {
		v, err := fn(ctx)
		if err != nil {
			return err
		}
		result = v
		return nil
	})
	return result, err
}

// breaker is the default Breaker implementation: a mutex-guarded state
// machine with no third-party dependency.
type breaker struct {
	failureThreshold uint32
	failureRatio     float64
	openTimeout      time.Duration
	halfOpenMaxCalls uint32

	onStateChange func(from, to State)
	isSuccessful  func(err error) bool
	now           func() time.Time

	mu         sync.Mutex
	state      State
	generation uint64
	openedAt   time.Time
	counts     counts
}

// counts accumulates outcomes for the breaker's current generation (reset on
// every state transition). requests/totalFailures/consecutiveFailures apply
// while Closed; halfOpenInFlight/halfOpenSuccesses apply while Half-Open.
type counts struct {
	requests            uint32
	totalFailures       uint32
	consecutiveFailures uint32
	halfOpenInFlight    uint32
	halfOpenSuccesses   uint32
}

// transition describes a state change to report via onStateChange.
type transition struct {
	from, to State
}

// NewBreaker builds a Breaker from cfg. Zero-valued fields in cfg are filled
// from DefaultConfig(), so a zero Config is always usable; call
// cfg.Validate() yourself first if you need to reject a malformed config
// (e.g. one loaded from YAML) instead of silently defaulting around it.
func NewBreaker(cfg Config, opts ...Option) Breaker {
	cfg = withDefaults(cfg)
	b := &breaker{
		failureThreshold: safeUint32(cfg.FailureThreshold),
		failureRatio:     cfg.FailureRatio,
		openTimeout:      cfg.OpenTimeout,
		halfOpenMaxCalls: safeUint32(cfg.HalfOpenMaxCalls),
		isSuccessful:     defaultIsSuccessful,
		now:              time.Now,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// defaultIsSuccessful treats any non-nil error as a breaker failure.
func defaultIsSuccessful(err error) bool { return err == nil }

// safeUint32 clamps a possibly-negative int to a non-negative uint32.
// NewBreaker doesn't call Config.Validate itself (see its doc comment), so a
// caller-supplied negative field is defused here instead of wrapping around.
func safeUint32(n int) uint32 {
	if n < 0 {
		return 0
	}
	return uint32(n) //nolint:gosec // bounds-checked above
}
