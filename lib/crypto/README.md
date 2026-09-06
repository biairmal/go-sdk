# Crypto Package

`crypto` encrypts field values at rest with AES-256-GCM and provides a deterministic HMAC-SHA256 `BlindIndex` so an encrypted column can still be looked up by exact match.

## Overview

Some columns (email, phone, other PII) need to be unreadable in the database but still searchable by exact value. AES-GCM alone can't do that: it's semantically secure, so encrypting the same plaintext twice produces different ciphertext, and `WHERE col = encrypt(?)` can never match. `crypto` solves this with two keyed primitives sharing one `Encryptor`: `Encrypt`/`Decrypt` for the ciphertext column, and `BlindIndex` — a deterministic HMAC-SHA256, stored in a companion `_hash` column — for the equality lookup. There's exactly one implementation (no swappable backend, no `NoOp`, no mock): AES-256-GCM and HMAC-SHA256 aren't configuration choices this package exposes.

## Features

- **Authenticated encryption**: AES-256-GCM via the standard library only — no third-party crypto dependency. A random nonce is generated per call and travels with the ciphertext, so `Encrypt` never needs separate nonce storage.
- **Tamper detection**: GCM's authentication tag makes a corrupted or forged ciphertext fail `Decrypt` with `ErrDecryptionFailed` rather than silently returning garbage.
- **Deterministic blind index for exact-match lookup**: `BlindIndex` is a separate, deterministic primitive (HMAC-SHA256) so equality search works without ever comparing ciphertext or indexing plaintext.
- **Enforced key separation**: `Config.Validate()` rejects a config where the encryption key and the blind-index key are the same value — they serve different purposes and must not be reused.
- **One validated `Encryptor` per key pair**: `New` decodes and checks both keys once; `Encrypt`/`Decrypt`/`BlindIndex` then just take the value, so there's no way to accidentally pass the wrong key on a given call.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/crypto
```

### Basic usage

```go
package main

import (
    "fmt"

    "github.com/biairmal/go-sdk/lib/crypto"
)

func main() {
    cfg := crypto.Config{
        EncryptionKey: encryptionKeyB64, // base64, decodes to 32 bytes — from a secret store
        BlindIndexKey: blindIndexKeyB64, // base64, decodes to >=16 bytes — distinct from EncryptionKey
    }
    enc, err := crypto.New(cfg)
    if err != nil {
        panic(err)
    }

    ciphertext, err := enc.Encrypt([]byte("jane@example.com"))
    if err != nil {
        panic(err)
    }
    hash := enc.BlindIndex("jane@example.com") // normalize the value yourself first if needed

    // ... store ciphertext in `email`, hash in `email_hash`

    plaintext, err := enc.Decrypt(ciphertext)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(plaintext))
}
```

### Exact-match lookup on an encrypted column

```go
// Normalization is the caller's business rule — crypto only hashes what it's given.
normalized := strings.ToLower(strings.TrimSpace(email))
row := db.QueryRowContext(ctx, `SELECT id, email FROM guests WHERE email_hash = $1`, enc.BlindIndex(normalized))
```

### Generating key material

```bash
# 32 random bytes, base64-encoded — one for EncryptionKey, a second (distinct) one for BlindIndexKey
openssl rand -base64 32
```

## Limitations

- **No key rotation.** One `Encryptor` holds exactly one active key pair. Rotating means re-encrypting existing rows under a new `Encryptor` at the application layer; there's no "old vs new key" concept here.
- **No KMS/HSM/secret-manager integration.** `Config` takes raw base64 key bytes — sourcing them securely (env var, Vault, a secrets manager) is the consumer's job, same as any other secret this SDK's packages take (e.g. `auth.hs256_secret`).
- **Not for large payloads.** Sized for short field values (names, emails, phone numbers). `Encrypt`/`Decrypt` hold the whole plaintext/ciphertext in memory — no streaming/chunking.
- **`BlindIndex` only supports exact match.** A hash has no notion of "starts with" or "contains" — partial/fuzzy search on an encrypted column isn't possible through this package.
- **`DefaultConfig()` is not valid on its own.** Unlike most packages in this SDK, there's no safe default: both keys are required secrets, so `DefaultConfig().Validate()` deliberately returns an error until real key material is supplied.

## Dependencies

None beyond the standard library (`crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/sha256`, `crypto/rand`). Uses `errorz` (foundational leaf) for error codes.

## See also

- [errorz](../errorz/README.md) – the error type `ErrDecryptionFailed` wraps.
- [config](../config/README.md) – loads `crypto.Config` from YAML/env alongside the rest of an app's configuration.
