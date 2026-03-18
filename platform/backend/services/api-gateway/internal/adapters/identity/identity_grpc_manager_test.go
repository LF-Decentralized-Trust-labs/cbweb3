package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

type mockClientConn struct {
	invokeFunc func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error
}

func (m *mockClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, method, args, reply, opts...)
	}
	return nil
}

func (m *mockClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("not implemented")
}

func TestNewIdentityGRPCManager(t *testing.T) {
	// Note: gRPC creates connection lazily, so it won't fail immediately
	// even if the server is not running
	manager, err := NewIdentityGRPCManager("localhost:99999", 1*1000000000) // 1 second in nanoseconds

	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.client)
	assert.NotNil(t, manager.codec)
}

func TestJSONCodec_Name(t *testing.T) {
	codec := jsonCodec{}
	assert.Equal(t, "json", codec.Name())
}

func TestJSONCodec_MarshalUnmarshal(t *testing.T) {
	codec := jsonCodec{}

	type testPayload struct {
		UserID  string `json:"user_id"`
		Country string `json:"country"`
	}

	original := testPayload{
		UserID:  "user123",
		Country: "BR",
	}

	data, err := codec.Marshal(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var decoded testPayload
	err = codec.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, original.UserID, decoded.UserID)
	assert.Equal(t, original.Country, decoded.Country)
}
