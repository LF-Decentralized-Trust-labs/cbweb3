// SPDX-License-Identifier: Apache-2.0

// Package complianceclient is the gRPC client that the identity service uses
// to communicate with the compliance-orchestrator service.
// It replaces dataaccessclient — all persistence and compliance operations now
// go through compliance (port 9093).
package complianceclient

import (
	"context"
	"time"

	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Participant holds all participant metadata.
type Participant struct {
	UserID              string
	InstitutionName     string
	CNPJ                string
	BankCode            string
	CountryCode         string
	Role                string
	WalletAddress       string
	Status              string
	CertificateData     string
	CertificateExpiry   *time.Time
	BlockchainPubKeyHex string
	CsrPem              string
	PopNonce            string
	PopNonceExpiresAt   *time.Time
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

// ParticipantFilter holds optional filters for ListParticipants.
type ParticipantFilter struct {
	Role     string
	Status   string
	BankCode string // exact match; passed via the Search proto field with "bank_code:" prefix
}

// SignedCSR contains the certificate produced by signing a participant's CSR.
type SignedCSR struct {
	CertPEM   string
	ExpiresAt string
}

// Client defines the compliance gRPC operations used by the identity service.
type Client interface {
	UpsertParticipant(ctx context.Context, p Participant) error
	GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error)
	ListParticipants(ctx context.Context, filter ParticipantFilter) ([]Participant, error)
	CreateAuditLog(ctx context.Context, entry AuditEntry) error
	IssueParticipantCertificate(ctx context.Context, userID, role, institutionName, cnpj string) (IssuedCertificate, error)
	SignParticipantCSR(ctx context.Context, csrPem, userID, role, institutionName, cnpj string) (SignedCSR, error)
	ManageParticipantStatus(ctx context.Context, subject, status, reason string) error
}

type grpcClient struct {
	cc compliancv1.ComplianceServiceClient
}

// New connects to the compliance-orchestrator at the given address.
func New(address string, timeout time.Duration) (Client, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//nolint:staticcheck
	conn, err := grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &grpcClient{cc: compliancv1.NewComplianceServiceClient(conn)}, nil
}

func (c *grpcClient) UpsertParticipant(ctx context.Context, p Participant) error {
	participant := &compliancv1.Participant{
		UserId:              p.UserID,
		InstitutionName:     p.InstitutionName,
		Cnpj:                p.CNPJ,
		BankCode:            p.BankCode,
		CountryCode:         p.CountryCode,
		Role:                p.Role,
		WalletAddress:       p.WalletAddress,
		Status:              p.Status,
		CertificateData:     p.CertificateData,
		BlockchainPubKeyHex: p.BlockchainPubKeyHex,
		CsrPem:              p.CsrPem,
		PopNonce:            p.PopNonce,
	}
	if p.CertificateExpiry != nil {
		participant.CertificateExpiry = timestamppb.New(*p.CertificateExpiry)
	}
	if p.PopNonceExpiresAt != nil {
		participant.PopNonceExpiresAt = timestamppb.New(*p.PopNonceExpiresAt)
	}
	_, err := c.cc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{Participant: participant})
	return err
}

func (c *grpcClient) GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error) {
	resp, err := c.cc.GetParticipantByUser(ctx, &compliancv1.GetParticipantByUserRequest{UserId: userID})
	if err != nil {
		return Participant{}, false, err
	}
	if !resp.Found {
		return Participant{}, false, nil
	}
	pp := resp.Participant
	result := Participant{
		UserID:              pp.UserId,
		InstitutionName:     pp.InstitutionName,
		CNPJ:                pp.Cnpj,
		BankCode:            pp.BankCode,
		CountryCode:         pp.CountryCode,
		Role:                pp.Role,
		WalletAddress:       pp.WalletAddress,
		Status:              pp.Status,
		CertificateData:     pp.CertificateData,
		BlockchainPubKeyHex: pp.BlockchainPubKeyHex,
		CsrPem:              pp.CsrPem,
		PopNonce:            pp.PopNonce,
	}
	if pp.CertificateExpiry != nil {
		t := pp.CertificateExpiry.AsTime()
		result.CertificateExpiry = &t
	}
	if pp.PopNonceExpiresAt != nil {
		t := pp.PopNonceExpiresAt.AsTime()
		result.PopNonceExpiresAt = &t
	}
	return result, true, nil
}

func (c *grpcClient) CreateAuditLog(ctx context.Context, entry AuditEntry) error {
	_, err := c.cc.CreateAuditLog(ctx, &compliancv1.CreateAuditLogRequest{
		Entry: &compliancv1.AuditLogEntry{
			ActorSubject:  entry.ActorSubject,
			ActorAddress:  entry.ActorAddress,
			ActionType:    entry.ActionType,
			TargetSubject: entry.TargetSubject,
			CorrelationId: entry.CorrelationID,
			IpAddress:     entry.IPAddress,
			Result:        entry.Result,
			Category:      entry.Category,
			Severity:      entry.Severity,
			Details:       entry.Details,
		},
	})
	return err
}

func (c *grpcClient) IssueParticipantCertificate(ctx context.Context, userID, role, institutionName, cnpj string) (IssuedCertificate, error) {
	resp, err := c.cc.IssueParticipantCertificate(ctx, &compliancv1.IssueParticipantCertificateRequest{
		UserId:          userID,
		Role:            role,
		InstitutionName: institutionName,
		Cnpj:            cnpj,
	})
	if err != nil {
		return IssuedCertificate{}, err
	}
	return IssuedCertificate{
		CertPEM:    resp.CertPem,
		PrivKeyPEM: resp.PrivKeyPem,
		ExpiresAt:  resp.ExpiresAt,
	}, nil
}

func (c *grpcClient) ListParticipants(ctx context.Context, filter ParticipantFilter) ([]Participant, error) {
	req := &compliancv1.ListParticipantsRequest{Status: filter.Status}
	if filter.BankCode != "" {
		req.Search = "bank_code:" + filter.BankCode
	}
	resp, err := c.cc.ListParticipants(ctx, req)
	if err != nil {
		return nil, err
	}
	result := make([]Participant, 0, len(resp.Participants))
	for _, p := range resp.Participants {
		result = append(result, Participant{
			UserID:          p.UserId,
			InstitutionName: p.InstitutionName,
			CNPJ:            p.Cnpj,
			BankCode:        p.BankCode,
			CountryCode:     p.CountryCode,
			Role:            p.Role,
			WalletAddress:   p.WalletAddress,
			Status:          p.Status,
		})
	}
	return result, nil
}

func (c *grpcClient) SignParticipantCSR(ctx context.Context, csrPem, userID, role, institutionName, cnpj string) (SignedCSR, error) {
	resp, err := c.cc.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{
		CsrPem:          csrPem,
		UserId:          userID,
		Role:            role,
		InstitutionName: institutionName,
		Cnpj:            cnpj,
	})
	if err != nil {
		return SignedCSR{}, err
	}
	return SignedCSR{
		CertPEM:   resp.CertPem,
		ExpiresAt: resp.ExpiresAt,
	}, nil
}

func (c *grpcClient) ManageParticipantStatus(ctx context.Context, subject, status, reason string) error {
	_, err := c.cc.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{
		Subject: subject,
		Status:  status,
		Reason:  reason,
	})
	return err
}
