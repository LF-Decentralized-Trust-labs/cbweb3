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
	identityLoginMethod         = "/identity.v1.IdentityService/Login"
	identityValidateTokenMethod = "/identity.v1.IdentityService/ValidateToken" // #nosec G101 -- This is a gRPC method path, not a credential
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
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type identityValidateTokenRequest struct {
	AccessToken string `json:"access_token"`
}

type identityValidateTokenResponse struct {
	Subject string   `json:"subject"`
	Issuer  string   `json:"issuer"`
	Roles   []string `json:"roles"`
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
	req := &identityLoginRequest{
		User:     clientID,
		Password: clientSecret,
	}
	out := &identityLoginResponse{}
	if err := p.client.Invoke(ctx, identityLoginMethod, req, out, grpc.ForceCodec(p.codec)); err != nil {
		return domain.AuthToken{}, domain.ErrInvalidCredentials
	}
	return domain.AuthToken{
		AccessToken: out.AccessToken,
		TokenType:   out.TokenType,
		ExpiresIn:   out.ExpiresIn,
	}, nil
}

// Validate delegates token validation to identity gRPC.
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
		Subject: out.Subject,
		Issuer:  out.Issuer,
		Roles:   out.Roles,
	}, nil
}
