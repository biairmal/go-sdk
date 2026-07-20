package ratelimit

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid memory config",
			cfg:  Config{Backend: BackendMemory, Rate: 10, Burst: 20, MaxKeys: 100},
		},
		{
			name: "valid redis config",
			cfg:  Config{Backend: BackendRedis, Burst: 20, Window: time.Second},
		},
		{
			name:    "unknown backend",
			cfg:     Config{Backend: "bogus", Burst: 20},
			wantErr: true,
		},
		{
			name:    "non-positive burst",
			cfg:     Config{Backend: BackendMemory, Rate: 10, Burst: 0, MaxKeys: 100},
			wantErr: true,
		},
		{
			name:    "memory backend needs positive rate",
			cfg:     Config{Backend: BackendMemory, Rate: 0, Burst: 20, MaxKeys: 100},
			wantErr: true,
		},
		{
			name:    "memory backend needs positive max_keys",
			cfg:     Config{Backend: BackendMemory, Rate: 10, Burst: 20, MaxKeys: 0},
			wantErr: true,
		},
		{
			name:    "redis backend needs positive window",
			cfg:     Config{Backend: BackendRedis, Burst: 20, Window: 0},
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

func TestFromConfig(t *testing.T) {
	t.Run("memory backend", func(t *testing.T) {
		l, err := FromConfig(Config{Backend: BackendMemory, Rate: 10, Burst: 20, MaxKeys: 100}, nil)
		if err != nil {
			t.Fatalf("FromConfig() error = %v, want nil", err)
		}
		if _, ok := l.(*memoryLimiter); !ok {
			t.Fatalf("FromConfig() type = %T, want *memoryLimiter", l)
		}
	})

	t.Run("redis backend requires a client", func(t *testing.T) {
		_, err := FromConfig(Config{Backend: BackendRedis, Burst: 20, Window: time.Second}, nil)
		if err == nil {
			t.Fatal("FromConfig() error = nil, want non-nil for missing redis client")
		}
	})

	t.Run("redis backend with client", func(t *testing.T) {
		client := &fakeRedisClient{}
		l, err := FromConfig(Config{Backend: BackendRedis, Burst: 20, Window: time.Second}, client)
		if err != nil {
			t.Fatalf("FromConfig() error = %v, want nil", err)
		}
		if _, ok := l.(*redisLimiter); !ok {
			t.Fatalf("FromConfig() type = %T, want *redisLimiter", l)
		}
	})

	t.Run("invalid config surfaces the validation error", func(t *testing.T) {
		_, err := FromConfig(Config{Backend: "bogus"}, nil)
		if err == nil {
			t.Fatal("FromConfig() error = nil, want non-nil for invalid config")
		}
	})
}
