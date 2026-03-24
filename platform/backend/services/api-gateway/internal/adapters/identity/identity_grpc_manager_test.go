package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockAuthClient is a test-only implementation of authv1.AuthServiceClient.
// Only the methods exercised by IdentityGRPCManager are non-trivially implemented.
type mockAuthClient struct {
	getKYCStatusFn         func(*authv1.GetKYCStatusRequest) (*authv1.GetKYCStatusResponse, error)
	provisionParticipantFn func(*authv1.ProvisionParticipantRequest) (*authv1.ProvisionParticipantResponse, error)
	onboardParticipantFn   func(*authv1.OnboardParticipantRequest) (*authv1.OnboardParticipantResponse, error)
	listUsersFn            func(*authv1.ListUsersRequest) (*authv1.ListUsersResponse, error)
	getUserFn              func(*authv1.GetUserRequest) (*authv1.GetUserResponse, error)
}

func (m *mockAuthClient) GetKYCStatus(_ context.Context, in *authv1.GetKYCStatusRequest, _ ...grpc.CallOption) (*authv1.GetKYCStatusResponse, error) {
	if m.getKYCStatusFn != nil {
		return m.getKYCStatusFn(in)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) ProvisionParticipant(_ context.Context, in *authv1.ProvisionParticipantRequest, _ ...grpc.CallOption) (*authv1.ProvisionParticipantResponse, error) {
	if m.provisionParticipantFn != nil {
		return m.provisionParticipantFn(in)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) OnboardParticipant(_ context.Context, in *authv1.OnboardParticipantRequest, _ ...grpc.CallOption) (*authv1.OnboardParticipantResponse, error) {
	if m.onboardParticipantFn != nil {
		return m.onboardParticipantFn(in)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) ListUsers(_ context.Context, in *authv1.ListUsersRequest, _ ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(in)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) GetUser(_ context.Context, in *authv1.GetUserRequest, _ ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	if m.getUserFn != nil {
		return m.getUserFn(in)
	}
	return nil, errors.New("not implemented")
}

// Stub implementations for unused methods (satisfy authv1.AuthServiceClient interface).
func (m *mockAuthClient) Login(_ context.Context, _ *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.LoginResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) RefreshToken(_ context.Context, _ *authv1.RefreshTokenRequest, _ ...grpc.CallOption) (*authv1.RefreshTokenResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) RevokeToken(_ context.Context, _ *authv1.RevokeTokenRequest, _ ...grpc.CallOption) (*authv1.RevokeTokenResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) ValidateToken(_ context.Context, _ *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) RegisterParticipant(_ context.Context, _ *authv1.RegisterParticipantRequest, _ ...grpc.CallOption) (*authv1.RegisterParticipantResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) SignTransaction(_ context.Context, _ *authv1.SignTransactionRequest, _ ...grpc.CallOption) (*authv1.SignTransactionResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) IssueLoginNonce(_ context.Context, _ *authv1.IssueLoginNonceRequest, _ ...grpc.CallOption) (*authv1.IssueLoginNonceResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) VerifyPKILogin(_ context.Context, _ *authv1.VerifyPKILoginRequest, _ ...grpc.CallOption) (*authv1.VerifyPKILoginResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAuthClient) ChangeClientSecret(_ context.Context, _ *authv1.ChangeClientSecretRequest, _ ...grpc.CallOption) (*authv1.ChangeClientSecretResponse, error) {
	return nil, errors.New("not implemented")
}

func TestNewIdentityGRPCManager(t *testing.T) {
	// Note: gRPC creates connection lazily, so it won't fail immediately
	// even if the server is not running
	manager, err := NewIdentityGRPCManager("localhost:99999", 1*1000000000) // 1 second in nanoseconds

	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.cc)
}

func TestOnboardParticipant_Success(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		cc: &mockAuthClient{
			onboardParticipantFn: func(in *authv1.OnboardParticipantRequest) (*authv1.OnboardParticipantResponse, error) {
				assert.Equal(t, "banco-brasil", in.Username)
				assert.Equal(t, "admin@bb.com", in.Email)
				assert.Equal(t, "ROLE_COMMERCIAL_BANK", in.Role)
				return &authv1.OnboardParticipantResponse{
					UserId:        "new-user-uuid",
					WalletAddress: "0xABCD",
					CertPem:       "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----",
					TxHash:        "0xtxhash",
				}, nil
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

func TestOnboardParticipant_AlreadyExists(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		cc: &mockAuthClient{
			onboardParticipantFn: func(_ *authv1.OnboardParticipantRequest) (*authv1.OnboardParticipantResponse, error) {
				return nil, status.Error(codes.AlreadyExists, "user already exists")
			},
		},
	}

	_, err := mgr.OnboardParticipant(context.Background(), interfaces.OnboardParticipantRequest{
		Username: "banco-brasil",
		Email:    "admin@bb.com",
		Role:     "ROLE_COMMERCIAL_BANK",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestOnboardParticipant_Error(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		cc: &mockAuthClient{
			onboardParticipantFn: func(_ *authv1.OnboardParticipantRequest) (*authv1.OnboardParticipantResponse, error) {
				return nil, errors.New("grpc error")
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

// Compile-time check: IdentityGRPCManager must implement ParticipantOnboarder and UserManager.
var _ interfaces.ParticipantOnboarder = (*IdentityGRPCManager)(nil)
var _ interfaces.UserManager = (*IdentityGRPCManager)(nil)

func TestListUsers_Success(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		cc: &mockAuthClient{
			listUsersFn: func(in *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
				assert.Equal(t, "ROLE_COMMERCIAL_BANK", in.Role)
				assert.Equal(t, "ACTIVE", in.Status)
				return &authv1.ListUsersResponse{
					Users: []*authv1.UserSummary{
						{
							UserId:          "uid-1",
							InstitutionName: "Banco do Brasil",
							Role:            "ROLE_COMMERCIAL_BANK",
							Status:          "ACTIVE",
							WalletAddress:   "0xABCD",
						},
					},
					Total: 1,
				}, nil
			},
		},
	}

	users, total, err := mgr.ListUsers(context.Background(), "ROLE_COMMERCIAL_BANK", "ACTIVE")
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, users, 1)
	assert.Equal(t, "uid-1", users[0].UserID)
	assert.Equal(t, "Banco do Brasil", users[0].InstitutionName)
}

func TestListUsers_Error(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		cc: &mockAuthClient{
			listUsersFn: func(_ *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
				return nil, errors.New("grpc error")
			},
		},
	}

	_, _, err := mgr.ListUsers(context.Background(), "", "")
	assert.Error(t, err)
}

func TestGetUser_Success(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		cc: &mockAuthClient{
			getUserFn: func(in *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
				assert.Equal(t, "uid-1", in.UserId)
				return &authv1.GetUserResponse{
					UserId:   "uid-1",
					Username: "banco-brasil",
					Email:    "admin@bb.com",
					Role:     "ROLE_COMMERCIAL_BANK",
					Status:   "ACTIVE",
				}, nil
			},
		},
	}

	user, err := mgr.GetUser(context.Background(), "uid-1")
	assert.NoError(t, err)
	assert.Equal(t, "uid-1", user.UserID)
	assert.Equal(t, "banco-brasil", user.Username)
	assert.Equal(t, "admin@bb.com", user.Email)
}

func TestGetUser_NotFound(t *testing.T) {
	t.Parallel()

	mgr := &IdentityGRPCManager{
		cc: &mockAuthClient{
			getUserFn: func(_ *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
				return nil, status.Error(codes.NotFound, "user not found")
			},
		},
	}

	_, err := mgr.GetUser(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}


