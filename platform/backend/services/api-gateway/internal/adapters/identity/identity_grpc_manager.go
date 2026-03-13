package identity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

const (
	bindWalletMethod = "/identity.v1.IdentityService/BindWallet"
	getByUserMethod  = "/identity.v1.IdentityService/GetByUser"
)

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

type bindWalletRequest struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

type walletBindingResponse struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

type getByUserRequest struct {
	UserID string `json:"user_id"`
}

type getByUserResponse struct {
	Binding *walletBindingResponse `json:"binding,omitempty"`
	Found   bool                   `json:"found"`
}

// IdentityGRPCManager delegates identity bindings to the identity microservice.
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

func (m *IdentityGRPCManager) BindWallet(userID, walletAddress string) (domain.WalletBinding, error) {
	req := &bindWalletRequest{
		UserID:        userID,
		WalletAddress: walletAddress,
	}
	out := &walletBindingResponse{}
	err := m.client.Invoke(context.Background(), bindWalletMethod, req, out, grpc.ForceCodec(m.codec))
	if err != nil {
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

func (m *IdentityGRPCManager) GetByUser(userID string) (domain.WalletBinding, bool) {
	req := &getByUserRequest{UserID: userID}
	out := &getByUserResponse{}
	err := m.client.Invoke(context.Background(), getByUserMethod, req, out, grpc.ForceCodec(m.codec))
	if err != nil || !out.Found || out.Binding == nil {
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
