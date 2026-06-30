package serializer

import (
	"bytes"
	"math"
	"reflect"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    []byte
		wantErr bool
	}{
		{
			name:  "marshals struct",
			input: sample{Name: "alice", Age: 30},
			want:  []byte(`{"name":"alice","age":30}`),
		},
		{
			name:  "marshals map",
			input: map[string]int{"a": 1},
			want:  []byte(`{"a":1}`),
		},
		{
			name:  "marshals slice",
			input: []string{"x", "y"},
			want:  []byte(`["x","y"]`),
		},
		{
			name:  "marshals string",
			input: "hello",
			want:  []byte(`"hello"`),
		},
		{
			name:  "marshals nil",
			input: nil,
			want:  []byte(`null`),
		},
		{
			name:    "returns error for unsupported value",
			input:   math.Inf(1),
			wantErr: true,
		},
		{
			name:    "returns error for channel",
			input:   make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToJSON(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ToJSON() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ToJSON() unexpected error = %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("ToJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    sample
		wantErr bool
	}{
		{
			name: "parses valid object",
			data: []byte(`{"name":"bob","age":42}`),
			want: sample{Name: "bob", Age: 42},
		},
		{
			name: "parses partial object",
			data: []byte(`{"name":"carol"}`),
			want: sample{Name: "carol"},
		},
		{
			name:    "returns error for invalid json",
			data:    []byte(`{"name":`),
			wantErr: true,
		},
		{
			name:    "returns error for type mismatch",
			data:    []byte(`{"age":"not-an-int"}`),
			wantErr: true,
		},
		{
			name:    "returns error for empty input",
			data:    []byte(``),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sample
			err := ParseJSON(tt.data, &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseJSON() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseJSON() unexpected error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseJSON() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseJSONNilTarget(t *testing.T) {
	if err := ParseJSON([]byte(`{"name":"x"}`), nil); err == nil {
		t.Fatal("ParseJSON(nil target) error = nil, want error")
	}
}

func TestRoundTrip(t *testing.T) {
	want := sample{Name: "dave", Age: 7}

	data, err := ToJSON(want)
	if err != nil {
		t.Fatalf("ToJSON() unexpected error = %v", err)
	}

	var got sample
	if err := ParseJSON(data, &got); err != nil {
		t.Fatalf("ParseJSON() unexpected error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}
