package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockAuthServiceClient implements authv1.AuthServiceClient for testing.
type mockAuthServiceClient struct {
	loginFunc              func(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error)
	refreshTokenFunc       func(ctx context.Context, in *authv1.RefreshTokenRequest, opts ...grpc.CallOption) (*authv1.RefreshTokenResponse, error)
	revokeTokenFunc        func(ctx context.Context, in *authv1.RevokeTokenRequest, opts ...grpc.CallOption) (*authv1.RevokeTokenResponse, error)
	validateTokenFunc      func(ctx context.Context, in *authv1.ValidateTokenRequest, opts ...grpc.CallOption) (*authv1.ValidateTokenResponse, error)
	issueLoginNonceFunc    func(ctx context.Context, in *authv1.IssueLoginNonceRequest, opts ...grpc.CallOption) (*authv1.IssueLoginNonceResponse, error)
	verifyPKILoginFunc     func(ctx context.Context, in *authv1.VerifyPKILoginRequest, opts ...grpc.CallOption) (*authv1.VerifyPKILoginResponse, error)
	changeClientSecretFunc func(ctx context.Context, in *authv1.ChangeClientSecretRequest, opts ...grpc.CallOption) (*authv1.ChangeClientSecretResponse, error)
}

func (m *mockAuthServiceClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, in, opts...)
	}
	return &authv1.LoginResponse{}, nil
}

func (m *mockAuthServiceClient) RefreshToken(ctx context.Context, in *authv1.RefreshTokenRequest, opts ...grpc.CallOption) (*authv1.RefreshTokenResponse, error) {
	if m.refreshTokenFunc != nil {
		return m.refreshTokenFunc(ctx, in, opts...)
	}
	return &authv1.RefreshTokenResponse{}, nil
}

func (m *mockAuthServiceClient) RevokeToken(ctx context.Context, in *authv1.RevokeTokenRequest, opts ...grpc.CallOption) (*authv1.RevokeTokenResponse, error) {
	if m.revokeTokenFunc != nil {
		return m.revokeTokenFunc(ctx, in, opts...)
	}
	return &authv1.RevokeTokenResponse{}, nil
}

func (m *mockAuthServiceClient) ValidateToken(ctx context.Context, in *authv1.ValidateTokenRequest, opts ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	if m.validateTokenFunc != nil {
		return m.validateTokenFunc(ctx, in, opts...)
	}
	return &authv1.ValidateTokenResponse{}, nil
}

func (m *mockAuthServiceClient) IssueLoginNonce(ctx context.Context, in *authv1.IssueLoginNonceRequest, opts ...grpc.CallOption) (*authv1.IssueLoginNonceResponse, error) {
	if m.issueLoginNonceFunc != nil {
		return m.issueLoginNonceFunc(ctx, in, opts...)
	}
	return &authv1.IssueLoginNonceResponse{}, nil
}

func (m *mockAuthServiceClient) VerifyPKILogin(ctx context.Context, in *authv1.VerifyPKILoginRequest, opts ...grpc.CallOption) (*authv1.VerifyPKILoginResponse, error) {
	if m.verifyPKILoginFunc != nil {
		return m.verifyPKILoginFunc(ctx, in, opts...)
	}
	return &authv1.VerifyPKILoginResponse{}, nil
}

func (m *mockAuthServiceClient) ChangeClientSecret(ctx context.Context, in *authv1.ChangeClientSecretRequest, opts ...grpc.CallOption) (*authv1.ChangeClientSecretResponse, error) {
	if m.changeClientSecretFunc != nil {
		return m.changeClientSecretFunc(ctx, in, opts...)
	}
	return &authv1.ChangeClientSecretResponse{}, nil
}

func (m *mockAuthServiceClient) RegisterParticipant(ctx context.Context, in *authv1.RegisterParticipantRequest, opts ...grpc.CallOption) (*authv1.RegisterParticipantResponse, error) {
	return &authv1.RegisterParticipantResponse{}, nil
}
func (m *mockAuthServiceClient) SignTransaction(ctx context.Context, in *authv1.SignTransactionRequest, opts ...grpc.CallOption) (*authv1.SignTransactionResponse, error) {
	return &authv1.SignTransactionResponse{}, nil
}
func (m *mockAuthServiceClient) GetKYCStatus(ctx context.Context, in *authv1.GetKYCStatusRequest, opts ...grpc.CallOption) (*authv1.GetKYCStatusResponse, error) {
	return &authv1.GetKYCStatusResponse{}, nil
}
func (m *mockAuthServiceClient) ProvisionParticipant(ctx context.Context, in *authv1.ProvisionParticipantRequest, opts ...grpc.CallOption) (*authv1.ProvisionParticipantResponse, error) {
	return &authv1.ProvisionParticipantResponse{}, nil
}
func (m *mockAuthServiceClient) OnboardParticipant(ctx context.Context, in *authv1.OnboardParticipantRequest, opts ...grpc.CallOption) (*authv1.OnboardParticipantResponse, error) {
	return &authv1.OnboardParticipantResponse{}, nil
}
func (m *mockAuthServiceClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return &authv1.ListUsersResponse{}, nil
}
func (m *mockAuthServiceClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	return &authv1.GetUserResponse{}, nil
}

func TestNewIdentityGRPCAuthProvider(t *testing.T) {
	// gRPC creates the connection lazily, so it won't fail even if server is not running.
	provider, err := NewIdentityGRPCAuthProvider("localhost:99999", 1*1000000000) // 1 second

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.cc)
}

func TestAuthenticate_Success(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		loginFunc: func(_ context.Context, in *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.LoginResponse, error) {
			return &authv1.LoginResponse{
				AccessToken: "test-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			}, nil
		},
	}

	provider := &IdentityGRPCAuthProvider{cc: mockClient}

	token, err := provider.Authenticate(context.Background(), "user123", "password123")

	assert.NoError(t, err)
	assert.Equal(t, "test-token", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, 3600, token.ExpiresIn)
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		loginFunc: func(_ context.Context, _ *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.LoginResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		},
	}

	provider := &IdentityGRPCAuthProvider{cc: mockClient}

	_, err := provider.Authenticate(context.Background(), "user123", "wrong-password")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestAuthenticate_NetworkError(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		loginFunc: func(_ context.Context, _ *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.LoginResponse, error) {
			return nil, errors.New("network error")
		},
	}

	provider := &IdentityGRPCAuthProvider{cc: mockClient}

	_, err := provider.Authenticate(context.Background(), "user123", "password123")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestValidate_Success(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		validateTokenFunc: func(_ context.Context, _ *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
			return &authv1.ValidateTokenResponse{
				Subject: "user123",
				Issuer:  "identity-service",
				Roles:   []string{"admin", "user"},
			}, nil
		},
	}

	provider := &IdentityGRPCAuthProvider{cc: mockClient}

	claims, err := provider.Validate(context.Background(), "valid-token")

	assert.NoError(t, err)
	assert.Equal(t, "user123", claims.Subject)
	assert.Equal(t, "identity-service", claims.Issuer)
	assert.Equal(t, []string{"admin", "user"}, claims.Roles)
}

func TestValidate_InvalidToken(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		validateTokenFunc: func(_ context.Context, _ *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		},
	}

	provider := &IdentityGRPCAuthProvider{cc: mockClient}

	_, err := provider.Validate(context.Background(), "invalid-token")

	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}

func TestValidate_NetworkError(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		validateTokenFunc: func(_ context.Context, _ *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
			return nil, errors.New("network error")
		},
	}

	provider := &IdentityGRPCAuthProvider{cc: mockClient}

	_, err := provider.Validate(context.Background(), "some-token")

	assert.ErrorIs(t, err, domain.ErrInvalidToken)
}
