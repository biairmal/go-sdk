# Queue Package

`queue` abstracts async message publishing and consuming behind swappable `Publisher`/`Subscriber` interfaces.

## Overview

Async, fire-and-forget message publishing for callers that need to hand off a message for later delivery without waiting on a broker round trip, plus a blocking consume-loop abstraction for reading it back. Consumers depend on `queue.Publisher`/`queue.Subscriber` and `queue.Config` only — never on a specific broker client — so swapping backends (or faking one in tests) is a config change, not a code change. `Publisher` ships three backends: `NewNoOp` (discards), `NewLogging` (logs — a stand-in for a real broker in dev/tests), and `NewKafka` (publishes through a [`kafka.Client`](../kafka/README.md)). `Subscriber` ships one: `NewKafkaSubscriber` (consumes through a [`kafka.Consumer`](../kafka/README.md)) — see Limitations for why there's no `NoOp`/`Logging` subscriber.

## Features

- **Swappable backend, one interface per direction**: `Publisher.Publish(ctx, topic, message, opts...) error` and `Subscriber.Subscribe(ctx, topic, handler) error` — same signature regardless of backend.
- **Backend selection by config**: `Config.Backend` (`"noop" | "logging" | "kafka"`) + `FromConfig` build the right `Publisher`, mirroring `ratelimit.FromConfig`.
- **Cross-backend per-message options**: `WithKey` (ordering key) and `WithHeaders` (metadata) — only options with a consistent meaning across backends live here; backend-only tuning (ack level, compression, ...) belongs in that backend's own config.
- **At-least-once consume, manual commit**: `Subscribe`'s `Handler` returning an error stops the call with that message uncommitted, so a restart redelivers it. No in-process retry/backoff/DLQ — see Limitations.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/queue
```

### Basic usage

```go
package main

import (
    "context"

    "github.com/biairmal/go-sdk/lib/queue"
)

func main() {
    pub := queue.NewNoOp()
    _ = pub.Publish(context.Background(), "orders.created", []byte(`{"order_id":"123"}`),
        queue.WithKey([]byte("123")),
    )
}
```

### Subscribing

```go
package main

import (
    "context"

    "github.com/biairmal/go-sdk/lib/kafka"
    "github.com/biairmal/go-sdk/lib/queue"
)

func main() {
    sub := queue.NewKafkaSubscriber(&kafka.Config{
        Brokers: []string{"localhost:9092"},
        Auth:    kafka.AuthConfig{Mechanism: kafka.MechanismNone},
    }, "orders-service")

    err := sub.Subscribe(context.Background(), "orders.created", func(ctx context.Context, msg queue.Message) error {
        // process msg.Value; a non-nil return leaves it uncommitted (redelivered on restart)
        return nil
    })
    _ = err
}
```

### From config

```go
pub, err := queue.FromConfig(&cfg, log, kafkaClient) // log/kafkaClient only required by their matching backend
```

## Options

| Option | Description |
|--------|-------------|
| `WithKey(key []byte)` | Ordering key (Kafka partition key, ...). Ignored by backends without ordering. |
| `WithHeaders(headers map[string]string)` | Per-message metadata headers. |

## Limitations

- **No `NoOp`/`Logging` `Subscriber`.** `Publisher.NewNoOp` lets a caller "publish to nowhere" — a meaningful no-op. A subscriber with nothing to consume from isn't: there's no message source to fake, so build one only if a concrete test need shows up.
- **No retry/backoff/DLQ.** A handler error stops `Subscribe` with that message uncommitted; the caller restarts it (redelivering from the last commit) — crash-and-resume is the deliberate v1 strategy, not an oversight. See [DEVELOPMENT_PLAN.md Phase 11](../../docs/DEVELOPMENT_PLAN.md).
- **No multi-topic single-group fan-in** — one `Subscribe` call per topic (each backed by its own `kafka.Consumer`).
- **No delivery-guarantee tuning** (acks, batching, compression, retries, min/max bytes) exposed yet — the underlying `kafka.Client`/`kafka.Consumer` defaults apply.

## Dependencies

None directly beyond this SDK's own [`kafka`](../kafka/README.md) and [`logger`](../logger/README.md) packages.

## See also

- [kafka](../kafka/README.md) – connection + auth this package's Kafka backend wraps.
- [ratelimit](../ratelimit/README.md) – same interface-over-a-connection-package shape, with `redis`.
