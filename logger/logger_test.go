package logger

import (
	"reflect"
	"testing"
)

func TestF(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    any
		expected Field
	}{
		{
			name:     "string value",
			key:      "user_id",
			value:    "123",
			expected: Field{Key: "user_id", Value: "123"},
		},
		{
			name:     "integer value",
			key:      "count",
			value:    42,
			expected: Field{Key: "count", Value: 42},
		},
		{
			name:     "float value",
			key:      "rate",
			value:    3.14,
			expected: Field{Key: "rate", Value: 3.14},
		},
		{
			name:     "boolean value",
			key:      "enabled",
			value:    true,
			expected: Field{Key: "enabled", Value: true},
		},
		{
			name:     "nil value",
			key:      "optional",
			value:    nil,
			expected: Field{Key: "optional", Value: nil},
		},
		{
			name:     "map value",
			key:      "metadata",
			value:    map[string]interface{}{"key": "value"},
			expected: Field{Key: "metadata", Value: map[string]interface{}{"key": "value"}},
		},
		{
			name:     "slice value",
			key:      "tags",
			value:    []string{"tag1", "tag2"},
			expected: Field{Key: "tags", Value: []string{"tag1", "tag2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := F(tt.key, tt.value)
			if result.Key != tt.expected.Key {
				t.Errorf("F() Key = %v, want %v", result.Key, tt.expected.Key)
			}
			// Use reflect.DeepEqual for comparing values that may not be directly comparable
			if !reflect.DeepEqual(result.Value, tt.expected.Value) {
				t.Errorf("F() Value = %v, want %v", result.Value, tt.expected.Value)
			}
		})
	}
}
