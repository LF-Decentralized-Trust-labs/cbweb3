package contract

import (
	"context"

	"google.golang.org/grpc"
)

const (
	ServiceName = "dataaccess.v1.DataAccessService"

	UpsertParticipantMethod    = "/dataaccess.v1.DataAccessService/UpsertParticipant"
	GetParticipantByUserMethod = "/dataaccess.v1.DataAccessService/GetParticipantByUser"
	UpsertKYCCredentialMethod  = "/dataaccess.v1.DataAccessService/UpsertKYCCredential"
	CreateAuditLogMethod       = "/dataaccess.v1.DataAccessService/CreateAuditLog"
)

// Participant carries the enriched participant fields (D7 §7.1).
type Participant struct {
	UserID          string `json:"user_id"`
	DID             string `json:"did"`
	WalletAddress   string `json:"wallet_address"`
	Country         string `json:"country"`
	BankCode        string `json:"bank_code"`
	Role            string `json:"role"`
	InstitutionName string `json:"institution_name"`
	WalletType      string `json:"wallet_type"`
	SignerProvider  string `json:"signer_provider"`
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
	Participant Participant `json:"participant"`
}

// KYCCredential carries the VC pointer for a participant.
type KYCCredential struct {
	Subject    string `json:"subject"`
	ZKPPointer string `json:"zkp_pointer"`
	VCJWT      string `json:"vc_jwt"`
	IssuedAt   string `json:"issued_at"`
}

type UpsertKYCCredentialRequest struct {
	Subject    string `json:"subject"`
	ZKPPointer string `json:"zkp_pointer"`
	VCJWT      string `json:"vc_jwt"`
	IssuedAt   string `json:"issued_at"`
}

type UpsertKYCCredentialResponse struct {
	Success bool `json:"success"`
}

// AuditLogEntry carries a single audit event (REQ-COM-006).
type AuditLogEntry struct {
	ActorSubject  string `json:"actor_subject"`
	ActorAddress  string `json:"actor_address"`
	ActionType    string `json:"action_type"`
	TargetSubject string `json:"target_subject"`
	CorrelationID string `json:"correlation_id"`
	IPAddress     string `json:"ip_address"`
	Result        string `json:"result"`
	Details       string `json:"details"`
}

type CreateAuditLogRequest struct {
	Entry AuditLogEntry `json:"entry"`
}

type CreateAuditLogResponse struct {
	Success bool `json:"success"`
}

// Client is the typed gRPC client interface for data-access.
type Client interface {
	UpsertParticipant(ctx context.Context, in *UpsertParticipantRequest, opts ...grpc.CallOption) (*UpsertParticipantResponse, error)
	GetParticipantByUser(ctx context.Context, in *GetParticipantByUserRequest, opts ...grpc.CallOption) (*GetParticipantByUserResponse, error)
	UpsertKYCCredential(ctx context.Context, in *UpsertKYCCredentialRequest, opts ...grpc.CallOption) (*UpsertKYCCredentialResponse, error)
	CreateAuditLog(ctx context.Context, in *CreateAuditLogRequest, opts ...grpc.CallOption) (*CreateAuditLogResponse, error)
}

type client struct {
	cc grpc.ClientConnInterface
}

func NewClient(cc grpc.ClientConnInterface) Client {
	return &client{cc: cc}
}

func (c *client) UpsertParticipant(ctx context.Context, in *UpsertParticipantRequest, opts ...grpc.CallOption) (*UpsertParticipantResponse, error) {
	out := new(UpsertParticipantResponse)
	return out, c.cc.Invoke(ctx, UpsertParticipantMethod, in, out, opts...)
}

func (c *client) GetParticipantByUser(ctx context.Context, in *GetParticipantByUserRequest, opts ...grpc.CallOption) (*GetParticipantByUserResponse, error) {
	out := new(GetParticipantByUserResponse)
	return out, c.cc.Invoke(ctx, GetParticipantByUserMethod, in, out, opts...)
}

func (c *client) UpsertKYCCredential(ctx context.Context, in *UpsertKYCCredentialRequest, opts ...grpc.CallOption) (*UpsertKYCCredentialResponse, error) {
	out := new(UpsertKYCCredentialResponse)
	return out, c.cc.Invoke(ctx, UpsertKYCCredentialMethod, in, out, opts...)
}

func (c *client) CreateAuditLog(ctx context.Context, in *CreateAuditLogRequest, opts ...grpc.CallOption) (*CreateAuditLogResponse, error) {
	out := new(CreateAuditLogResponse)
	return out, c.cc.Invoke(ctx, CreateAuditLogMethod, in, out, opts...)
}
