package circuitbreaker

import "testing"

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg:  DefaultConfig(),
		},
		{
			name:    "non-positive failure_threshold",
			cfg:     Config{FailureThreshold: 0, FailureRatio: 0.5, OpenTimeout: sec, HalfOpenMaxCalls: 1},
			wantErr: true,
		},
		{
			name:    "failure_ratio zero",
			cfg:     Config{FailureThreshold: 5, FailureRatio: 0, OpenTimeout: sec, HalfOpenMaxCalls: 1},
			wantErr: true,
		},
		{
			name:    "failure_ratio above one",
			cfg:     Config{FailureThreshold: 5, FailureRatio: 1.5, OpenTimeout: sec, HalfOpenMaxCalls: 1},
			wantErr: true,
		},
		{
			name:    "non-positive open_timeout",
			cfg:     Config{FailureThreshold: 5, FailureRatio: 0.5, OpenTimeout: 0, HalfOpenMaxCalls: 1},
			wantErr: true,
		},
		{
			name:    "non-positive half_open_max_calls",
			cfg:     Config{FailureThreshold: 5, FailureRatio: 0.5, OpenTimeout: sec, HalfOpenMaxCalls: 0},
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

func TestDefaultConfig_IsValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() = %v, want nil", err)
	}
}

func TestNewBreaker_ZeroConfigUsesDefaults(t *testing.T) {
	b := NewBreaker(Config{})
	if got := b.State(); got != StateClosed {
		t.Fatalf("State() = %v, want Closed", got)
	}
}
