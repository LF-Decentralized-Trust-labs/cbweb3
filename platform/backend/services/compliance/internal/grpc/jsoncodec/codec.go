package jsoncodec

import "encoding/json"

// Codec enables gRPC transport with JSON payloads (no protobuf).
type Codec struct{}

func (Codec) Name() string { return "json" }

func (Codec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (Codec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
