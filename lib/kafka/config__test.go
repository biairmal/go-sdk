package kafka

import "testing"

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid none",
			cfg:  Config{Brokers: []string{"localhost:9092"}, Auth: AuthConfig{Mechanism: MechanismNone}},
		},
		{
			name: "valid plain",
			cfg: Config{
				Brokers: []string{"localhost:9092"},
				Auth:    AuthConfig{Mechanism: MechanismPlain, Username: "u", Password: "p"},
			},
		},
		{
			name: "valid scram-sha256",
			cfg: Config{
				Brokers: []string{"localhost:9092"},
				Auth:    AuthConfig{Mechanism: MechanismScramSHA256, Username: "u", Password: "p"},
			},
		},
		{
			name: "valid mtls",
			cfg: Config{
				Brokers: []string{"localhost:9092"},
				Auth: AuthConfig{
					Mechanism: MechanismMTLS,
					TLS:       TLSConfig{CertFile: "c", KeyFile: "k", CAFile: "ca"},
				},
			},
		},
		{
			name:    "no brokers",
			cfg:     Config{Auth: AuthConfig{Mechanism: MechanismNone}},
			wantErr: true,
		},
		{
			name:    "unknown mechanism",
			cfg:     Config{Brokers: []string{"localhost:9092"}, Auth: AuthConfig{Mechanism: "bogus"}},
			wantErr: true,
		},
		{
			name:    "plain missing credentials",
			cfg:     Config{Brokers: []string{"localhost:9092"}, Auth: AuthConfig{Mechanism: MechanismPlain}},
			wantErr: true,
		},
		{
			name:    "mtls missing tls paths",
			cfg:     Config{Brokers: []string{"localhost:9092"}, Auth: AuthConfig{Mechanism: MechanismMTLS}},
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

func TestDefaultConfig_RequiresBrokers(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Fatal("DefaultConfig().Validate() = nil, want an error (brokers must be set)")
	}
}
