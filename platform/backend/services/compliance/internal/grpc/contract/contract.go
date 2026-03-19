package contract

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

const (
	ServiceName = "compliance.v1.ComplianceService"

	// Participant persistence (previously in data-access)
	UpsertParticipantMethod    = "/compliance.v1.ComplianceService/UpsertParticipant"
	GetParticipantByUserMethod = "/compliance.v1.ComplianceService/GetParticipantByUser"
	ListParticipantsMethod     = "/compliance.v1.ComplianceService/ListParticipants"

	// Audit log
	CreateAuditLogMethod = "/compliance.v1.ComplianceService/CreateAuditLog"
	GetAuditLogsMethod   = "/compliance.v1.ComplianceService/GetAuditLogs"

	// PKI certificate issuance
	IssueParticipantCertificateMethod = "/compliance.v1.ComplianceService/IssueParticipantCertificate"

	// Governance operations
	ApproveKYCMethod              = "/compliance.v1.ComplianceService/ApproveKYC"
	ManageParticipantStatusMethod = "/compliance.v1.ComplianceService/ManageParticipantStatus"
	GetCircuitBreakerStatusMethod = "/compliance.v1.ComplianceService/GetCircuitBreakerStatus"
	ToggleCircuitBreakerMethod    = "/compliance.v1.ComplianceService/ToggleCircuitBreaker"
	GetSystemParametersMethod     = "/compliance.v1.ComplianceService/GetSystemParameters"
	UpdateSystemParametersMethod  = "/compliance.v1.ComplianceService/UpdateSystemParameters"
)

// --- Participant ---

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

type UpsertParticipantRequest struct {
	Participant Participant `json:"participant"`
}

type UpsertParticipantResponse struct {
	Success bool `json:"success"`
}

type GetParticipantByUserRequest struct {
	UserID string `json:"user_id"`
}

type GetParticipantByUserResponse struct {
	Found       bool        `json:"found"`
	Participant Participant `json:"participant,omitempty"`
}

type ListParticipantsRequest struct {
	Status string `json:"status,omitempty"`
	Search string `json:"search,omitempty"`
}

type ListParticipantsResponse struct {
	Participants []Participant `json:"participants"`
}

// --- Audit Log ---

type AuditLogEntry struct {
	ActorSubject  string `json:"actor_subject"`
	ActorAddress  string `json:"actor_address"`
	ActionType    string `json:"action_type"`
	TargetSubject string `json:"target_subject"`
	CorrelationID string `json:"correlation_id"`
	IPAddress     string `json:"ip_address"`
	Result        string `json:"result"`
	Category      string `json:"category"`
	Severity      string `json:"severity"`
	Details       string `json:"details,omitempty"`
}

type CreateAuditLogRequest struct {
	Entry AuditLogEntry `json:"entry"`
}

type CreateAuditLogResponse struct {
	Success bool `json:"success"`
}

type GetAuditLogsRequest struct {
	Category  string `json:"category,omitempty"`
	Severity  string `json:"severity,omitempty"`
	FromDate  string `json:"from_date,omitempty"` // ISO-8601 UTC
	ToDate    string `json:"to_date,omitempty"`   // ISO-8601 UTC
	Page      int32  `json:"page,omitempty"`
	Limit     int32  `json:"limit,omitempty"`
}

type AuditLogRecord struct {
	LogID         string `json:"log_id"`
	Timestamp     string `json:"timestamp"`
	ActorSubject  string `json:"actor"`
	ActorAddress  string `json:"actor_address"`
	ActionType    string `json:"action"`
	TargetSubject string `json:"target_subject,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Category      string `json:"category"`
	Severity      string `json:"severity"`
	Result        string `json:"outcome"`
	Details       string `json:"details,omitempty"`
}

type GetAuditLogsResponse struct {
	Logs []AuditLogRecord `json:"logs"`
}

// --- PKI ---

type IssueParticipantCertificateRequest struct {
	UserID          string `json:"user_id"`
	Role            string `json:"role"`
	InstitutionName string `json:"institution_name"`
	CNPJ            string `json:"cnpj"`
}

type IssueParticipantCertificateResponse struct {
	CertPEM    string `json:"cert_pem"`
	PrivKeyPEM string `json:"priv_key_pem"` // caller must persist securely
	ExpiresAt  string `json:"expires_at"`   // ISO-8601 UTC
}

// --- Governance ---

type ApproveKYCRequest struct {
	Subject      string `json:"subject"`       // Keycloak user ID
	ActorSubject string `json:"actor_subject"` // Who is approving
	Reason       string `json:"reason,omitempty"`
}

type ApproveKYCResponse struct {
	Subject string `json:"subject"`
	Status  string `json:"status"`  // ACTIVE
	TxHash  string `json:"tx_hash"` // on-chain transaction hash
}

type ManageParticipantStatusRequest struct {
	Subject string `json:"subject"`
	Status  string `json:"status"` // ACTIVE | FROZEN | REVOKED
	Reason  string `json:"reason"`
}

type ManageParticipantStatusResponse struct {
	Subject string `json:"subject"`
	Status  string `json:"status"`
}

type GetCircuitBreakerStatusResponse struct {
	IsPaused   bool   `json:"is_paused"`
	LastUpdate string `json:"last_update"` // ISO-8601 UTC
	UpdatedBy  string `json:"updated_by"`
}

type ToggleCircuitBreakerRequest struct {
	Pause  bool   `json:"pause"`
	Reason string `json:"reason"`
}

type ToggleCircuitBreakerResponse struct {
	IsPaused bool   `json:"is_paused"`
	TxHash   string `json:"tx_hash,omitempty"`
}

type GetSystemParametersResponse struct {
	TransactionMinimum string  `json:"transaction_minimum"`
	TransactionMaximum string  `json:"transaction_maximum"`
	SlippageTolerance  float64 `json:"slippage_tolerance"`
	SettlementWindow   int64   `json:"settlement_window"`
}

type UpdateSystemParametersRequest struct {
	TransactionMinimum string  `json:"transaction_minimum"`
	TransactionMaximum string  `json:"transaction_maximum"`
	SlippageTolerance  float64 `json:"slippage_tolerance"`
	SettlementWindow   int64   `json:"settlement_window"`
	Reason             string  `json:"reason"`
	ActorSubject       string  `json:"actor_subject"`
}

type UpdateSystemParametersResponse struct {
	Success bool `json:"success"`
}

// --- Client Interface ---

type ComplianceServiceClient interface {
	UpsertParticipant(ctx context.Context, in *UpsertParticipantRequest, opts ...grpc.CallOption) (*UpsertParticipantResponse, error)
	GetParticipantByUser(ctx context.Context, in *GetParticipantByUserRequest, opts ...grpc.CallOption) (*GetParticipantByUserResponse, error)
	ListParticipants(ctx context.Context, in *ListParticipantsRequest, opts ...grpc.CallOption) (*ListParticipantsResponse, error)
	CreateAuditLog(ctx context.Context, in *CreateAuditLogRequest, opts ...grpc.CallOption) (*CreateAuditLogResponse, error)
	GetAuditLogs(ctx context.Context, in *GetAuditLogsRequest, opts ...grpc.CallOption) (*GetAuditLogsResponse, error)
	IssueParticipantCertificate(ctx context.Context, in *IssueParticipantCertificateRequest, opts ...grpc.CallOption) (*IssueParticipantCertificateResponse, error)
	ManageParticipantStatus(ctx context.Context, in *ManageParticipantStatusRequest, opts ...grpc.CallOption) (*ManageParticipantStatusResponse, error)
	GetCircuitBreakerStatus(ctx context.Context, opts ...grpc.CallOption) (*GetCircuitBreakerStatusResponse, error)
	ToggleCircuitBreaker(ctx context.Context, in *ToggleCircuitBreakerRequest, opts ...grpc.CallOption) (*ToggleCircuitBreakerResponse, error)
	GetSystemParameters(ctx context.Context, opts ...grpc.CallOption) (*GetSystemParametersResponse, error)
	UpdateSystemParameters(ctx context.Context, in *UpdateSystemParametersRequest, opts ...grpc.CallOption) (*UpdateSystemParametersResponse, error)
}

type complianceServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewComplianceServiceClient(cc grpc.ClientConnInterface) ComplianceServiceClient {
	return &complianceServiceClient{cc: cc}
}

func (c *complianceServiceClient) UpsertParticipant(ctx context.Context, in *UpsertParticipantRequest, opts ...grpc.CallOption) (*UpsertParticipantResponse, error) {
	out := new(UpsertParticipantResponse)
	return out, c.cc.Invoke(ctx, UpsertParticipantMethod, in, out, opts...)
}

func (c *complianceServiceClient) GetParticipantByUser(ctx context.Context, in *GetParticipantByUserRequest, opts ...grpc.CallOption) (*GetParticipantByUserResponse, error) {
	out := new(GetParticipantByUserResponse)
	return out, c.cc.Invoke(ctx, GetParticipantByUserMethod, in, out, opts...)
}

func (c *complianceServiceClient) ListParticipants(ctx context.Context, in *ListParticipantsRequest, opts ...grpc.CallOption) (*ListParticipantsResponse, error) {
	out := new(ListParticipantsResponse)
	return out, c.cc.Invoke(ctx, ListParticipantsMethod, in, out, opts...)
}

func (c *complianceServiceClient) CreateAuditLog(ctx context.Context, in *CreateAuditLogRequest, opts ...grpc.CallOption) (*CreateAuditLogResponse, error) {
	out := new(CreateAuditLogResponse)
	return out, c.cc.Invoke(ctx, CreateAuditLogMethod, in, out, opts...)
}

func (c *complianceServiceClient) GetAuditLogs(ctx context.Context, in *GetAuditLogsRequest, opts ...grpc.CallOption) (*GetAuditLogsResponse, error) {
	out := new(GetAuditLogsResponse)
	return out, c.cc.Invoke(ctx, GetAuditLogsMethod, in, out, opts...)
}

func (c *complianceServiceClient) IssueParticipantCertificate(ctx context.Context, in *IssueParticipantCertificateRequest, opts ...grpc.CallOption) (*IssueParticipantCertificateResponse, error) {
	out := new(IssueParticipantCertificateResponse)
	return out, c.cc.Invoke(ctx, IssueParticipantCertificateMethod, in, out, opts...)
}

func (c *complianceServiceClient) ManageParticipantStatus(ctx context.Context, in *ManageParticipantStatusRequest, opts ...grpc.CallOption) (*ManageParticipantStatusResponse, error) {
	out := new(ManageParticipantStatusResponse)
	return out, c.cc.Invoke(ctx, ManageParticipantStatusMethod, in, out, opts...)
}

func (c *complianceServiceClient) GetCircuitBreakerStatus(ctx context.Context, opts ...grpc.CallOption) (*GetCircuitBreakerStatusResponse, error) {
	out := new(GetCircuitBreakerStatusResponse)
	return out, c.cc.Invoke(ctx, GetCircuitBreakerStatusMethod, struct{}{}, out, opts...)
}

func (c *complianceServiceClient) ToggleCircuitBreaker(ctx context.Context, in *ToggleCircuitBreakerRequest, opts ...grpc.CallOption) (*ToggleCircuitBreakerResponse, error) {
	out := new(ToggleCircuitBreakerResponse)
	return out, c.cc.Invoke(ctx, ToggleCircuitBreakerMethod, in, out, opts...)
}

func (c *complianceServiceClient) GetSystemParameters(ctx context.Context, opts ...grpc.CallOption) (*GetSystemParametersResponse, error) {
	out := new(GetSystemParametersResponse)
	return out, c.cc.Invoke(ctx, GetSystemParametersMethod, struct{}{}, out, opts...)
}

func (c *complianceServiceClient) UpdateSystemParameters(ctx context.Context, in *UpdateSystemParametersRequest, opts ...grpc.CallOption) (*UpdateSystemParametersResponse, error) {
	out := new(UpdateSystemParametersResponse)
	return out, c.cc.Invoke(ctx, UpdateSystemParametersMethod, in, out, opts...)
}
