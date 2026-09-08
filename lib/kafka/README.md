# Kafka Package

`kafka` wraps connection and authentication to an Apache Kafka cluster. A `Client` produces messages to any topic; a `Consumer` reads from one topic under one consumer group.

## Overview

This package owns connection and auth only — no messaging semantics, no topic abstraction beyond passing the name through. `queue` is the layer above that exposes swappable `Publisher`/`Subscriber` interfaces; `queue.NewKafka`/`queue.NewKafkaSubscriber` wrap `kafka` types the same way `ratelimit.NewRedis` wraps a `redis.Client`.

## Features

- **Config-driven auth**: `none`, `plain`, `scram-sha256`, `scram-sha512`, or `mtls`, selected by `AuthConfig.Mechanism` — one active mechanism per `Config`, like `auth.Config`'s mode switch. Built once per `Client`/`Consumer` via the shared `buildSASLMechanism`/`buildTLSConfig`, into a `*kafka.Transport` (producer) or `*kafka.Dialer` (consumer).
- **SASL-over-TLS**: setting `tls.ca_file` alongside a SASL mechanism connects over TLS with a private CA, without switching to `mtls`.
- **Producer, any topic**: `Client.Produce` takes `topic` per call — the underlying `Writer` isn't pinned to one topic at construction.
- **Consumer, one topic per group**: `Consumer` is bound to one `(groupID, topic)` at construction — `kafka.Reader`'s own model. `Consume` runs a blocking Fetch → handler → Commit loop, one message at a time, in partition order.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/kafka
```

### Basic usage

```go
package main

import (
    "context"

    "github.com/biairmal/go-sdk/lib/kafka"
)

func main() {
    cfg := &kafka.Config{
        Brokers: []string{"localhost:9092"},
        Auth:    kafka.AuthConfig{Mechanism: kafka.MechanismNone},
    }
    client, err := kafka.New(cfg)
    if err != nil {
        panic(err)
    }
    defer client.Close()

    err = client.Produce(context.Background(), "orders.created", []byte("order-123"), []byte(`{"id":123}`), nil)
    if err != nil {
        panic(err)
    }
}
```

### Consuming

```go
package main

import (
    "context"

    "github.com/biairmal/go-sdk/lib/kafka"
)

func main() {
    cfg := &kafka.Config{
        Brokers: []string{"localhost:9092"},
        Auth:    kafka.AuthConfig{Mechanism: kafka.MechanismNone},
    }
    consumer, err := kafka.NewConsumer(cfg, "orders-service", "orders.created")
    if err != nil {
        panic(err)
    }
    defer consumer.Close()

    err = consumer.Consume(context.Background(), func(ctx context.Context, msg kafka.Message) error {
        // process msg.Value; a non-nil return leaves it uncommitted (redelivered on restart)
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

### SCRAM auth

```go
cfg := &kafka.Config{
    Brokers: []string{"broker:9093"},
    Auth: kafka.AuthConfig{
        Mechanism: kafka.MechanismScramSHA512,
        Username:  "app",
        Password:  "secret",
        TLS:       kafka.TLSConfig{CAFile: "/etc/kafka/ca.pem"},
    },
}
```

### mTLS

```go
cfg := &kafka.Config{
    Brokers: []string{"broker:9093"},
    Auth: kafka.AuthConfig{
        Mechanism: kafka.MechanismMTLS,
        TLS: kafka.TLSConfig{
            CertFile: "/etc/kafka/client-cert.pem",
            KeyFile:  "/etc/kafka/client-key.pem",
            CAFile:   "/etc/kafka/ca.pem",
        },
    },
}
```

## Limitations

- **No retry/backoff/DLQ on the consumer side.** A handler error stops `Consume` with that message uncommitted; the caller decides whether to restart (redelivering from the last commit). This is deliberate — see [DEVELOPMENT_PLAN.md Phase 11](../../docs/DEVELOPMENT_PLAN.md) for the reasoning.
- **No multi-topic single-group fan-in.** One `Consumer` = one topic; subscribe to several topics with several `Consumer`s.
- **No OAUTHBEARER / Kerberos / AWS MSK IAM.** Not built into `segmentio/kafka-go`'s `sasl` package — adding one is a new `case` in `buildSASLMechanism`, not a redesign.
- **No delivery-guarantee tuning exposed.** `kafka.Writer`/`kafka.Reader` defaults apply (acks, batching, compression, retries, min/max bytes, max wait); add as `Config` fields when a concrete reliability requirement shows up.

## Dependencies

- [github.com/segmentio/kafka-go](https://github.com/segmentio/kafka-go) – producer (`kafka.Writer`), `Transport`, `sasl/plain`, `sasl/scram`.

## See also

- [queue](../queue/README.md) – the `Publisher` interface consumers use instead of this package directly.
- [ratelimit](../ratelimit/README.md) – same connection-package-behind-an-interface shape, with `redis`.
