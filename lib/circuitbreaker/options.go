package circuitbreaker

import "time"

// Option configures NewBreaker with optional, non-serializable behavior.
type Option func(*breaker)

// WithOnStateChange registers a callback invoked after every real state
// transition (Closed↔Open↔Half-Open), outside the breaker's internal lock —
// it's safe for fn to call back into the same Breaker (e.g. State()). Use it
// to emit metrics.Recorder gauges or log transitions. Nil-safe: a nil fn
// (the default) disables the callback.
func WithOnStateChange(fn func(from, to State)) Option {
	return func(b *breaker) {
		b.onStateChange = fn
	}
}

// WithIsSuccessful overrides how Execute classifies fn's returned error as a
// breaker success or failure. The default treats any non-nil error as a
// failure; override this to, for example, ignore context.Canceled or treat
// only certain errorz codes (e.g. CodeServiceUnavailable/CodeInternal, but
// not CodeBadRequest) as breaker-worthy so client errors don't trip it.
// Nil-safe: a nil fn leaves the default in place.
func WithIsSuccessful(fn func(err error) bool) Option {
	return func(b *breaker) {
		if fn != nil {
			b.isSuccessful = fn
		}
	}
}

// WithClock overrides the breaker's time source. Intended for deterministic
// tests of OpenTimeout expiry; production callers should leave this unset
// (it defaults to time.Now). Nil-safe: a nil now leaves the default in place.
func WithClock(now func() time.Time) Option {
	return func(b *breaker) {
		if now != nil {
			b.now = now
		}
	}
}
