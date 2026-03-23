package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
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

func TestOnboardParticipant_Success(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		codec: jsonCodec{},
		client: &mockClientConn{
			invokeFunc: func(_ context.Context, method string, args, reply any, _ ...grpc.CallOption) error {
				assert.Equal(t, onboardParticipantMethod, method)
			out := reply.(*struct {
				UserID        string `json:"user_id"`
				WalletAddress string `json:"wallet_address,omitempty"`
				CertPEM       string `json:"cert_pem,omitempty"`
				TxHash        string `json:"tx_hash,omitempty"`
			})
			out.UserID = "new-user-uuid"
			out.WalletAddress = "0xABCD"
			out.CertPEM = "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----"
			out.TxHash = "0xtxhash"
				return nil
			},
		},
	}

	result, err := mgr.OnboardParticipant(context.Background(), interfaces.OnboardParticipantRequest{
		Username: "banco-brasil",
		Email:    "admin@bb.com",
		Role:     "ROLE_COMMERCIAL_BANK",
	})

	assert.NoError(t, err)
	assert.Equal(t, "new-user-uuid", result.UserID)
	assert.Equal(t, "0xABCD", result.WalletAddress)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----", result.CertPEM)
	assert.Equal(t, "0xtxhash", result.TxHash)
}

func TestOnboardParticipant_Error(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		codec: jsonCodec{},
		client: &mockClientConn{
			invokeFunc: func(_ context.Context, _ string, _, _ any, _ ...grpc.CallOption) error {
				return errors.New("grpc error")
			},
		},
	}

	_, err := mgr.OnboardParticipant(context.Background(), interfaces.OnboardParticipantRequest{
		Username: "banco-brasil",
		Email:    "admin@bb.com",
		Role:     "ROLE_COMMERCIAL_BANK",
	})

	assert.Error(t, err)
}

// Compile-time check: IdentityGRPCManager must implement ParticipantOnboarder.
var _ interfaces.ParticipantOnboarder = (*IdentityGRPCManager)(nil)
