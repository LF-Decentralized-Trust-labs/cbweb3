// Package compliance provides a gRPC client adapter for the compliance-orchestrator.
// Used by the governance handler to manage participants, certificates, audit logs,
// circuit breaker, and system parameters.
package compliance

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const (
	listParticipantsMethod            = "/compliance.v1.ComplianceService/ListParticipants"
	upsertParticipantMethod           = "/compliance.v1.ComplianceService/UpsertParticipant"
	issueParticipantCertificateMethod = "/compliance.v1.ComplianceService/IssueParticipantCertificate"
	manageParticipantStatusMethod     = "/compliance.v1.ComplianceService/ManageParticipantStatus"
	getAuditLogsMethod                = "/compliance.v1.ComplianceService/GetAuditLogs"
	getCircuitBreakerStatusMethod     = "/compliance.v1.ComplianceService/GetCircuitBreakerStatus"
	toggleCircuitBreakerMethod        = "/compliance.v1.ComplianceService/ToggleCircuitBreaker"
	getSystemParametersMethod         = "/compliance.v1.ComplianceService/GetSystemParameters"
	updateSystemParametersMethod      = "/compliance.v1.ComplianceService/UpdateSystemParameters"
)

type jsonCodec struct{}

func (jsonCodec) Name() string                       { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// Participant is a simplified view used in HTTP handler responses.
type Participant struct {
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

type IssuedCertificate struct {
	CertPEM    string `json:"cert_pem"`
	PrivKeyPEM string `json:"priv_key_pem"`
	ExpiresAt  string `json:"expires_at"`
}

// GRPCAdapter is the api-gateway adapter for the compliance-orchestrator gRPC service.
type GRPCAdapter struct {
	cc    grpc.ClientConnInterface
	codec encoding.Codec
}

// NewGRPCAdapter connects to the compliance-orchestrator and returns a GRPCAdapter.
func NewGRPCAdapter(address string, timeout time.Duration) (*GRPCAdapter, error) {
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
	return &GRPCAdapter{cc: conn, codec: codec}, nil
}

func (a *GRPCAdapter) ListParticipants(ctx context.Context, statusFilter, search string) ([]Participant, error) {
	req := struct {
		Status string `json:"status,omitempty"`
		Search string `json:"search,omitempty"`
	}{Status: statusFilter, Search: search}

	var resp struct {
		Participants []Participant `json:"participants"`
	}
	if err := a.cc.Invoke(ctx, listParticipantsMethod, &req, &resp, grpc.ForceCodec(a.codec)); err != nil {
		return nil, err
	}
	return resp.Participants, nil
}

func (a *GRPCAdapter) RegisterParticipant(ctx context.Context, p Participant) error {
	req := struct {
		Participant Participant `json:"participant"`
	}{Participant: p}
	var resp struct{ Success bool `json:"success"` }
	return a.cc.Invoke(ctx, upsertParticipantMethod, &req, &resp, grpc.ForceCodec(a.codec))
}

func (a *GRPCAdapter) IssueParticipantCertificate(ctx context.Context, userID, role, institutionName, cnpj string) (IssuedCertificate, error) {
	req := struct {
		UserID          string `json:"user_id"`
		Role            string `json:"role"`
		InstitutionName string `json:"institution_name"`
		CNPJ            string `json:"cnpj"`
	}{UserID: userID, Role: role, InstitutionName: institutionName, CNPJ: cnpj}

	var resp IssuedCertificate
	if err := a.cc.Invoke(ctx, issueParticipantCertificateMethod, &req, &resp, grpc.ForceCodec(a.codec)); err != nil {
		return IssuedCertificate{}, err
	}
	return resp, nil
}

func (a *GRPCAdapter) ManageParticipantStatus(ctx context.Context, subject, statusVal, reason string) error {
	req := struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
	}{Subject: subject, Status: statusVal, Reason: reason}
	var resp struct{ Subject string `json:"subject"` }
	return a.cc.Invoke(ctx, manageParticipantStatusMethod, &req, &resp, grpc.ForceCodec(a.codec))
}

func (a *GRPCAdapter) GetAuditLogs(ctx context.Context, category, severity, fromDate, toDate string, page, limit int) ([]AuditRecord, error) {
	req := struct {
		Category string `json:"category,omitempty"`
		Severity string `json:"severity,omitempty"`
		FromDate string `json:"from_date,omitempty"`
		ToDate   string `json:"to_date,omitempty"`
		Page     int    `json:"page,omitempty"`
		Limit    int    `json:"limit,omitempty"`
	}{Category: category, Severity: severity, FromDate: fromDate, ToDate: toDate, Page: page, Limit: limit}

	var resp struct {
		Logs []AuditRecord `json:"logs"`
	}
	if err := a.cc.Invoke(ctx, getAuditLogsMethod, &req, &resp, grpc.ForceCodec(a.codec)); err != nil {
		return nil, err
	}
	return resp.Logs, nil
}

func (a *GRPCAdapter) GetCircuitBreakerStatus(ctx context.Context) (CircuitBreakerStatus, error) {
	var resp CircuitBreakerStatus
	if err := a.cc.Invoke(ctx, getCircuitBreakerStatusMethod, struct{}{}, &resp, grpc.ForceCodec(a.codec)); err != nil {
		return CircuitBreakerStatus{}, err
	}
	return resp, nil
}

func (a *GRPCAdapter) ToggleCircuitBreaker(ctx context.Context, pause bool, reason string) (bool, error) {
	req := struct {
		Pause  bool   `json:"pause"`
		Reason string `json:"reason"`
	}{Pause: pause, Reason: reason}
	var resp struct {
		IsPaused bool   `json:"is_paused"`
		TxHash   string `json:"tx_hash,omitempty"`
	}
	if err := a.cc.Invoke(ctx, toggleCircuitBreakerMethod, &req, &resp, grpc.ForceCodec(a.codec)); err != nil {
		return false, err
	}
	return resp.IsPaused, nil
}

func (a *GRPCAdapter) GetSystemParameters(ctx context.Context) (SystemParameters, error) {
	var resp SystemParameters
	if err := a.cc.Invoke(ctx, getSystemParametersMethod, struct{}{}, &resp, grpc.ForceCodec(a.codec)); err != nil {
		return SystemParameters{}, err
	}
	return resp, nil
}

func (a *GRPCAdapter) UpdateSystemParameters(ctx context.Context, params SystemParameters, reason, actorSubject string) error {
	req := struct {
		TransactionMinimum string  `json:"transaction_minimum"`
		TransactionMaximum string  `json:"transaction_maximum"`
		SlippageTolerance  float64 `json:"slippage_tolerance"`
		SettlementWindow   int64   `json:"settlement_window"`
		Reason             string  `json:"reason"`
		ActorSubject       string  `json:"actor_subject"`
	}{
		TransactionMinimum: params.TransactionMinimum,
		TransactionMaximum: params.TransactionMaximum,
		SlippageTolerance:  params.SlippageTolerance,
		SettlementWindow:   params.SettlementWindow,
		Reason:             reason,
		ActorSubject:       actorSubject,
	}
	var resp struct{ Success bool `json:"success"` }
	return a.cc.Invoke(ctx, updateSystemParametersMethod, &req, &resp, grpc.ForceCodec(a.codec))
}
