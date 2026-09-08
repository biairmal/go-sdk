# Kafka Package

`kafka` wraps connection and authentication to an Apache Kafka cluster. A `Client` built from `Config` produces messages to any topic.

## Overview

This package owns connection and auth only — no messaging semantics, no topic abstraction beyond passing the name through. `queue` is the layer above that exposes a swappable `Publisher` interface; `queue.NewKafka` wraps a `*kafka.Client` the same way `ratelimit.NewRedis` wraps a `redis.Client`.

## Features

- **Config-driven auth**: `none`, `plain`, `scram-sha256`, `scram-sha512`, or `mtls`, selected by `AuthConfig.Mechanism` — one active mechanism per `Config`, like `auth.Config`'s mode switch. Built once per `Client` into a shared `*kafka.Transport`.
- **SASL-over-TLS**: setting `tls.ca_file` alongside a SASL mechanism connects over TLS with a private CA, without switching to `mtls`.
- **Producer only**: models `segmentio/kafka-go`'s `kafka.Writer`. A consumer, if ever needed, is a new `kafka.Reader`-backed type in this same package, reusing the same auth/TLS building code (`buildSASLMechanism`/`buildTLSConfig`).
- **One `Client`, any topic**: `Produce` takes `topic` per call — the underlying `Writer` isn't pinned to one topic at construction.

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

- **No consumer/`Reader`.** Producer-only; see Features.
- **No OAUTHBEARER / Kerberos / AWS MSK IAM.** Not built into `segmentio/kafka-go`'s `sasl` package — adding one is a new `case` in `buildSASLMechanism`, not a redesign.
- **No delivery-guarantee tuning exposed.** `kafka.Writer`'s defaults apply (acks, batching, compression, retries); add as `Config` fields when a concrete reliability requirement shows up.

## Dependencies

- [github.com/segmentio/kafka-go](https://github.com/segmentio/kafka-go) – producer (`kafka.Writer`), `Transport`, `sasl/plain`, `sasl/scram`.

## See also

- [queue](../queue/README.md) – the `Publisher` interface consumers use instead of this package directly.
- [ratelimit](../ratelimit/README.md) – same connection-package-behind-an-interface shape, with `redis`.
