package identity

import (
	"context"
	"encoding/json"
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
	registerParticipantMethod  = "/identity.v1.IdentityService/RegisterParticipant"
	getKYCStatusMethod         = "/identity.v1.IdentityService/GetKYCStatus"
	provisionParticipantMethod = "/identity.v1.IdentityService/ProvisionParticipant"
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
// It implements KYCChecker, KYCManager, and ParticipantRegistrar.
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

// --- ParticipantRegistrar ---

func (m *IdentityGRPCManager) RegisterParticipant(ctx context.Context, accessToken, country, bankCode, role, institutionName string) (interfaces.RegisterParticipantResult, error) {
	req := &struct {
		AccessToken     string `json:"access_token"`
		Country         string `json:"country"`
		BankCode        string `json:"bank_code"`
		Role            string `json:"role"`
		InstitutionName string `json:"institution_name"`
	}{
		AccessToken:     accessToken,
		Country:         country,
		BankCode:        bankCode,
		Role:            role,
		InstitutionName: institutionName,
	}
	out := &struct {
		UserID        string `json:"user_id"`
		DID           string `json:"did"`
		WalletAddress string `json:"wallet_address"`
	}{}
	if err := m.client.Invoke(ctx, registerParticipantMethod, req, out, grpc.ForceCodec(m.codec)); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return interfaces.RegisterParticipantResult{}, domain.ErrWalletAlreadyBound
		}
		return interfaces.RegisterParticipantResult{}, err
	}
	return interfaces.RegisterParticipantResult{
		UserID:        out.UserID,
		DID:           out.DID,
		WalletAddress: out.WalletAddress,
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
