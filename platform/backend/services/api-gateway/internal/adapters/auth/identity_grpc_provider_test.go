package auth

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

type mockAuthClientConn struct {
	invokeFunc func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error
}

func (m *mockAuthClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, method, args, reply, opts...)
	}
	return nil
}

func (m *mockAuthClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("not implemented")
}

func TestNewIdentityGRPCAuthProvider(t *testing.T) {
	// Test that it can create the provider successfully
	// Note: gRPC creates connection lazily, so it won't fail immediately
	// even if the server is not running
	provider, err := NewIdentityGRPCAuthProvider("localhost:99999", 1*1000000000) // 1 second

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.conn)
	assert.NotNil(t, provider.client)
	assert.NotNil(t, provider.codec)
}

func TestAuthenticate_Success(t *testing.T) {
	mockClient := &mockAuthClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			if method == identityLoginMethod {
				resp := reply.(*identityLoginResponse)
				resp.AccessToken = "test-token"
				resp.TokenType = "Bearer"
				resp.ExpiresIn = 3600
				return nil
			}
			return errors.New("unexpected method")
		},
	}

	provider := &IdentityGRPCAuthProvider{
		client: mockClient,
		codec:  jsonCodec{},
	}

	token, err := provider.Authenticate(context.Background(), "user123", "password123")

	assert.NoError(t, err)
	assert.Equal(t, "test-token", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, 3600, token.ExpiresIn)
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	mockClient := &mockAuthClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return status.Error(codes.Unauthenticated, "invalid credentials")
		},
	}

	provider := &IdentityGRPCAuthProvider{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, err := provider.Authenticate(context.Background(), "user123", "wrong-password")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestAuthenticate_NetworkError(t *testing.T) {
	mockClient := &mockAuthClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return errors.New("network error")
		},
	}

	provider := &IdentityGRPCAuthProvider{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, err := provider.Authenticate(context.Background(), "user123", "password123")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestValidate_Success(t *testing.T) {
	mockClient := &mockAuthClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			if method == identityValidateTokenMethod {
				resp := reply.(*identityValidateTokenResponse)
				resp.Subject = "user123"
				resp.Issuer = "identity-service"
				resp.Roles = []string{"admin", "user"}
				return nil
			}
			return errors.New("unexpected method")
		},
	}

	provider := &IdentityGRPCAuthProvider{
		client: mockClient,
		codec:  jsonCodec{},
	}

	claims, err := provider.Validate(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, "user123", claims.Subject)
	assert.Equal(t, "identity-service", claims.Issuer)
	assert.Equal(t, []string{"admin", "user"}, claims.Roles)
}

func TestValidate_InvalidToken(t *testing.T) {
	mockClient := &mockAuthClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return status.Error(codes.Unauthenticated, "invalid token")
		},
	}

	provider := &IdentityGRPCAuthProvider{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, err := provider.Validate(context.Background(), "invalid-token")

	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}

func TestValidate_NetworkError(t *testing.T) {
	mockClient := &mockAuthClientConn{
		invokeFunc: func(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
			return errors.New("network error")
		},
	}

	provider := &IdentityGRPCAuthProvider{
		client: mockClient,
		codec:  jsonCodec{},
	}

	_, err := provider.Validate(context.Background(), "some-token")

	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}

func TestJSONCodec_Auth(t *testing.T) {
	codec := jsonCodec{}

	assert.Equal(t, "json", codec.Name())

	original := identityLoginRequest{
		User:     "testuser",
		Password: "testpass",
	}

	data, err := codec.Marshal(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var decoded identityLoginRequest
	err = codec.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, original.User, decoded.User)
	assert.Equal(t, original.Password, decoded.Password)
}
