package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
)

func newTestEncryptor(t *testing.T) *Encryptor {
	t.Helper()
	enc, err := New(validConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return enc
}

func TestNew_InvalidConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("New(Config{}) error = nil, want error")
	}
}

func TestEncryptor_EncryptDecrypt_RoundTrip(t *testing.T) {
	enc := newTestEncryptor(t)
	plaintext := []byte("jane@example.com")

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	got, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestEncryptor_Encrypt_NonDeterministic(t *testing.T) {
	enc := newTestEncryptor(t)
	plaintext := []byte("jane@example.com")

	a, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	b, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if a == b {
		t.Error("Encrypt() produced identical ciphertext for the same plaintext twice, want a fresh nonce each call")
	}
}

func TestEncryptor_Decrypt_WrongKeyFails(t *testing.T) {
	enc := newTestEncryptor(t)
	ciphertext, err := enc.Encrypt([]byte("jane@example.com"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	other, err := New(Config{EncryptionKey: keyB64(32, 0x09), BlindIndexKey: keyB64(33, 0x0A)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := other.Decrypt(ciphertext); !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("Decrypt() with wrong key error = %v, want ErrDecryptionFailed", err)
	}
}

func TestEncryptor_Decrypt_TamperedCiphertextFails(t *testing.T) {
	enc := newTestEncryptor(t)
	ciphertext, err := enc.Encrypt([]byte("jane@example.com"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	sealed, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	sealed[len(sealed)-1] ^= 0xFF // flip the last byte of the auth tag
	tampered := base64.StdEncoding.EncodeToString(sealed)

	if _, err := enc.Decrypt(tampered); !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("Decrypt() of tampered ciphertext error = %v, want ErrDecryptionFailed", err)
	}
}

func TestEncryptor_Decrypt_Malformed(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext string
	}{
		{name: "not base64", ciphertext: "not-valid-base64!!"},
		{name: "too short for a nonce", ciphertext: base64.StdEncoding.EncodeToString([]byte("x"))},
		{name: "empty string", ciphertext: ""},
	}

	enc := newTestEncryptor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := enc.Decrypt(tt.ciphertext); !errors.Is(err, ErrDecryptionFailed) {
				t.Errorf("Decrypt(%q) error = %v, want ErrDecryptionFailed", tt.ciphertext, err)
			}
		})
	}
}

func TestErrDecryptionFailed_WrapsErrInternal(t *testing.T) {
	if !errors.Is(ErrDecryptionFailed, errorz.ErrInternal) {
		t.Error("ErrDecryptionFailed does not wrap errorz.ErrInternal")
	}
}

func TestEncryptor_BlindIndex_Deterministic(t *testing.T) {
	enc := newTestEncryptor(t)

	a := enc.BlindIndex("jane@example.com")
	b := enc.BlindIndex("jane@example.com")
	if a != b {
		t.Errorf("BlindIndex() = %q and %q for the same value, want equal", a, b)
	}
}

func TestEncryptor_BlindIndex_DiffersByValue(t *testing.T) {
	enc := newTestEncryptor(t)

	a := enc.BlindIndex("jane@example.com")
	b := enc.BlindIndex("john@example.com")
	if a == b {
		t.Error("BlindIndex() produced the same output for two different values")
	}
}

func TestEncryptor_BlindIndex_DiffersByKey(t *testing.T) {
	enc1 := newTestEncryptor(t)
	enc2, err := New(Config{EncryptionKey: keyB64(32, 0x09), BlindIndexKey: keyB64(33, 0x0A)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a := enc1.BlindIndex("jane@example.com")
	b := enc2.BlindIndex("jane@example.com")
	if a == b {
		t.Error("BlindIndex() produced the same output under two different keys")
	}
}
