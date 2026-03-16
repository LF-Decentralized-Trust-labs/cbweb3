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
	bindWalletMethod           = "/identity.v1.IdentityService/BindWallet"
	getByUserMethod            = "/identity.v1.IdentityService/GetByUser"
	registerParticipantMethod  = "/identity.v1.IdentityService/RegisterParticipant"
	getKYCStatusMethod         = "/identity.v1.IdentityService/GetKYCStatus"
	issueKYCCredentialMethod   = "/identity.v1.IdentityService/IssueKYCCredential"
	verifyKYCProofMethod       = "/identity.v1.IdentityService/VerifyKYCProof"
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
// It implements IIdentityManager, KYCManager, and ParticipantRegistrar.
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

// named private types for testability within the same package
type bindWalletRequest struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

type walletBindingResponse struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

type getByUserResponse struct {
	Binding *walletBindingResponse `json:"binding"`
	Found   bool                   `json:"found"`
}

// --- IIdentityManager ---

func (m *IdentityGRPCManager) BindWallet(ctx context.Context, userID, walletAddress string) (domain.WalletBinding, error) {
	req := &bindWalletRequest{UserID: userID, WalletAddress: walletAddress}
	out := &walletBindingResponse{}
	if err := m.client.Invoke(ctx, bindWalletMethod, req, out, grpc.ForceCodec(m.codec)); err != nil {
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.AlreadyExists {
				return domain.WalletBinding{}, domain.ErrWalletAlreadyBound
			}
			if st.Code() == codes.FailedPrecondition {
				return domain.WalletBinding{}, domain.ErrUserAlreadyBound
			}
		}
		return domain.WalletBinding{}, err
	}
	return domain.WalletBinding{
		UserID:        out.UserID,
		WalletAddress: out.WalletAddress,
	}, nil
}

func (m *IdentityGRPCManager) GetByUser(ctx context.Context, userID string) (domain.WalletBinding, bool) {
	req := &struct {
		UserID string `json:"user_id"`
	}{UserID: userID}
	out := &getByUserResponse{}
	if err := m.client.Invoke(ctx, getByUserMethod, req, out, grpc.ForceCodec(m.codec)); err != nil {
		return domain.WalletBinding{}, false
	}
	if !out.Found || out.Binding == nil {
		return domain.WalletBinding{}, false
	}
	if out.Binding.UserID == "" || out.Binding.WalletAddress == "" {
		return domain.WalletBinding{}, false
	}
	return domain.WalletBinding{
		UserID:        out.Binding.UserID,
		WalletAddress: out.Binding.WalletAddress,
	}, true
}

// --- ParticipantRegistrar ---

func (m *IdentityGRPCManager) RegisterParticipant(ctx context.Context, accessToken, country, bankCode, role, institutionName, walletType string) (interfaces.RegisterParticipantResult, error) {
	req := &struct {
		AccessToken     string `json:"access_token"`
		Country         string `json:"country"`
		BankCode        string `json:"bank_code"`
		Role            string `json:"role"`
		InstitutionName string `json:"institution_name"`
		WalletType      string `json:"wallet_type"`
	}{
		AccessToken:     accessToken,
		Country:         country,
		BankCode:        bankCode,
		Role:            role,
		InstitutionName: institutionName,
		WalletType:      walletType,
	}
	out := &struct {
		UserID         string `json:"user_id"`
		DID            string `json:"did"`
		WalletAddress  string `json:"wallet_address"`
		SignerProvider string `json:"signer_provider"`
	}{}
	if err := m.client.Invoke(ctx, registerParticipantMethod, req, out, grpc.ForceCodec(m.codec)); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return interfaces.RegisterParticipantResult{}, domain.ErrWalletAlreadyBound
		}
		return interfaces.RegisterParticipantResult{}, err
	}
	return interfaces.RegisterParticipantResult{
		UserID:         out.UserID,
		DID:            out.DID,
		WalletAddress:  out.WalletAddress,
		SignerProvider: out.SignerProvider,
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
	status, err := m.GetKYCStatus(context.Background(), subject)
	if err != nil {
		return domain.KYCPending
	}
	return status
}

func (m *IdentityGRPCManager) IssueKYCCredential(ctx context.Context, subject, issuerSubject, institutionName, countryCode, bankCode string) (interfaces.KYCCredentialResult, error) {
	req := &struct {
		Subject         string `json:"subject"`
		IssuerSubject   string `json:"issuer_subject"`
		InstitutionName string `json:"institution_name"`
		CountryCode     string `json:"country_code"`
		BankCode        string `json:"bank_code"`
	}{
		Subject:         subject,
		IssuerSubject:   issuerSubject,
		InstitutionName: institutionName,
		CountryCode:     countryCode,
		BankCode:        bankCode,
	}
	out := &struct {
		VCJWT      string `json:"vc_jwt"`
		ZKPPointer string `json:"zkp_pointer"`
		IssuedAt   string `json:"issued_at"`
	}{}
	if err := m.client.Invoke(ctx, issueKYCCredentialMethod, req, out, grpc.ForceCodec(m.codec)); err != nil {
		return interfaces.KYCCredentialResult{}, err
	}
	return interfaces.KYCCredentialResult{
		VCJWT:      out.VCJWT,
		ZKPPointer: out.ZKPPointer,
		IssuedAt:   out.IssuedAt,
	}, nil
}

func (m *IdentityGRPCManager) VerifyKYCProof(ctx context.Context, zkpPointer string) (bool, error) {
	req := &struct {
		ZKPPointer string `json:"zkp_pointer"`
	}{ZKPPointer: zkpPointer}
	out := &struct {
		Valid bool `json:"valid"`
	}{}
	if err := m.client.Invoke(ctx, verifyKYCProofMethod, req, out, grpc.ForceCodec(m.codec)); err != nil {
		return false, err
	}
	return out.Valid, nil
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
