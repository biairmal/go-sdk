// Package serializer provides thin wrappers around [encoding/json] for
// marshaling Go values to JSON bytes and unmarshalling JSON bytes back into
// Go values. It adds no configuration or state; all behavior is delegated
// directly to the standard library.
package serializer

import "encoding/json"

// ToJSON marshals v to its JSON encoding.
// It returns an error if v contains types that cannot be marshaled
// (e.g. channels, functions, or non-finite floating-point values).
func ToJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// ParseJSON parses the JSON-encoded data and stores the result in the value
// pointed to by v. It returns an error if data is not valid JSON or if the
// decoded value cannot be stored in v.
func ParseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
