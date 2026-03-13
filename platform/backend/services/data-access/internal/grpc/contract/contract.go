package contract

import (
	"context"

	"google.golang.org/grpc"
)

const (
	ServiceName = "dataaccess.v1.DataAccessService"

	UpsertParticipantMethod    = "/dataaccess.v1.DataAccessService/UpsertParticipant"
	GetParticipantByUserMethod = "/dataaccess.v1.DataAccessService/GetParticipantByUser"
)

type Participant struct {
	UserID         string `json:"user_id"`
	DID            string `json:"did"`
	WalletAddress  string `json:"wallet_address"`
	Country        string `json:"country"`
	BankCode       string `json:"bank_code"`
	Role           string `json:"role"`
	SignerProvider string `json:"signer_provider"`
	KMSKeyID       string `json:"kms_key_id"`
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

type Client interface {
	UpsertParticipant(ctx context.Context, in *UpsertParticipantRequest, opts ...grpc.CallOption) (*UpsertParticipantResponse, error)
	GetParticipantByUser(ctx context.Context, in *GetParticipantByUserRequest, opts ...grpc.CallOption) (*GetParticipantByUserResponse, error)
}

type client struct {
	cc grpc.ClientConnInterface
}

func NewClient(cc grpc.ClientConnInterface) Client {
	return &client{cc: cc}
}

func (c *client) UpsertParticipant(ctx context.Context, in *UpsertParticipantRequest, opts ...grpc.CallOption) (*UpsertParticipantResponse, error) {
	out := new(UpsertParticipantResponse)
	if err := c.cc.Invoke(ctx, UpsertParticipantMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *client) GetParticipantByUser(ctx context.Context, in *GetParticipantByUserRequest, opts ...grpc.CallOption) (*GetParticipantByUserResponse, error) {
	out := new(GetParticipantByUserResponse)
	if err := c.cc.Invoke(ctx, GetParticipantByUserMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}
