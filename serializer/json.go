package serializer

import "encoding/json"

func ToJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func ParseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
