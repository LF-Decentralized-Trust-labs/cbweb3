package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

const (
	getKYCStatusMethod         = "/auth.v1.AuthService/GetKYCStatus"
	provisionParticipantMethod = "/auth.v1.AuthService/ProvisionParticipant"
	onboardParticipantMethod   = "/auth.v1.AuthService/OnboardParticipant"
)

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// IdentityGRPCManager delegates identity and KYC operations to the identity service.
// It implements KYCChecker, KYCManager, and ParticipantOnboarder.
type IdentityGRPCManager struct {
	client grpc.ClientConnInterface
	codec  encoding.Codec
}

func NewIdentityGRPCManager(address string, timeout time.Duration) (*IdentityGRPCManager, error) {
	codec := jsonCodec{}
	encoding.RegisterCodec(codec)
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec)),
	)
	if err != nil {
		return nil, err
	}
	return &IdentityGRPCManager{
		client: conn,
		codec:  codec,
	}, nil
}

// --- KYCManager ---

func (m *IdentityGRPCManager) GetKYCStatus(ctx context.Context, subject string) (domain.KYCStatus, error) {
	req := &struct {
		Subject string `json:"subject"`
	}{Subject: subject}
	out := &struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}{}
	if err := m.client.Invoke(ctx, getKYCStatusMethod, req, out, grpc.ForceCodec(m.codec)); err != nil {
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
	req := &struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}{Subject: subject, Status: string(kycStatus)}
	var out struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}
	return m.client.Invoke(ctx, provisionParticipantMethod, req, &out, grpc.ForceCodec(m.codec))
}

// --- ParticipantOnboarder ---

// OnboardParticipant delegates administrative participant registration to the
// identity gRPC service's OnboardParticipant method.
func (m *IdentityGRPCManager) OnboardParticipant(ctx context.Context, req interfaces.OnboardParticipantRequest) (interfaces.OnboardParticipantResult, error) {
	grpcReq := &struct {
		Username        string `json:"username"`
		Email           string `json:"email"`
		Role            string `json:"role"`
		InstitutionName string `json:"institution_name,omitempty"`
		Country         string `json:"country,omitempty"`
		BankCode        string `json:"bank_code,omitempty"`
	}{
		Username:        req.Username,
		Email:           req.Email,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
		Country:         req.Country,
		BankCode:        req.BankCode,
	}
	out := &struct {
		UserID        string `json:"user_id"`
		WalletAddress string `json:"wallet_address,omitempty"`
		CertPEM       string `json:"cert_pem,omitempty"`
		TxHash        string `json:"tx_hash,omitempty"`
	}{}
	if err := m.client.Invoke(ctx, onboardParticipantMethod, grpcReq, out, grpc.ForceCodec(m.codec)); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return interfaces.OnboardParticipantResult{}, fmt.Errorf("already exists: %s", st.Message())
		}
		return interfaces.OnboardParticipantResult{}, err
	}
	return interfaces.OnboardParticipantResult{
		UserID:        out.UserID,
		WalletAddress: out.WalletAddress,
		CertPEM:       out.CertPEM,
		TxHash:        out.TxHash,
	}, nil
}
