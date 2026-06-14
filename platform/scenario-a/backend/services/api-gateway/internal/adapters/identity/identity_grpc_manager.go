package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// IdentityGRPCManager delegates identity and KYC operations to the identity service.
// It implements KYCChecker, KYCManager, and ParticipantOnboarder.
type IdentityGRPCManager struct {
	conn *grpc.ClientConn
	cc   authv1.AuthServiceClient
}

func NewIdentityGRPCManager(address string, timeout time.Duration) (*IdentityGRPCManager, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext( //nolint:staticcheck
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return NewIdentityGRPCManagerFromConn(conn), nil
}

// NewIdentityGRPCManagerFromConn wraps an existing gRPC connection.
// The caller retains ownership of conn lifecycle when using this constructor.
func NewIdentityGRPCManagerFromConn(conn *grpc.ClientConn) *IdentityGRPCManager {
	return &IdentityGRPCManager{conn: conn, cc: authv1.NewAuthServiceClient(conn)}
}

// Close releases the underlying gRPC connection.
func (m *IdentityGRPCManager) Close() error {
	if m.conn != nil {
		return m.conn.Close()
	}
	return nil
}

// --- KYCManager ---

func (m *IdentityGRPCManager) GetKYCStatus(ctx context.Context, subject string) (domain.KYCStatus, error) {
	out, err := m.cc.GetKYCStatus(ctx, &authv1.GetKYCStatusRequest{Subject: subject})
	if err != nil {
		return domain.KYCPending, err
	}
	return domain.KYCStatus(out.Status), nil
}

// GetStatus satisfies KYCChecker (sync fallback — uses background context).
func (m *IdentityGRPCManager) GetStatus(subject string) domain.KYCStatus {
	s, err := m.GetKYCStatus(context.Background(), subject)
	if err != nil {
		return domain.KYCPending
	}
	return s
}

func (m *IdentityGRPCManager) ProvisionParticipant(ctx context.Context, subject string, kycStatus domain.KYCStatus) error {
	_, err := m.cc.ProvisionParticipant(ctx, &authv1.ProvisionParticipantRequest{
		Subject: subject,
		Status:  string(kycStatus),
	})
	return err
}

// --- ParticipantOnboarder ---

// OnboardParticipant delegates administrative participant registration to the
// identity gRPC service's OnboardParticipant method.
func (m *IdentityGRPCManager) OnboardParticipant(ctx context.Context, req interfaces.OnboardParticipantRequest) (interfaces.OnboardParticipantResult, error) {
	out, err := m.cc.OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{
		Username:        req.Username,
		Email:           req.Email,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
		Country:         req.Country,
		BankCode:        req.BankCode,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return interfaces.OnboardParticipantResult{}, fmt.Errorf("already exists: %s", st.Message())
		}
		return interfaces.OnboardParticipantResult{}, err
	}
	return interfaces.OnboardParticipantResult{
		UserID:        out.UserId,
		WalletAddress: out.WalletAddress,
		CertPEM:       out.CertPem,
		TxHash:        out.TxHash,
		ClientSecret:  out.ClientSecret,
	}, nil
}

// --- UserManager ---

// ListUsers delegates to the identity gRPC service's ListUsers method.
func (m *IdentityGRPCManager) ListUsers(ctx context.Context, role, userStatus string) ([]interfaces.UserSummary, int, error) {
	out, err := m.cc.ListUsers(ctx, &authv1.ListUsersRequest{Role: role, Status: userStatus})
	if err != nil {
		return nil, 0, err
	}
	result := make([]interfaces.UserSummary, 0, len(out.Users))
	for _, u := range out.Users {
		result = append(result, interfaces.UserSummary{
			UserID:          u.UserId,
			InstitutionName: u.InstitutionName,
			Role:            u.Role,
			Status:          u.Status,
			WalletAddress:   u.WalletAddress,
			Country:         u.Country,
			BankCode:        u.BankCode,
		})
	}
	return result, int(out.Total), nil
}

// --- Onboarding (3-phase PKI flow) ---

func (m *IdentityGRPCManager) SubmitCredentialRequest(ctx context.Context, req interfaces.CredentialRequest) (interfaces.CredentialRequestResult, error) {
	out, err := m.cc.SubmitCredentialRequest(ctx, &authv1.SubmitCredentialRequestReq{
		CsrPem:              req.CsrPem,
		BlockchainPubKeyHex: req.BlockchainPubKeyHex,
		InstitutionName:     req.InstitutionName,
		LegalEntityId:       req.LegalEntityID,
		BankCode:            req.BankCode,
		Country:             req.Country,
		Role:                req.Role,
		Email:               req.Email,
		Username:            req.Username,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return interfaces.CredentialRequestResult{}, fmt.Errorf("already exists: %s", st.Message())
		}
		return interfaces.CredentialRequestResult{}, err
	}
	return interfaces.CredentialRequestResult{
		RequestID:     out.RequestId,
		UserID:        out.UserId,
		WalletAddress: out.WalletAddress,
		Status:        out.Status,
	}, nil
}

func (m *IdentityGRPCManager) GetOnboardingStatus(ctx context.Context, requestID string) (interfaces.OnboardingStatus, error) {
	out, err := m.cc.GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: requestID})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return interfaces.OnboardingStatus{}, fmt.Errorf("not found: %s", st.Message())
		}
		return interfaces.OnboardingStatus{}, err
	}
	return interfaces.OnboardingStatus{
		RequestID:     out.RequestId,
		UserID:        out.UserId,
		Status:        out.Status,
		PopNonce:      out.PopNonce,
		WalletAddress: out.WalletAddress,
	}, nil
}

// GetOnboardingStatusByBankCode resolves a bank's onboarding status without
// requiring the caller to supply the original request_id.
func (m *IdentityGRPCManager) GetOnboardingStatusByBankCode(ctx context.Context, bankCode string) (interfaces.OnboardingStatus, error) {
	return m.GetOnboardingStatus(ctx, "bank_code:"+bankCode)
}

func (m *IdentityGRPCManager) CompleteOnboarding(ctx context.Context, req interfaces.CompleteOnboardingRequest) (interfaces.CompleteOnboardingResult, error) {
	out, err := m.cc.CompleteOnboarding(ctx, &authv1.CompleteOnboardingRequest{
		RequestId:           req.RequestID,
		UserId:              req.UserID,
		PopSignatureHex:     req.PopSignatureHex,
		BlockchainPubKeyHex: req.BlockchainPubKeyHex,
	})
	if err != nil {
		return interfaces.CompleteOnboardingResult{}, err
	}
	return interfaces.CompleteOnboardingResult{
		UserID:        out.UserId,
		WalletAddress: out.WalletAddress,
		CertPEM:       out.CertPem,
		TxHash:        out.TxHash,
		ClientSecret:  out.ClientSecret,
		Status:        out.Status,
	}, nil
}

// GetUser delegates to the identity gRPC service's GetUser method.
func (m *IdentityGRPCManager) GetUser(ctx context.Context, userID string) (interfaces.UserDetail, error) {
	out, err := m.cc.GetUser(ctx, &authv1.GetUserRequest{UserId: userID})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return interfaces.UserDetail{}, fmt.Errorf("not found: %s", st.Message())
		}
		return interfaces.UserDetail{}, err
	}
	return interfaces.UserDetail{
		UserID:          out.UserId,
		Username:        out.Username,
		Email:           out.Email,
		InstitutionName: out.InstitutionName,
		Role:            out.Role,
		Status:          out.Status,
		WalletAddress:   out.WalletAddress,
		Country:         out.Country,
		BankCode:        out.BankCode,
	}, nil
}

// --- OnboardingKeyManager (KMS operations for commercial bank proxy) ---

// CreateOnboardingKey delegates to the auth service's CreateOnboardingKey RPC.
func (m *IdentityGRPCManager) CreateOnboardingKey(ctx context.Context, bankCode string) (string, string, error) {
	out, err := m.cc.CreateOnboardingKey(ctx, &authv1.CreateOnboardingKeyRequest{BankCode: bankCode})
	if err != nil {
		return "", "", fmt.Errorf("create onboarding key: %w", err)
	}
	return out.PubKeyHex, out.Address, nil
}

// SignOnboardingPoP delegates to the auth service's SignOnboardingPoP RPC.
func (m *IdentityGRPCManager) SignOnboardingPoP(ctx context.Context, keyID, nonceHex string) (string, string, error) {
	out, err := m.cc.SignOnboardingPoP(ctx, &authv1.SignOnboardingPoPRequest{KeyId: keyID, NonceHex: nonceHex})
	if err != nil {
		return "", "", fmt.Errorf("sign onboarding pop: %w", err)
	}
	return out.SignatureHex, out.PubKeyHex, nil
}
