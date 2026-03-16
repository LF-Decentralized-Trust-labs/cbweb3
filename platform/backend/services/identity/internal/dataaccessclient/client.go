package dataaccessclient

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const (
	upsertParticipantMethod    = "/dataaccess.v1.DataAccessService/UpsertParticipant"
	getParticipantByUserMethod = "/dataaccess.v1.DataAccessService/GetParticipantByUser"
	upsertKYCCredentialMethod  = "/dataaccess.v1.DataAccessService/UpsertKYCCredential"
	createAuditLogMethod       = "/dataaccess.v1.DataAccessService/CreateAuditLog"
)

// Participant holds all participant metadata (D7 §7.1).
type Participant struct {
	UserID          string
	DID             string
	WalletAddress   string
	Country         string
	BankCode        string
	Role            string
	InstitutionName string
}

// KYCCredential stores the issued VC pointer for a participant.
type KYCCredential struct {
	Subject    string
	ZKPPointer string
	VCJWT      string
	IssuedAt   string
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
	Details       string
}

// Client defines the data-access gRPC operations used by identity service.
type Client interface {
	UpsertParticipant(ctx context.Context, p Participant) error
	GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error)
	UpsertKYCCredential(ctx context.Context, cred KYCCredential) error
	CreateAuditLog(ctx context.Context, entry AuditEntry) error
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

type grpcClient struct {
	cc    grpc.ClientConnInterface
	codec encoding.Codec
}

func New(address string, timeout time.Duration) (Client, error) {
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
	return &grpcClient{cc: conn, codec: codec}, nil
}

func (c *grpcClient) UpsertParticipant(ctx context.Context, p Participant) error {
	req := struct {
		Participant struct {
			UserID          string `json:"user_id"`
			DID             string `json:"did"`
			WalletAddress   string `json:"wallet_address"`
			Country         string `json:"country"`
			BankCode        string `json:"bank_code"`
			Role            string `json:"role"`
			InstitutionName string `json:"institution_name"`
		} `json:"participant"`
	}{}
	req.Participant.UserID = p.UserID
	req.Participant.DID = p.DID
	req.Participant.WalletAddress = p.WalletAddress
	req.Participant.Country = p.Country
	req.Participant.BankCode = p.BankCode
	req.Participant.Role = p.Role
	req.Participant.InstitutionName = p.InstitutionName
	var resp struct {
		Success bool `json:"success"`
	}
	return c.cc.Invoke(ctx, upsertParticipantMethod, &req, &resp, grpc.ForceCodec(c.codec))
}

func (c *grpcClient) UpsertKYCCredential(ctx context.Context, cred KYCCredential) error {
	req := struct {
		Subject    string `json:"subject"`
		ZKPPointer string `json:"zkp_pointer"`
		VCJWT      string `json:"vc_jwt"`
		IssuedAt   string `json:"issued_at"`
	}{
		Subject:    cred.Subject,
		ZKPPointer: cred.ZKPPointer,
		VCJWT:      cred.VCJWT,
		IssuedAt:   cred.IssuedAt,
	}
	var resp struct {
		Success bool `json:"success"`
	}
	return c.cc.Invoke(ctx, upsertKYCCredentialMethod, &req, &resp, grpc.ForceCodec(c.codec))
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
	req.Entry.Details = entry.Details
	var resp struct {
		Success bool `json:"success"`
	}
	return c.cc.Invoke(ctx, createAuditLogMethod, &req, &resp, grpc.ForceCodec(c.codec))
}

func (c *grpcClient) GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error) {
	req := struct {
		UserID string `json:"user_id"`
	}{UserID: userID}
	var resp struct {
		Found       bool `json:"found"`
		Participant struct {
			UserID          string `json:"user_id"`
			DID             string `json:"did"`
			WalletAddress   string `json:"wallet_address"`
			Country         string `json:"country"`
			BankCode        string `json:"bank_code"`
			Role            string `json:"role"`
			InstitutionName string `json:"institution_name"`
		} `json:"participant"`
	}
	if err := c.cc.Invoke(ctx, getParticipantByUserMethod, &req, &resp, grpc.ForceCodec(c.codec)); err != nil {
		return Participant{}, false, err
	}
	if !resp.Found {
		return Participant{}, false, nil
	}
	return Participant{
		UserID:          resp.Participant.UserID,
		DID:             resp.Participant.DID,
		WalletAddress:   resp.Participant.WalletAddress,
		Country:         resp.Participant.Country,
		BankCode:        resp.Participant.BankCode,
		Role:            resp.Participant.Role,
		InstitutionName: resp.Participant.InstitutionName,
	}, true, nil
}
