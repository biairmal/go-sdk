// Package crypto provides field-level authenticated encryption and deterministic blind indexing
// for values that must be encrypted at rest but still support exact-match lookup — e.g. PII
// columns like email/phone. Encrypt/Decrypt use AES-256-GCM with a random nonce per call, so
// ciphertext is never comparable across calls; BlindIndex is a deterministic HMAC-SHA256 over a
// caller-normalized value, stored alongside the ciphertext to support equality lookups without
// ever indexing plaintext.
//
// There is exactly one implementation — no swappable backend, no NoOp, no mock — because there
// is nothing to swap: AES-256-GCM and HMAC-SHA256 aren't algorithm choices this package exposes
// as configuration.
//
// Example usage:
//
//	enc, err := crypto.New(crypto.Config{EncryptionKey: encKeyB64, BlindIndexKey: blindKeyB64})
//	ciphertext, err := enc.Encrypt([]byte("jane@example.com"))
//	plaintext, err := enc.Decrypt(ciphertext)
//	hash := enc.BlindIndex("jane@example.com") // store alongside ciphertext for exact-match lookup
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// ErrDecryptionFailed indicates Decrypt could not recover the plaintext: a wrong key, corrupted
// or truncated ciphertext, or a failed authentication tag (tamper detection). It wraps
// errorz.ErrInternal since a bad key or forged ciphertext reaching Decrypt is a data-integrity
// fault, not a client input problem.
var ErrDecryptionFailed = fmt.Errorf("crypto: decryption failed: %w", errorz.ErrInternal)

// Encryptor performs authenticated field encryption (AES-256-GCM) and deterministic blind
// indexing (HMAC-SHA256) over one validated key pair. Safe for concurrent use.
type Encryptor struct {
	aead     cipher.AEAD
	blindKey []byte
}

// New validates cfg and builds an Encryptor from its decoded keys.
func New(cfg Config) (*Encryptor, error) {
	encKey, blindKey, err := decodedKeys(cfg)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("crypto: build AES cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("crypto: build GCM")
	}

	return &Encryptor{aead: aead, blindKey: blindKey}, nil
}

// Encrypt returns a base64-encoded, randomly-nonced AES-256-GCM ciphertext of plaintext.
// Encrypting the same plaintext twice yields different output — semantic security by design, not
// a bug — so ciphertext can never be compared for equality; use BlindIndex for that.
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("crypto: generate nonce")
	}
	sealed := e.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt, returning ErrDecryptionFailed if ciphertext isn't valid base64, is
// too short to contain a nonce, or fails GCM authentication (wrong key or tampered data).
func (e *Encryptor) Decrypt(ciphertext string) ([]byte, error) {
	sealed, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, wrapDecryptErr(err)
	}

	nonceSize := e.aead.NonceSize()
	if len(sealed) < nonceSize {
		return nil, wrapDecryptErr(nil)
	}
	nonce, sealedCiphertext := sealed[:nonceSize], sealed[nonceSize:]

	plaintext, err := e.aead.Open(nil, nonce, sealedCiphertext, nil)
	if err != nil {
		return nil, wrapDecryptErr(err)
	}
	return plaintext, nil
}

// wrapDecryptErr builds the *errorz.Error returned by Decrypt on any failure, preserving cause
// when there is one. A fresh *errorz.Error per call (rather than a single package-level value)
// avoids callers' With* calls on one failure leaking Meta into an unrelated one.
func wrapDecryptErr(cause error) error {
	if cause == nil {
		return errorz.Wrap(ErrDecryptionFailed).WithCode(errorz.CodeInternal).WithMessage(ErrDecryptionFailed.Error())
	}
	return errorz.Wrap(fmt.Errorf("%w: %w", ErrDecryptionFailed, cause)).
		WithCode(errorz.CodeInternal).WithMessage(ErrDecryptionFailed.Error())
}

// BlindIndex returns a deterministic, hex-encoded HMAC-SHA256 of value, for exact-match lookups
// against an encrypted column. Same value + same key always produce the same output. Callers
// normalize value (case-folding, trimming, digits-only phone, ...) before calling — that's the
// consuming feature's business rule, not this package's concern.
func (e *Encryptor) BlindIndex(value string) string {
	mac := hmac.New(sha256.New, e.blindKey)
	_, _ = mac.Write([]byte(value)) // hash.Hash.Write never errors
	return hex.EncodeToString(mac.Sum(nil))
}
