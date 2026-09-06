package crypto

import (
	"encoding/base64"

	"github.com/biairmal/go-sdk/lib/errorz"
)

const (
	// encryptionKeySize is the required decoded length of Config.EncryptionKey: 32 bytes for AES-256.
	encryptionKeySize = 32
	// minBlindIndexKeySize is the minimum decoded length of Config.BlindIndexKey, an HMAC-SHA256 key.
	minBlindIndexKeySize = 16
)

// Config holds the key material for field encryption and blind indexing. Both fields are
// standard-base64-encoded; source real values from a secret store, never commit them.
type Config struct {
	// EncryptionKey is a standard-base64-encoded AES-256-GCM key. Must decode to exactly 32 bytes.
	EncryptionKey string `mapstructure:"encryption_key"`
	// BlindIndexKey is a standard-base64-encoded HMAC-SHA256 key, distinct from EncryptionKey.
	// Must decode to at least 16 bytes.
	BlindIndexKey string `mapstructure:"blind_index_key"`
}

// DefaultConfig returns a zero Config. Unlike most packages in this SDK, there is no safe
// default here — both keys are secrets that must be supplied explicitly (e.g. via env through
// config.Load), so DefaultConfig().Validate() deliberately returns an error.
func DefaultConfig() Config {
	return Config{}
}

// Validate checks that both keys are present, base64-decodable, correctly sized, and distinct
// from each other. Key separation (distinct encryption and blind-index keys) is enforced here,
// not just documented, since an encryption key and a MAC key serve different security purposes
// and must not be reused.
func (c Config) Validate() error {
	_, _, err := decodedKeys(c)
	return err
}

// decodedKeys validates c the same way Validate does, additionally returning the decoded key
// bytes so New doesn't have to re-decode (and re-ignore an already-checked error) afterward.
func decodedKeys(c Config) (encKey, blindKey []byte, err error) {
	encKey, err = decodeKey(c.EncryptionKey, "encryption_key")
	if err != nil {
		return nil, nil, err
	}
	if len(encKey) != encryptionKeySize {
		return nil, nil, errorz.BadRequest().WithMessage("crypto: encryption_key must decode to 32 bytes (AES-256)")
	}

	blindKey, err = decodeKey(c.BlindIndexKey, "blind_index_key")
	if err != nil {
		return nil, nil, err
	}
	if len(blindKey) < minBlindIndexKeySize {
		return nil, nil, errorz.BadRequest().WithMessage("crypto: blind_index_key must decode to at least 16 bytes")
	}

	if c.EncryptionKey == c.BlindIndexKey {
		return nil, nil, errorz.BadRequest().WithMessage("crypto: encryption_key and blind_index_key must be distinct")
	}
	return encKey, blindKey, nil
}

// decodeKey base64-decodes one Config key field, wrapping a decode failure with the field name.
func decodeKey(encoded, field string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeBadRequest).WithMessage("crypto: " + field + " is not valid base64")
	}
	return key, nil
}
