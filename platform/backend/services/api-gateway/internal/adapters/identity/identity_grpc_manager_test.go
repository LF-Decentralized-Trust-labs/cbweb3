package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	// Test that it can create the manager successfully
	// Note: gRPC creates connection lazily, so it won't fail immediately
	// even if the server is not running
	manager, err := NewIdentityGRPCManager("localhost:99999", 1*1000000000) // 1 second in nanoseconds

	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.client)
	assert.NotNil(t, manager.codec)
}

func TestBindWallet_Success(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			if method == bindWalletMethod {
				resp := reply.(*walletBindingResponse)
				resp.UserID = "user123"
				resp.WalletAddress = "0xabc"
				return nil
			}
			return errors.New("unexpected method")
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	binding, err := manager.BindWallet(context.Background(), "user123", "0xabc")

	assert.NoError(t, err)
	assert.Equal(t, "user123", binding.UserID)
	assert.Equal(t, "0xabc", binding.WalletAddress)
}

func TestBindWallet_AlreadyExists(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return status.Error(codes.AlreadyExists, "wallet already bound")
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, err := manager.BindWallet(context.Background(), "user123", "0xabc")

	assert.ErrorIs(t, err, domain.ErrWalletAlreadyBound)
}

func TestBindWallet_UserAlreadyBound(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return status.Error(codes.FailedPrecondition, "user already bound")
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, err := manager.BindWallet(context.Background(), "user123", "0xabc")

	assert.ErrorIs(t, err, domain.ErrUserAlreadyBound)
}

func TestBindWallet_GenericError(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return errors.New("network error")
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, err := manager.BindWallet(context.Background(), "user123", "0xabc")

	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrWalletAlreadyBound)
	assert.NotErrorIs(t, err, domain.ErrUserAlreadyBound)
}

func TestGetByUser_Success(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			if method == getByUserMethod {
				resp := reply.(*getByUserResponse)
				resp.Found = true
				resp.Binding = &walletBindingResponse{
					UserID:        "user123",
					WalletAddress: "0xdef",
				}
				return nil
			}
			return errors.New("unexpected method")
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	binding, found := manager.GetByUser(context.Background(), "user123")

	assert.True(t, found)
	assert.Equal(t, "user123", binding.UserID)
	assert.Equal(t, "0xdef", binding.WalletAddress)
}

func TestGetByUser_NotFound(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			resp := reply.(*getByUserResponse)
			resp.Found = false
			return nil
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, found := manager.GetByUser(context.Background(), "user123")

	assert.False(t, found)
}

func TestGetByUser_Error(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return errors.New("connection error")
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, found := manager.GetByUser(context.Background(), "user123")

	assert.False(t, found)
}

func TestGetByUser_NilBinding(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			resp := reply.(*getByUserResponse)
			resp.Found = true
			resp.Binding = nil
			return nil
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, found := manager.GetByUser(context.Background(), "user123")

	assert.False(t, found)
}

func TestGetByUser_EmptyUserID(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			resp := reply.(*getByUserResponse)
			resp.Found = true
			resp.Binding = &walletBindingResponse{
				UserID:        "",
				WalletAddress: "0xdef",
			}
			return nil
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, found := manager.GetByUser(context.Background(), "user123")

	assert.False(t, found)
}

func TestGetByUser_EmptyWalletAddress(t *testing.T) {
	mockClient := &mockClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			resp := reply.(*getByUserResponse)
			resp.Found = true
			resp.Binding = &walletBindingResponse{
				UserID:        "user123",
				WalletAddress: "",
			}
			return nil
		},
	}

	manager := &IdentityGRPCManager{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, found := manager.GetByUser(context.Background(), "user123")

	assert.False(t, found)
}

func TestJSONCodec_Name(t *testing.T) {
	codec := jsonCodec{}
	assert.Equal(t, "json", codec.Name())
}

func TestJSONCodec_MarshalUnmarshal(t *testing.T) {
	codec := jsonCodec{}

	original := bindWalletRequest{
		UserID:        "user123",
		WalletAddress: "0xabc",
	}

	data, err := codec.Marshal(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var decoded bindWalletRequest
	err = codec.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, original.UserID, decoded.UserID)
	assert.Equal(t, original.WalletAddress, decoded.WalletAddress)
}
