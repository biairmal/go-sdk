# Queue Package

`queue` abstracts async, fire-and-forget message publishing behind a swappable `Publisher` interface.

## Overview

Async, fire-and-forget message publishing for callers that need to hand off a message for later delivery without waiting on a broker round trip. Consumers depend on `queue.Publisher` and `queue.Config` only — never on a specific broker client — so swapping backends (or faking one in tests) is a config change, not a code change. Three backends ship: `NewNoOp` (discards), `NewLogging` (logs — a stand-in for a real broker in dev/tests), and `NewKafka` (publishes through a [`kafka.Client`](../kafka/README.md)).

## Features

- **Swappable backend, one interface**: `Publish(ctx, topic, message, opts...) error` — same signature regardless of backend.
- **Backend selection by config**: `Config.Backend` (`"noop" | "logging" | "kafka"`) + `FromConfig` build the right `Publisher`, mirroring `ratelimit.FromConfig`.
- **Cross-backend per-message options**: `WithKey` (ordering key) and `WithHeaders` (metadata) — only options with a consistent meaning across backends live here; backend-only tuning (ack level, compression, ...) belongs in that backend's own config.

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

- **No consumer/`Subscriber`.** Producer-only, matching `kafka`'s scope; added when a real consumer use case exists.
- **No delivery-guarantee tuning** (acks, batching, compression, retries/DLQ) exposed yet — the underlying `kafka.Client`'s defaults apply.

## Dependencies

None directly beyond this SDK's own [`kafka`](../kafka/README.md) and [`logger`](../logger/README.md) packages.

## See also

- [kafka](../kafka/README.md) – connection + auth this package's Kafka backend wraps.
- [ratelimit](../ratelimit/README.md) – same interface-over-a-connection-package shape, with `redis`.
