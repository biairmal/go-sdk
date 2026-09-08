package queue

import "context"

// noOp discards every message. Useful when a topic isn't wired up yet, or in
// tests where publishing is beside the point.
type noOp struct{}

// NewNoOp builds a Publisher that discards every message and always returns
// nil.
func NewNoOp() Publisher {
	return noOp{}
}

// Publish discards message and returns nil.
func (noOp) Publish(_ context.Context, _ string, _ []byte, _ ...PublishOption) error {
	return nil
}
