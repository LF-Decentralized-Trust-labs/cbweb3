package jsoncodec

import "encoding/json"

// Codec allows gRPC transport using JSON payloads for local contracts.
type Codec struct{}

func (Codec) Name() string { return "json" }

func (Codec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (Codec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
