package crypto

import (
	"encoding/base64"
	"testing"
)

// keyB64 builds n bytes of value fill, base64-encoded — a deterministic stand-in for real key
// material, sized and distinguished the way tests need without hardcoding literals.
func keyB64(n int, fill byte) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = fill
	}
	return base64.StdEncoding.EncodeToString(b)
}

// validConfig returns a Config that passes Validate(): a 32-byte encryption key and a distinct
// 33-byte blind-index key.
func validConfig() Config {
	return Config{
		EncryptionKey: keyB64(encryptionKeySize, 0x01),
		BlindIndexKey: keyB64(encryptionKeySize+1, 0x02),
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "valid config", cfg: validConfig()},
		{name: "empty config", cfg: Config{}, wantErr: true},
		{
			name:    "invalid base64 encryption key",
			cfg:     Config{EncryptionKey: "not-base64!!", BlindIndexKey: keyB64(32, 0x02)},
			wantErr: true,
		},
		{
			name:    "wrong-size encryption key",
			cfg:     Config{EncryptionKey: keyB64(16, 0x01), BlindIndexKey: keyB64(32, 0x02)},
			wantErr: true,
		},
		{
			name:    "invalid base64 blind index key",
			cfg:     Config{EncryptionKey: keyB64(32, 0x01), BlindIndexKey: "not-base64!!"},
			wantErr: true,
		},
		{
			name:    "too-short blind index key",
			cfg:     Config{EncryptionKey: keyB64(32, 0x01), BlindIndexKey: keyB64(8, 0x02)},
			wantErr: true,
		},
		{
			name: "keys not distinct",
			cfg: Config{
				EncryptionKey: keyB64(32, 0x01),
				BlindIndexKey: keyB64(32, 0x01),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig_IsInvalid(t *testing.T) {
	if err := DefaultConfig().Validate(); err == nil {
		t.Fatal("DefaultConfig().Validate() = nil, want error (keys are required secrets, no safe default)")
	}
}
