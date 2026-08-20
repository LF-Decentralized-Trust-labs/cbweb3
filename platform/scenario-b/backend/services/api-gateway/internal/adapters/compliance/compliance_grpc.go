// SPDX-License-Identifier: Apache-2.0

// Package compliance provides a gRPC client adapter for the compliance-orchestrator.
// Used by the governance handler to manage participants, certificates, audit logs,
// circuit breaker, and system parameters.
package compliance

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/grpcx"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/deadlines"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Participant is a simplified view used in HTTP handler responses.
type Participant struct {
	UserID            string     `json:"user_id"`
	InstitutionName   string     `json:"institution_name"`
	LegalEntityID     string     `json:"legal_entity_id"`
	BankCode          string     `json:"bank_code"`
	CountryCode       string     `json:"country_code"`
	Role              string     `json:"role"`
	WalletAddress     string     `json:"wallet_address"`
	Status            string     `json:"status"`
	CertificateData   string     `json:"certificate_data,omitempty"`
	CertificateExpiry *time.Time `json:"certificate_expiry,omitempty"`
}

type AuditRecord struct {
	LogID         string `json:"log_id"`
	Timestamp     string `json:"timestamp"`
	ActorSubject  string `json:"actor"`
	ActorAddress  string `json:"actor_address"`
	ActionType    string `json:"action"`
	TargetSubject string `json:"target_subject,omitempty"`
	Category      string `json:"category"`
	Severity      string `json:"severity"`
	Result        string `json:"outcome"`
	Details       string `json:"details,omitempty"`
}

type SystemParameters struct {
	TransactionMinimum string  `json:"transaction_minimum"`
	TransactionMaximum string  `json:"transaction_maximum"`
	SlippageTolerance  float64 `json:"slippage_tolerance"`
	SettlementWindow   int64   `json:"settlement_window"`
}

type CircuitBreakerStatus struct {
	IsPaused   bool   `json:"is_paused"`
	LastUpdate string `json:"last_update"`
	UpdatedBy  string `json:"updated_by"`
}

// SignedCSRResult holds the CA-signed certificate returned from a CSR submission.
type SignedCSRResult struct {
	CertPEM   string `json:"cert_pem"`
	ExpiresAt string `json:"expires_at"`
}

// ApproveKYCResult holds the result of an ApproveKYC call.
type ApproveKYCResult struct {
	Subject  string
	Status   string
	TxHash   string
	PopNonce string
}

// GRPCAdapter is the api-gateway adapter for the compliance-orchestrator gRPC service.
type GRPCAdapter struct {
	conn *grpc.ClientConn
	cc   compliancv1.ComplianceServiceClient
}

// NewGRPCAdapter connects to the compliance-orchestrator and returns a GRPCAdapter.
func NewGRPCAdapter(address string, timeout time.Duration) (*GRPCAdapter, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	credOpt, err := authz.ClientDialOptionFromEnv("compliance")
	if err != nil {
		return nil, err
	}
	//nolint:staticcheck
	conn, err := grpc.DialContext(
		dialCtx,
		address,
		credOpt,
		grpc.WithChainUnaryInterceptor(grpcx.WithDefaultDeadline(deadlines.Compliance)),
	)
	if err != nil {
		return nil, err
	}
	return &GRPCAdapter{conn: conn, cc: compliancv1.NewComplianceServiceClient(conn)}, nil
}

// Close releases the underlying gRPC connection.
func (a *GRPCAdapter) Close() error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

func (a *GRPCAdapter) ListParticipants(ctx context.Context, statusFilter, search string) ([]Participant, error) {
	resp, err := a.cc.ListParticipants(ctx, &compliancv1.ListParticipantsRequest{
		Status: statusFilter,
		Search: search,
	})
	if err != nil {
		return nil, err
	}
	result := make([]Participant, 0, len(resp.Participants))
	for _, p := range resp.Participants {
		part := Participant{
			UserID:          p.UserId,
			InstitutionName: p.InstitutionName,
			LegalEntityID:   p.LegalEntityId,
			BankCode:        p.BankCode,
			CountryCode:     p.CountryCode,
			Role:            p.Role,
			WalletAddress:   p.WalletAddress,
			Status:          p.Status,
			CertificateData: p.CertificateData,
		}
		if p.CertificateExpiry != nil {
			t := p.CertificateExpiry.AsTime()
			part.CertificateExpiry = &t
		}
		result = append(result, part)
	}
	return result, nil
}

func (a *GRPCAdapter) RegisterParticipant(ctx context.Context, p Participant) error {
	participant := &compliancv1.Participant{
		UserId:          p.UserID,
		InstitutionName: p.InstitutionName,
		LegalEntityId:   p.LegalEntityID,
		BankCode:        p.BankCode,
		CountryCode:     p.CountryCode,
		Role:            p.Role,
		WalletAddress:   p.WalletAddress,
		Status:          p.Status,
		CertificateData: p.CertificateData,
	}
	if p.CertificateExpiry != nil {
		participant.CertificateExpiry = timestamppb.New(*p.CertificateExpiry)
	}
	_, err := a.cc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{Participant: participant})
	return err
}

// RegisterParticipantOnChain registers a wallet on the IdentityRegistry via the
// compliance service (the compliance signer must hold GOVERNANCE_ROLE). Returns
// the tx hash (empty when the wallet was already registered) and the
// already-registered flag. Machine-to-machine self-registration path.
func (a *GRPCAdapter) RegisterParticipantOnChain(ctx context.Context, walletAddress, institutionName, role, bankCode string) (txHash string, alreadyRegistered bool, err error) {
	resp, err := a.cc.RegisterParticipantOnChain(ctx, &compliancv1.RegisterParticipantOnChainRequest{
		WalletAddress:   walletAddress,
		InstitutionName: institutionName,
		Role:            role,
		BankCode:        bankCode,
	})
	if err != nil {
		return "", false, err
	}
	return resp.TxHash, resp.AlreadyRegistered, nil
}

// RegisterCurrencyOnChain asks the hub compliance service to deploy a founding
// central bank's bridge token (W-token) and register its sovereign currency
// on-chain. Returns the W-token symbol, deployed token address (empty when
// already registered), the already-registered flag, and the last tx hash.
func (a *GRPCAdapter) RegisterCurrencyOnChain(ctx context.Context, currency, cbAddress, spokeID string) (symbol, tokenAddr string, alreadyRegistered bool, txHash string, err error) {
	resp, err := a.cc.RegisterCurrencyOnChain(ctx, &compliancv1.RegisterCurrencyOnChainRequest{
		Currency:  currency,
		CbAddress: cbAddress,
		SpokeId:   spokeID,
	})
	if err != nil {
		return "", "", false, "", err
	}
	return resp.Symbol, resp.TokenAddress, resp.AlreadyRegistered, resp.TxHash, nil
}

// RegisterPairOnChain asks the hub compliance service to deploy the sovereign-
// pair AMM over two already-registered W-tokens and register the pair
// (proposePair + confirmPair). Returns the deployed AMM address (empty when
// already registered), the already-registered flag, and the last tx hash.
func (a *GRPCAdapter) RegisterPairOnChain(ctx context.Context, currencyA, currencyB, pairID string) (ammAddr string, alreadyRegistered bool, txHash string, err error) {
	resp, err := a.cc.RegisterPairOnChain(ctx, &compliancv1.RegisterPairOnChainRequest{
		CurrencyA: currencyA,
		CurrencyB: currencyB,
		PairId:    pairID,
	})
	if err != nil {
		return "", false, "", err
	}
	return resp.AmmAddress, resp.AlreadyRegistered, resp.TxHash, nil
}

// SignParticipantCSR submits a PKCS#10 CSR to the compliance-orchestrator for
// signing by the CA. The participant record is created/updated automatically.
func (a *GRPCAdapter) SignParticipantCSR(ctx context.Context, csrPEM, userID, role, institutionName, legal_entity_id string) (SignedCSRResult, error) {
	resp, err := a.cc.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{
		CsrPem:          csrPEM,
		UserId:          userID,
		Role:            role,
		InstitutionName: institutionName,
		LegalEntityId:   legal_entity_id,
	})
	if err != nil {
		return SignedCSRResult{}, err
	}
	return SignedCSRResult{CertPEM: resp.CertPem, ExpiresAt: resp.ExpiresAt}, nil
}

func (a *GRPCAdapter) ApproveKYC(ctx context.Context, subject, actorSubject, reason string) (ApproveKYCResult, error) {
	resp, err := a.cc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{
		Subject:      subject,
		ActorSubject: actorSubject,
		Reason:       reason,
	})
	if err != nil {
		return ApproveKYCResult{}, err
	}
	return ApproveKYCResult{Subject: resp.Subject, Status: resp.Status, TxHash: resp.TxHash, PopNonce: resp.PopNonce}, nil
}

func (a *GRPCAdapter) ManageParticipantStatus(ctx context.Context, subject, statusVal, reason string) error {
	_, err := a.cc.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{
		Subject: subject,
		Status:  statusVal,
		Reason:  reason,
	})
	return err
}

func (a *GRPCAdapter) GetAuditLogs(ctx context.Context, category, severity, fromDate, toDate string, page, limit int) ([]AuditRecord, error) {
	resp, err := a.cc.GetAuditLogs(ctx, &compliancv1.GetAuditLogsRequest{
		Category: category,
		Severity: severity,
		FromDate: fromDate,
		ToDate:   toDate,
		Page:     int32(page),  // #nosec G115 -- pagination value, validated/small; fits int32.
		Limit:    int32(limit), // #nosec G115 -- pagination value, validated/small; fits int32.
	})
	if err != nil {
		return nil, err
	}
	result := make([]AuditRecord, 0, len(resp.Logs))
	for _, r := range resp.Logs {
		result = append(result, AuditRecord{
			LogID:         r.LogId,
			Timestamp:     r.Timestamp,
			ActorSubject:  r.ActorSubject,
			ActorAddress:  r.ActorAddress,
			ActionType:    r.ActionType,
			TargetSubject: r.TargetSubject,
			Category:      r.Category,
			Severity:      r.Severity,
			Result:        r.Result,
			Details:       r.Details,
		})
	}
	return result, nil
}

func (a *GRPCAdapter) GetCircuitBreakerStatus(ctx context.Context) (CircuitBreakerStatus, error) {
	resp, err := a.cc.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return CircuitBreakerStatus{}, err
	}
	return CircuitBreakerStatus{
		IsPaused:   resp.IsPaused,
		LastUpdate: resp.LastUpdate,
		UpdatedBy:  resp.UpdatedBy,
	}, nil
}

func (a *GRPCAdapter) ToggleCircuitBreaker(ctx context.Context, pause bool, reason string) (bool, error) {
	resp, err := a.cc.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{
		Pause:  pause,
		Reason: reason,
	})
	if err != nil {
		return false, err
	}
	return resp.IsPaused, nil
}

func (a *GRPCAdapter) GetSystemParameters(ctx context.Context) (SystemParameters, error) {
	resp, err := a.cc.GetSystemParameters(ctx, &emptypb.Empty{})
	if err != nil {
		return SystemParameters{}, err
	}
	return SystemParameters{
		TransactionMinimum: resp.TransactionMinimum,
		TransactionMaximum: resp.TransactionMaximum,
		SlippageTolerance:  resp.SlippageTolerance,
		SettlementWindow:   resp.SettlementWindow,
	}, nil
}

func (a *GRPCAdapter) UpdateSystemParameters(ctx context.Context, params SystemParameters, reason, actorSubject string) error {
	_, err := a.cc.UpdateSystemParameters(ctx, &compliancv1.UpdateSystemParametersRequest{
		TransactionMinimum: params.TransactionMinimum,
		TransactionMaximum: params.TransactionMaximum,
		SlippageTolerance:  params.SlippageTolerance,
		SettlementWindow:   params.SettlementWindow,
		Reason:             reason,
		ActorSubject:       actorSubject,
	})
	return err
}
