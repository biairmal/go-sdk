package queue

import "context"

// Message is one delivery handed to a Subscriber's Handler.
type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

// Handler processes one Message. A nil error commits it (advances the
// offset, for backends with that concept); a non-nil error stops the owning
// Subscribe call with the message uncommitted, so it can be redelivered.
type Handler func(ctx context.Context, msg Message) error

// Subscriber abstracts a blocking consume loop behind a swappable backend.
// Implementations must be safe for concurrent use across different topics.
type Subscriber interface {
	// Subscribe blocks, delivering messages from topic to handler one at a
	// time, until ctx is done or an unrecoverable backend error occurs.
	Subscribe(ctx context.Context, topic string, handler Handler) error
}
