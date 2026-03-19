// Package complianceclient is the gRPC client that the identity service uses
// to communicate with the compliance-orchestrator service.
// It replaces dataaccessclient — all persistence and compliance operations now
// go through compliance (port 9093).
package complianceclient

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const (
	upsertParticipantMethod           = "/compliance.v1.ComplianceService/UpsertParticipant"
	getParticipantByUserMethod        = "/compliance.v1.ComplianceService/GetParticipantByUser"
	createAuditLogMethod              = "/compliance.v1.ComplianceService/CreateAuditLog"
	issueParticipantCertificateMethod = "/compliance.v1.ComplianceService/IssueParticipantCertificate"
	manageParticipantStatusMethod     = "/compliance.v1.ComplianceService/ManageParticipantStatus"
)

// Participant holds all participant metadata.
type Participant struct {
	UserID            string
	InstitutionName   string
	CNPJ              string
	BankCode          string
	CountryCode       string
	Role              string
	WalletAddress     string
	Status            string     // PENDING | ACTIVE | FROZEN | REVOKED
	CertificateData   string
	CertificateExpiry *time.Time
}

// AuditEntry carries a single audit event for persistence.
type AuditEntry struct {
	ActorSubject  string
	ActorAddress  string
	ActionType    string
	TargetSubject string
	CorrelationID string
	IPAddress     string
	Result        string
	Category      string
	Severity      string
	Details       string
}

// IssuedCertificate contains the PKI certificate issued for a participant.
type IssuedCertificate struct {
	CertPEM    string
	PrivKeyPEM string
	ExpiresAt  string
}

// Client defines the compliance gRPC operations used by the identity service.
type Client interface {
	UpsertParticipant(ctx context.Context, p Participant) error
	GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error)
	CreateAuditLog(ctx context.Context, entry AuditEntry) error
	IssueParticipantCertificate(ctx context.Context, userID, role, institutionName, cnpj string) (IssuedCertificate, error)
	ManageParticipantStatus(ctx context.Context, subject, status, reason string) error
}

type jsonCodec struct{}

func (jsonCodec) Name() string                  { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

type grpcClient struct {
	cc    grpc.ClientConnInterface
	codec encoding.Codec
}

// New connects to the compliance-orchestrator at the given address.
func New(address string, timeout time.Duration) (Client, error) {
	codec := jsonCodec{}
	encoding.RegisterCodec(codec)

	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//nolint:staticcheck
	conn, err := grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec)),
	)
	if err != nil {
		return nil, err
	}
	return &grpcClient{cc: conn, codec: codec}, nil
}

func (c *grpcClient) UpsertParticipant(ctx context.Context, p Participant) error {
	req := struct {
		Participant struct {
			UserID            string     `json:"user_id"`
			InstitutionName   string     `json:"institution_name"`
			CNPJ              string     `json:"cnpj"`
			BankCode          string     `json:"bank_code"`
			CountryCode       string     `json:"country_code"`
			Role              string     `json:"role"`
			WalletAddress     string     `json:"wallet_address"`
			Status            string     `json:"status"`
			CertificateData   string     `json:"certificate_data,omitempty"`
			CertificateExpiry *time.Time `json:"certificate_expiry,omitempty"`
		} `json:"participant"`
	}{}
	req.Participant.UserID = p.UserID
	req.Participant.InstitutionName = p.InstitutionName
	req.Participant.CNPJ = p.CNPJ
	req.Participant.BankCode = p.BankCode
	req.Participant.CountryCode = p.CountryCode
	req.Participant.Role = p.Role
	req.Participant.WalletAddress = p.WalletAddress
	req.Participant.Status = p.Status
	req.Participant.CertificateData = p.CertificateData
	req.Participant.CertificateExpiry = p.CertificateExpiry

	var resp struct {
		Success bool `json:"success"`
	}
	return c.cc.Invoke(ctx, upsertParticipantMethod, &req, &resp, grpc.ForceCodec(c.codec))
}

func (c *grpcClient) GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error) {
	req := struct {
		UserID string `json:"user_id"`
	}{UserID: userID}

	var resp struct {
		Found       bool `json:"found"`
		Participant struct {
			UserID            string     `json:"user_id"`
			InstitutionName   string     `json:"institution_name"`
			CNPJ              string     `json:"cnpj"`
			BankCode          string     `json:"bank_code"`
			CountryCode       string     `json:"country_code"`
			Role              string     `json:"role"`
			WalletAddress     string     `json:"wallet_address"`
			Status            string     `json:"status"`
			CertificateData   string     `json:"certificate_data"`
			CertificateExpiry *time.Time `json:"certificate_expiry"`
		} `json:"participant"`
	}
	if err := c.cc.Invoke(ctx, getParticipantByUserMethod, &req, &resp, grpc.ForceCodec(c.codec)); err != nil {
		return Participant{}, false, err
	}
	if !resp.Found {
		return Participant{}, false, nil
	}
	return Participant{
		UserID:            resp.Participant.UserID,
		InstitutionName:   resp.Participant.InstitutionName,
		CNPJ:              resp.Participant.CNPJ,
		BankCode:          resp.Participant.BankCode,
		CountryCode:       resp.Participant.CountryCode,
		Role:              resp.Participant.Role,
		WalletAddress:     resp.Participant.WalletAddress,
		Status:            resp.Participant.Status,
		CertificateData:   resp.Participant.CertificateData,
		CertificateExpiry: resp.Participant.CertificateExpiry,
	}, true, nil
}

func (c *grpcClient) CreateAuditLog(ctx context.Context, entry AuditEntry) error {
	req := struct {
		Entry struct {
			ActorSubject  string `json:"actor_subject"`
			ActorAddress  string `json:"actor_address"`
			ActionType    string `json:"action_type"`
			TargetSubject string `json:"target_subject"`
			CorrelationID string `json:"correlation_id"`
			IPAddress     string `json:"ip_address"`
			Result        string `json:"result"`
			Category      string `json:"category"`
			Severity      string `json:"severity"`
			Details       string `json:"details"`
		} `json:"entry"`
	}{}
	req.Entry.ActorSubject = entry.ActorSubject
	req.Entry.ActorAddress = entry.ActorAddress
	req.Entry.ActionType = entry.ActionType
	req.Entry.TargetSubject = entry.TargetSubject
	req.Entry.CorrelationID = entry.CorrelationID
	req.Entry.IPAddress = entry.IPAddress
	req.Entry.Result = entry.Result
	req.Entry.Category = entry.Category
	req.Entry.Severity = entry.Severity
	req.Entry.Details = entry.Details
	var resp struct {
		Success bool `json:"success"`
	}
	return c.cc.Invoke(ctx, createAuditLogMethod, &req, &resp, grpc.ForceCodec(c.codec))
}

func (c *grpcClient) IssueParticipantCertificate(ctx context.Context, userID, role, institutionName, cnpj string) (IssuedCertificate, error) {
	req := struct {
		UserID          string `json:"user_id"`
		Role            string `json:"role"`
		InstitutionName string `json:"institution_name"`
		CNPJ            string `json:"cnpj"`
	}{
		UserID:          userID,
		Role:            role,
		InstitutionName: institutionName,
		CNPJ:            cnpj,
	}
	var resp struct {
		CertPEM    string `json:"cert_pem"`
		PrivKeyPEM string `json:"priv_key_pem"`
		ExpiresAt  string `json:"expires_at"`
	}
	if err := c.cc.Invoke(ctx, issueParticipantCertificateMethod, &req, &resp, grpc.ForceCodec(c.codec)); err != nil {
		return IssuedCertificate{}, err
	}
	return IssuedCertificate{
		CertPEM:    resp.CertPEM,
		PrivKeyPEM: resp.PrivKeyPEM,
		ExpiresAt:  resp.ExpiresAt,
	}, nil
}

func (c *grpcClient) ManageParticipantStatus(ctx context.Context, subject, status, reason string) error {
	req := struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
	}{Subject: subject, Status: status, Reason: reason}
	var resp struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}
	return c.cc.Invoke(ctx, manageParticipantStatusMethod, &req, &resp, grpc.ForceCodec(c.codec))
}
