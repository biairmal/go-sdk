package circuitbreaker

import (
	"context"
	"time"
)

// Execute implements Breaker.
func (b *breaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	generation, trans, admitErr := b.before()
	b.notify(trans)
	if admitErr != nil {
		return admitErr
	}

	defer func() {
		if r := recover(); r != nil {
			b.notify(b.after(generation, false))
			panic(r)
		}
	}()

	err := fn(ctx)
	b.notify(b.after(generation, b.isSuccessful(err)))
	return err
}

// State implements Breaker.
func (b *breaker) State() State {
	b.mu.Lock()
	now := b.now()
	var trans *transition
	if b.state == StateOpen && now.Sub(b.openedAt) >= b.openTimeout {
		t := b.setStateLocked(StateHalfOpen, now)
		trans = &t
	}
	state := b.state
	b.mu.Unlock()

	b.notify(trans)
	return state
}

// before checks whether a call may proceed, lazily resolving an elapsed
// OpenTimeout into a Half-Open transition first. On admission it returns the
// generation the call was admitted under (for after to detect a state
// change that happened while the call was running) and reserves a Half-Open
// probe slot if applicable. On rejection it returns ErrOpen.
func (b *breaker) before() (generation uint64, trans *transition, err error) {
	b.mu.Lock()

	now := b.now()
	if b.state == StateOpen && now.Sub(b.openedAt) >= b.openTimeout {
		t := b.setStateLocked(StateHalfOpen, now)
		trans = &t
	}

	switch {
	case b.state == StateOpen:
		b.mu.Unlock()
		return 0, trans, wrapOpen()
	case b.state == StateHalfOpen && b.counts.halfOpenInFlight >= b.halfOpenMaxCalls:
		b.mu.Unlock()
		return 0, trans, wrapOpen()
	case b.state == StateHalfOpen:
		b.counts.halfOpenInFlight++
	}

	generation = b.generation
	b.mu.Unlock()
	return generation, trans, nil
}

// after records a call's outcome against generation, ignoring it if the
// breaker has since moved on to a new generation (a state transition
// happened while the call was in flight, e.g. another concurrent Half-Open
// probe already reopened the breaker).
func (b *breaker) after(generation uint64, success bool) *transition {
	b.mu.Lock()

	if generation != b.generation {
		b.mu.Unlock()
		return nil
	}

	var trans *transition
	switch b.state {
	case StateClosed:
		trans = b.onClosedResultLocked(success)
	case StateHalfOpen:
		trans = b.onHalfOpenResultLocked(success)
	}

	b.mu.Unlock()
	return trans
}

// onClosedResultLocked updates the Closed-state counts and trips the
// breaker to Open if the failure threshold or ratio is reached. Caller must
// hold b.mu.
func (b *breaker) onClosedResultLocked(success bool) *transition {
	b.counts.requests++
	if success {
		b.counts.consecutiveFailures = 0
		return nil
	}

	b.counts.totalFailures++
	b.counts.consecutiveFailures++
	if !b.shouldTripLocked() {
		return nil
	}
	t := b.setStateLocked(StateOpen, b.now())
	return &t
}

// shouldTripLocked reports whether the Closed-state counts have crossed
// FailureThreshold (consecutive failures) or FailureRatio (once at least
// FailureThreshold requests have been observed). Caller must hold b.mu.
func (b *breaker) shouldTripLocked() bool {
	if b.counts.consecutiveFailures >= b.failureThreshold {
		return true
	}
	if b.counts.requests < b.failureThreshold {
		return false
	}
	ratio := float64(b.counts.totalFailures) / float64(b.counts.requests)
	return ratio >= b.failureRatio
}

// onHalfOpenResultLocked frees the probe slot the call reserved, then either
// reopens the breaker on failure or closes it once HalfOpenMaxCalls
// consecutive probes have succeeded. Caller must hold b.mu.
func (b *breaker) onHalfOpenResultLocked(success bool) *transition {
	if b.counts.halfOpenInFlight > 0 {
		b.counts.halfOpenInFlight--
	}

	if !success {
		t := b.setStateLocked(StateOpen, b.now())
		return &t
	}

	b.counts.halfOpenSuccesses++
	if b.counts.halfOpenSuccesses < b.halfOpenMaxCalls {
		return nil
	}
	t := b.setStateLocked(StateClosed, b.now())
	return &t
}

// setStateLocked moves the breaker to state to, resetting its counts and
// bumping the generation so in-flight calls from the previous state are
// recognized as stale by after. Caller must hold b.mu.
func (b *breaker) setStateLocked(to State, now time.Time) transition {
	from := b.state
	b.state = to
	b.generation++
	b.counts = counts{}
	if to == StateOpen {
		b.openedAt = now
	}
	return transition{from: from, to: to}
}

// notify invokes onStateChange for a real transition, outside b.mu so the
// callback may safely call back into the breaker (e.g. State()).
func (b *breaker) notify(t *transition) {
	if t == nil || b.onStateChange == nil || t.from == t.to {
		return
	}
	b.onStateChange(t.from, t.to)
}
