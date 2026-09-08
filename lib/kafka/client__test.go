package kafka

import (
	"reflect"
	"sort"
	"testing"
)

func TestNew_RejectsInvalidConfig(t *testing.T) {
	_, err := New(&Config{})
	if err == nil {
		t.Fatal("New() error = nil, want a validation error for an empty Config")
	}
}

func TestNew_BuildsClientForEachMechanism(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "none",
			cfg:  Config{Brokers: []string{"localhost:9092"}, Auth: AuthConfig{Mechanism: MechanismNone}},
		},
		{
			name: "plain",
			cfg: Config{
				Brokers: []string{"localhost:9092"},
				Auth:    AuthConfig{Mechanism: MechanismPlain, Username: "u", Password: "p"},
			},
		},
		{
			name: "scram-sha256",
			cfg: Config{
				Brokers: []string{"localhost:9092"},
				Auth:    AuthConfig{Mechanism: MechanismScramSHA256, Username: "u", Password: "p"},
			},
		},
		{
			name: "scram-sha512",
			cfg: Config{
				Brokers: []string{"localhost:9092"},
				Auth:    AuthConfig{Mechanism: MechanismScramSHA512, Username: "u", Password: "p"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(&tt.cfg)
			if err != nil {
				t.Fatalf("New() error = %v, want nil", err)
			}
			if c == nil || c.writer == nil {
				t.Fatal("New() built a Client with a nil writer")
			}
			_ = c.Close()
		})
	}
}

func TestToHeaders(t *testing.T) {
	t.Run("empty is nil", func(t *testing.T) {
		if h := toHeaders(nil); h != nil {
			t.Errorf("toHeaders(nil) = %v, want nil", h)
		}
		if h := toHeaders(map[string]string{}); h != nil {
			t.Errorf("toHeaders({}) = %v, want nil", h)
		}
	})

	t.Run("converts every entry", func(t *testing.T) {
		got := toHeaders(map[string]string{"a": "1", "b": "2"})
		sort.Slice(got, func(i, j int) bool { return got[i].Key < got[j].Key })
		want := []struct {
			Key   string
			Value string
		}{{"a", "1"}, {"b", "2"}}
		if len(got) != len(want) {
			t.Fatalf("toHeaders() len = %d, want %d", len(got), len(want))
		}
		for i, w := range want {
			if got[i].Key != w.Key || !reflect.DeepEqual(got[i].Value, []byte(w.Value)) {
				t.Errorf("toHeaders()[%d] = %+v, want {%s %s}", i, got[i], w.Key, w.Value)
			}
		}
	})
}
