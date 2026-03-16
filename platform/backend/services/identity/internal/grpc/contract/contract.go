package contract

import (
	"context"

	"google.golang.org/grpc"
)

const (
	ServiceName = "identity.v1.IdentityService"

	LoginMethod               = "/identity.v1.IdentityService/Login"
	RefreshTokenMethod        = "/identity.v1.IdentityService/RefreshToken"
	RevokeTokenMethod         = "/identity.v1.IdentityService/RevokeToken"
	CreateWalletMethod        = "/identity.v1.IdentityService/CreateWallet"
	ValidateTokenMethod       = "/identity.v1.IdentityService/ValidateToken"
	BindWalletMethod          = "/identity.v1.IdentityService/BindWallet"
	GetByUserMethod           = "/identity.v1.IdentityService/GetByUser"
	RegisterParticipantMethod = "/identity.v1.IdentityService/RegisterParticipant"
	SignTransactionMethod     = "/identity.v1.IdentityService/SignTransaction"
	IssueKYCCredentialMethod  = "/identity.v1.IdentityService/IssueKYCCredential"
	VerifyKYCProofMethod      = "/identity.v1.IdentityService/VerifyKYCProof"
	GetKYCStatusMethod        = "/identity.v1.IdentityService/GetKYCStatus"
	ProvisionParticipantMethod = "/identity.v1.IdentityService/ProvisionParticipant"
)

// --- Auth ---

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

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int32  `json:"expires_in"`
}

type RevokeTokenRequest struct {
	AccessToken string `json:"access_token"`
}

type RevokeTokenResponse struct {
	Success bool `json:"success"`
}

// --- Token Validation ---

type ValidateTokenRequest struct {
	AccessToken string `json:"access_token"`
}

// ValidateTokenResponse carries enriched claims from the internal JWT (D7 §7.4).
type ValidateTokenResponse struct {
	Subject      string   `json:"subject"`
	Issuer       string   `json:"issuer"`
	Roles        []string `json:"roles"`
	DID          string   `json:"did,omitempty"`
	Wallet       string   `json:"wallet,omitempty"`
	Country      string   `json:"country,omitempty"`
	BankID       string   `json:"bank_id,omitempty"`
	PrivacyGroup string   `json:"privacy_group,omitempty"`
}

// --- Wallet ---

type CreateWalletRequest struct {
	AccessToken string `json:"access_token"`
}

type CreateWalletResponse struct {
	DID       string `json:"did"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
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

// --- Participant Registration ---

type RegisterParticipantRequest struct {
	AccessToken     string `json:"access_token"`
	Country         string `json:"country"`
	BankCode        string `json:"bank_code"`
	Role            string `json:"role"`
	InstitutionName string `json:"institution_name"`
	WalletType      string `json:"wallet_type"` // Direct, Correspondent, Escrow (REQ-CAP-004)
}

type RegisterParticipantResponse struct {
	UserID         string `json:"user_id"`
	DID            string `json:"did"`
	WalletAddress  string `json:"wallet_address"`
	SignerProvider string `json:"signer_provider"`
}

// --- Signing ---

type SignTransactionRequest struct {
	UserID string `json:"user_id"`
	Digest string `json:"digest"`
}

type SignTransactionResponse struct {
	UserID         string `json:"user_id"`
	SignerProvider string `json:"signer_provider"`
	Address        string `json:"address"`
	Signature      string `json:"signature"`
}

// --- KYC / Credentials ---

type IssueKYCCredentialRequest struct {
	Subject         string `json:"subject"`
	IssuerSubject   string `json:"issuer_subject"`
	InstitutionName string `json:"institution_name"`
	CountryCode     string `json:"country_code"`
	BankCode        string `json:"bank_code"`
}

type IssueKYCCredentialResponse struct {
	VCJWT      string `json:"vc_jwt"`
	ZKPPointer string `json:"zkp_pointer"`
	IssuedAt   string `json:"issued_at"`
}

type VerifyKYCProofRequest struct {
	ZKPPointer string `json:"zkp_pointer"`
}

type VerifyKYCProofResponse struct {
	Valid bool `json:"valid"`
}

type GetKYCStatusRequest struct {
	Subject string `json:"subject"`
}

type GetKYCStatusResponse struct {
	Subject string `json:"subject"`
	Status  string `json:"status"`
}

type ProvisionParticipantRequest struct {
	Subject string `json:"subject"`
	Status  string `json:"status"` // PENDING, APPROVED, FROZEN, REVOKED
}

type ProvisionParticipantResponse struct {
	Subject string `json:"subject"`
	Status  string `json:"status"`
}

// --- Client Interface ---

// IdentityServiceClient is the typed client for all identity gRPC methods.
type IdentityServiceClient interface {
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error)
	RefreshToken(ctx context.Context, in *RefreshTokenRequest, opts ...grpc.CallOption) (*RefreshTokenResponse, error)
	RevokeToken(ctx context.Context, in *RevokeTokenRequest, opts ...grpc.CallOption) (*RevokeTokenResponse, error)
	CreateWallet(ctx context.Context, in *CreateWalletRequest, opts ...grpc.CallOption) (*CreateWalletResponse, error)
	ValidateToken(ctx context.Context, in *ValidateTokenRequest, opts ...grpc.CallOption) (*ValidateTokenResponse, error)
	BindWallet(ctx context.Context, in *BindWalletRequest, opts ...grpc.CallOption) (*BindWalletResponse, error)
	GetByUser(ctx context.Context, in *GetByUserRequest, opts ...grpc.CallOption) (*GetByUserResponse, error)
	RegisterParticipant(ctx context.Context, in *RegisterParticipantRequest, opts ...grpc.CallOption) (*RegisterParticipantResponse, error)
	SignTransaction(ctx context.Context, in *SignTransactionRequest, opts ...grpc.CallOption) (*SignTransactionResponse, error)
	IssueKYCCredential(ctx context.Context, in *IssueKYCCredentialRequest, opts ...grpc.CallOption) (*IssueKYCCredentialResponse, error)
	VerifyKYCProof(ctx context.Context, in *VerifyKYCProofRequest, opts ...grpc.CallOption) (*VerifyKYCProofResponse, error)
	GetKYCStatus(ctx context.Context, in *GetKYCStatusRequest, opts ...grpc.CallOption) (*GetKYCStatusResponse, error)
	ProvisionParticipant(ctx context.Context, in *ProvisionParticipantRequest, opts ...grpc.CallOption) (*ProvisionParticipantResponse, error)
}

type identityServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewIdentityServiceClient(cc grpc.ClientConnInterface) IdentityServiceClient {
	return &identityServiceClient{cc: cc}
}

func (c *identityServiceClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error) {
	out := new(LoginResponse)
	return out, c.cc.Invoke(ctx, LoginMethod, in, out, opts...)
}

func (c *identityServiceClient) RefreshToken(ctx context.Context, in *RefreshTokenRequest, opts ...grpc.CallOption) (*RefreshTokenResponse, error) {
	out := new(RefreshTokenResponse)
	return out, c.cc.Invoke(ctx, RefreshTokenMethod, in, out, opts...)
}

func (c *identityServiceClient) RevokeToken(ctx context.Context, in *RevokeTokenRequest, opts ...grpc.CallOption) (*RevokeTokenResponse, error) {
	out := new(RevokeTokenResponse)
	return out, c.cc.Invoke(ctx, RevokeTokenMethod, in, out, opts...)
}

func (c *identityServiceClient) CreateWallet(ctx context.Context, in *CreateWalletRequest, opts ...grpc.CallOption) (*CreateWalletResponse, error) {
	out := new(CreateWalletResponse)
	return out, c.cc.Invoke(ctx, CreateWalletMethod, in, out, opts...)
}

func (c *identityServiceClient) ValidateToken(ctx context.Context, in *ValidateTokenRequest, opts ...grpc.CallOption) (*ValidateTokenResponse, error) {
	out := new(ValidateTokenResponse)
	return out, c.cc.Invoke(ctx, ValidateTokenMethod, in, out, opts...)
}

func (c *identityServiceClient) BindWallet(ctx context.Context, in *BindWalletRequest, opts ...grpc.CallOption) (*BindWalletResponse, error) {
	out := new(BindWalletResponse)
	return out, c.cc.Invoke(ctx, BindWalletMethod, in, out, opts...)
}

func (c *identityServiceClient) GetByUser(ctx context.Context, in *GetByUserRequest, opts ...grpc.CallOption) (*GetByUserResponse, error) {
	out := new(GetByUserResponse)
	return out, c.cc.Invoke(ctx, GetByUserMethod, in, out, opts...)
}

func (c *identityServiceClient) RegisterParticipant(ctx context.Context, in *RegisterParticipantRequest, opts ...grpc.CallOption) (*RegisterParticipantResponse, error) {
	out := new(RegisterParticipantResponse)
	return out, c.cc.Invoke(ctx, RegisterParticipantMethod, in, out, opts...)
}

func (c *identityServiceClient) SignTransaction(ctx context.Context, in *SignTransactionRequest, opts ...grpc.CallOption) (*SignTransactionResponse, error) {
	out := new(SignTransactionResponse)
	return out, c.cc.Invoke(ctx, SignTransactionMethod, in, out, opts...)
}

func (c *identityServiceClient) IssueKYCCredential(ctx context.Context, in *IssueKYCCredentialRequest, opts ...grpc.CallOption) (*IssueKYCCredentialResponse, error) {
	out := new(IssueKYCCredentialResponse)
	return out, c.cc.Invoke(ctx, IssueKYCCredentialMethod, in, out, opts...)
}

func (c *identityServiceClient) VerifyKYCProof(ctx context.Context, in *VerifyKYCProofRequest, opts ...grpc.CallOption) (*VerifyKYCProofResponse, error) {
	out := new(VerifyKYCProofResponse)
	return out, c.cc.Invoke(ctx, VerifyKYCProofMethod, in, out, opts...)
}

func (c *identityServiceClient) GetKYCStatus(ctx context.Context, in *GetKYCStatusRequest, opts ...grpc.CallOption) (*GetKYCStatusResponse, error) {
	out := new(GetKYCStatusResponse)
	return out, c.cc.Invoke(ctx, GetKYCStatusMethod, in, out, opts...)
}

func (c *identityServiceClient) ProvisionParticipant(ctx context.Context, in *ProvisionParticipantRequest, opts ...grpc.CallOption) (*ProvisionParticipantResponse, error) {
	out := new(ProvisionParticipantResponse)
	return out, c.cc.Invoke(ctx, ProvisionParticipantMethod, in, out, opts...)
}
