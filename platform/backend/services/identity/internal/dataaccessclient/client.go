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
)

type Participant struct {
	UserID         string
	DID            string
	WalletAddress  string
	Country        string
	BankCode       string
	Role           string
	SignerProvider string
	KMSKeyID       string
}

type Client interface {
	UpsertParticipant(ctx context.Context, p Participant) error
	GetParticipantByUser(ctx context.Context, userID string) (Participant, bool, error)
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
			UserID         string `json:"user_id"`
			DID            string `json:"did"`
			WalletAddress  string `json:"wallet_address"`
			Country        string `json:"country"`
			BankCode       string `json:"bank_code"`
			Role           string `json:"role"`
			SignerProvider string `json:"signer_provider"`
			KMSKeyID       string `json:"kms_key_id"`
		} `json:"participant"`
	}{}
	req.Participant = struct {
		UserID         string `json:"user_id"`
		DID            string `json:"did"`
		WalletAddress  string `json:"wallet_address"`
		Country        string `json:"country"`
		BankCode       string `json:"bank_code"`
		Role           string `json:"role"`
		SignerProvider string `json:"signer_provider"`
		KMSKeyID       string `json:"kms_key_id"`
	}{
		UserID:         p.UserID,
		DID:            p.DID,
		WalletAddress:  p.WalletAddress,
		Country:        p.Country,
		BankCode:       p.BankCode,
		Role:           p.Role,
		SignerProvider: p.SignerProvider,
		KMSKeyID:       p.KMSKeyID,
	}
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
			UserID         string `json:"user_id"`
			DID            string `json:"did"`
			WalletAddress  string `json:"wallet_address"`
			Country        string `json:"country"`
			BankCode       string `json:"bank_code"`
			Role           string `json:"role"`
			SignerProvider string `json:"signer_provider"`
			KMSKeyID       string `json:"kms_key_id"`
		} `json:"participant"`
	}
	if err := c.cc.Invoke(ctx, getParticipantByUserMethod, &req, &resp, grpc.ForceCodec(c.codec)); err != nil {
		return Participant{}, false, err
	}
	if !resp.Found {
		return Participant{}, false, nil
	}
	return Participant{
		UserID:         resp.Participant.UserID,
		DID:            resp.Participant.DID,
		WalletAddress:  resp.Participant.WalletAddress,
		Country:        resp.Participant.Country,
		BankCode:       resp.Participant.BankCode,
		Role:           resp.Participant.Role,
		SignerProvider: resp.Participant.SignerProvider,
		KMSKeyID:       resp.Participant.KMSKeyID,
	}, true, nil
}
