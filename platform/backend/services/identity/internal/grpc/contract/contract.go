package contract

import (
	"context"

	"google.golang.org/grpc"
)

const (
	// ServiceName is the canonical gRPC service name.
	ServiceName = "identity.v1.IdentityService"
	// LoginMethod is the canonical full path for login.
	LoginMethod = "/identity.v1.IdentityService/Login"
	// CreateWalletMethod is the canonical full path for wallet creation.
	CreateWalletMethod = "/identity.v1.IdentityService/CreateWallet"
	// ValidateTokenMethod is the canonical full path for token validation.
	ValidateTokenMethod = "/identity.v1.IdentityService/ValidateToken"
	// BindWalletMethod is the canonical full path for binding wallets.
	BindWalletMethod = "/identity.v1.IdentityService/BindWallet"
	// GetByUserMethod is the canonical full path for reading wallet binding by user.
	GetByUserMethod = "/identity.v1.IdentityService/GetByUser"
	// RegisterParticipantMethod is the canonical full path for onboarding registration.
	RegisterParticipantMethod = "/identity.v1.IdentityService/RegisterParticipant"
	// SignTransactionMethod is the canonical full path for signing requests.
	SignTransactionMethod = "/identity.v1.IdentityService/SignTransaction"
)

type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int32  `json:"expires_in"`
}

type CreateWalletRequest struct {
	AccessToken string `json:"access_token"`
}

type CreateWalletResponse struct {
	DID       string `json:"did"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
}

type ValidateTokenRequest struct {
	AccessToken string `json:"access_token"`
}

type ValidateTokenResponse struct {
	Subject string   `json:"subject"`
	Issuer  string   `json:"issuer"`
	Roles   []string `json:"roles"`
}

type BindWalletRequest struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

type BindWalletResponse struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

type GetByUserRequest struct {
	UserID string `json:"user_id"`
}

type GetByUserResponse struct {
	Binding *BindWalletResponse `json:"binding,omitempty"`
	Found   bool                `json:"found"`
}

type RegisterParticipantRequest struct {
	AccessToken string `json:"access_token"`
	Country     string `json:"country"`
	BankCode    string `json:"bank_code"`
	Role        string `json:"role"`
}

type RegisterParticipantResponse struct {
	UserID         string `json:"user_id"`
	DID            string `json:"did"`
	WalletAddress  string `json:"wallet_address"`
	SignerProvider string `json:"signer_provider"`
	KMSKeyID       string `json:"kms_key_id"`
}

type SignTransactionRequest struct {
	UserID string `json:"user_id"`
	Digest string `json:"digest"`
}

type SignTransactionResponse struct {
	UserID         string `json:"user_id"`
	SignerProvider string `json:"signer_provider"`
	KMSKeyID       string `json:"kms_key_id"`
	Address        string `json:"address"`
	Signature      string `json:"signature"`
}

// IdentityServiceClient is the typed client for identity gRPC methods.
type IdentityServiceClient interface {
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error)
	CreateWallet(ctx context.Context, in *CreateWalletRequest, opts ...grpc.CallOption) (*CreateWalletResponse, error)
	ValidateToken(ctx context.Context, in *ValidateTokenRequest, opts ...grpc.CallOption) (*ValidateTokenResponse, error)
	BindWallet(ctx context.Context, in *BindWalletRequest, opts ...grpc.CallOption) (*BindWalletResponse, error)
	GetByUser(ctx context.Context, in *GetByUserRequest, opts ...grpc.CallOption) (*GetByUserResponse, error)
	RegisterParticipant(ctx context.Context, in *RegisterParticipantRequest, opts ...grpc.CallOption) (*RegisterParticipantResponse, error)
	SignTransaction(ctx context.Context, in *SignTransactionRequest, opts ...grpc.CallOption) (*SignTransactionResponse, error)
}

type identityServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewIdentityServiceClient(cc grpc.ClientConnInterface) IdentityServiceClient {
	return &identityServiceClient{cc: cc}
}

func (c *identityServiceClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error) {
	out := new(LoginResponse)
	if err := c.cc.Invoke(ctx, LoginMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *identityServiceClient) CreateWallet(ctx context.Context, in *CreateWalletRequest, opts ...grpc.CallOption) (*CreateWalletResponse, error) {
	out := new(CreateWalletResponse)
	if err := c.cc.Invoke(ctx, CreateWalletMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *identityServiceClient) ValidateToken(ctx context.Context, in *ValidateTokenRequest, opts ...grpc.CallOption) (*ValidateTokenResponse, error) {
	out := new(ValidateTokenResponse)
	if err := c.cc.Invoke(ctx, ValidateTokenMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *identityServiceClient) BindWallet(ctx context.Context, in *BindWalletRequest, opts ...grpc.CallOption) (*BindWalletResponse, error) {
	out := new(BindWalletResponse)
	if err := c.cc.Invoke(ctx, BindWalletMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *identityServiceClient) GetByUser(ctx context.Context, in *GetByUserRequest, opts ...grpc.CallOption) (*GetByUserResponse, error) {
	out := new(GetByUserResponse)
	if err := c.cc.Invoke(ctx, GetByUserMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *identityServiceClient) RegisterParticipant(ctx context.Context, in *RegisterParticipantRequest, opts ...grpc.CallOption) (*RegisterParticipantResponse, error) {
	out := new(RegisterParticipantResponse)
	if err := c.cc.Invoke(ctx, RegisterParticipantMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *identityServiceClient) SignTransaction(ctx context.Context, in *SignTransactionRequest, opts ...grpc.CallOption) (*SignTransactionResponse, error) {
	out := new(SignTransactionResponse)
	if err := c.cc.Invoke(ctx, SignTransactionMethod, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}
