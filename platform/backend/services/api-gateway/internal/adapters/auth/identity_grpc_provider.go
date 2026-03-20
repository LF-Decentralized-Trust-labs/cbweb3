package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

const (
	identityLoginMethod         = "/auth.v1.AuthService/Login"
	identityRefreshTokenMethod  = "/auth.v1.AuthService/RefreshToken"
	identityRevokeTokenMethod   = "/auth.v1.AuthService/RevokeToken"
	identityValidateTokenMethod = "/auth.v1.AuthService/ValidateToken" // #nosec G101 -- gRPC method path
	identityIssueNonceMethod    = "/auth.v1.AuthService/IssueLoginNonce"
	identityVerifyPKIMethod     = "/auth.v1.AuthService/VerifyPKILogin"
)

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

type identityLoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type identityLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type identityRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type identityRevokeTokenRequest struct {
	AccessToken string `json:"access_token"`
}

type identityValidateTokenRequest struct {
	AccessToken string `json:"access_token"`
}

// identityValidateTokenResponse includes enriched D7 §7.4 claims.
type identityValidateTokenResponse struct {
	Subject      string   `json:"subject"`
	Issuer       string   `json:"issuer"`
	Roles        []string `json:"roles"`
	Wallet       string   `json:"wallet"`
	Country      string   `json:"country"`
	BankID       string   `json:"bank_id"`
	PrivacyGroup string   `json:"privacy_group"`
}

// IdentityGRPCAuthProvider authenticates users via identity gRPC service.
type IdentityGRPCAuthProvider struct {
	conn   *grpc.ClientConn
	codec  encoding.Codec
	client grpc.ClientConnInterface
}

// NewIdentityGRPCAuthProvider builds a gRPC-backed auth provider + token validator.
func NewIdentityGRPCAuthProvider(address string, timeout time.Duration) (*IdentityGRPCAuthProvider, error) {
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
	return &IdentityGRPCAuthProvider{
		conn:   conn,
		codec:  codec,
		client: conn,
	}, nil
}

// Authenticate delegates login to identity gRPC.
func (p *IdentityGRPCAuthProvider) Authenticate(ctx context.Context, clientID, clientSecret string) (domain.AuthToken, error) {
	req := &identityLoginRequest{User: clientID, Password: clientSecret}
	out := &identityLoginResponse{}
	if err := p.client.Invoke(ctx, identityLoginMethod, req, out, grpc.ForceCodec(p.codec)); err != nil {
		return domain.AuthToken{}, domain.ErrInvalidCredentials
	}
	return domain.AuthToken{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresIn:    out.ExpiresIn,
	}, nil
}

// RefreshToken issues a new access token via identity gRPC.
func (p *IdentityGRPCAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (domain.AuthToken, error) {
	req := &identityRefreshTokenRequest{RefreshToken: refreshToken}
	out := &identityLoginResponse{}
	if err := p.client.Invoke(ctx, identityRefreshTokenMethod, req, out, grpc.ForceCodec(p.codec)); err != nil {
		return domain.AuthToken{}, domain.ErrInvalidToken
	}
	return domain.AuthToken{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresIn:    out.ExpiresIn,
	}, nil
}

// Logout revokes the access token via identity gRPC.
func (p *IdentityGRPCAuthProvider) Logout(ctx context.Context, accessToken string) error {
	req := &identityRevokeTokenRequest{AccessToken: accessToken}
	var out struct {
		Success bool `json:"success"`
	}
	if err := p.client.Invoke(ctx, identityRevokeTokenMethod, req, &out, grpc.ForceCodec(p.codec)); err != nil {
		return domain.ErrInvalidToken
	}
	return nil
}

// Validate delegates token validation to identity gRPC, returning enriched claims.
func (p *IdentityGRPCAuthProvider) Validate(ctx context.Context, token string) (domain.TokenClaims, error) {
	req := &identityValidateTokenRequest{AccessToken: token}
	out := &identityValidateTokenResponse{}
	if err := p.client.Invoke(ctx, identityValidateTokenMethod, req, out, grpc.ForceCodec(p.codec)); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != 0 {
			return domain.TokenClaims{}, domain.ErrInvalidToken
		}
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	return domain.TokenClaims{
		Subject:      out.Subject,
		Issuer:       out.Issuer,
		Roles:        out.Roles,
		Wallet:       out.Wallet,
		Country:      out.Country,
		BankID:       out.BankID,
		PrivacyGroup: out.PrivacyGroup,
	}, nil
}

// IssueLoginNonce requests a PKI login nonce for the given user (step 1).
func (p *IdentityGRPCAuthProvider) IssueLoginNonce(ctx context.Context, userID string) (string, error) {
	req := struct {
		UserID string `json:"user_id"`
	}{UserID: userID}
	var out struct {
		Nonce string `json:"nonce"`
	}
	if err := p.client.Invoke(ctx, identityIssueNonceMethod, &req, &out, grpc.ForceCodec(p.codec)); err != nil {
		return "", err
	}
	return out.Nonce, nil
}

// VerifyPKILogin completes PKI login step 2: validates the signed nonce + X.509 cert.
func (p *IdentityGRPCAuthProvider) VerifyPKILogin(ctx context.Context, userID, nonceSignatureHex, certPEM string) (domain.AuthToken, error) {
	req := struct {
		UserID            string `json:"user_id"`
		NonceSignatureHex string `json:"nonce_signature_hex"`
		CertPEM           string `json:"cert_pem"`
	}{UserID: userID, NonceSignatureHex: nonceSignatureHex, CertPEM: certPEM}

	out := &identityLoginResponse{}
	if err := p.client.Invoke(ctx, identityVerifyPKIMethod, &req, out, grpc.ForceCodec(p.codec)); err != nil {
		return domain.AuthToken{}, err
	}
	return domain.AuthToken{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresIn:    out.ExpiresIn,
	}, nil
}
