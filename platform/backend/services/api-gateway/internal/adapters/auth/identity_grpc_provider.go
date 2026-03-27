package auth

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// IdentityGRPCAuthProvider authenticates users via identity gRPC service.
type IdentityGRPCAuthProvider struct {
	conn *grpc.ClientConn
	cc   authv1.AuthServiceClient
}

// NewIdentityGRPCAuthProvider dials a new gRPC connection and returns a provider.
func NewIdentityGRPCAuthProvider(address string, timeout time.Duration) (*IdentityGRPCAuthProvider, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext( //nolint:staticcheck
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return NewIdentityGRPCAuthProviderFromConn(conn), nil
}

// NewIdentityGRPCAuthProviderFromConn wraps an existing gRPC connection.
// The caller retains ownership of conn lifecycle when using this constructor.
func NewIdentityGRPCAuthProviderFromConn(conn *grpc.ClientConn) *IdentityGRPCAuthProvider {
	return &IdentityGRPCAuthProvider{conn: conn, cc: authv1.NewAuthServiceClient(conn)}
}

// Close releases the underlying gRPC connection.
func (p *IdentityGRPCAuthProvider) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// Authenticate delegates login to identity gRPC.
func (p *IdentityGRPCAuthProvider) Authenticate(ctx context.Context, clientID, clientSecret string) (domain.AuthToken, error) {
	out, err := p.cc.Login(ctx, &authv1.LoginRequest{User: clientID, Password: clientSecret})
	if err != nil {
		return domain.AuthToken{}, domain.ErrInvalidCredentials
	}
	return domain.AuthToken{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresIn:    int(out.ExpiresIn),
	}, nil
}

// RefreshToken issues a new access token via identity gRPC.
func (p *IdentityGRPCAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (domain.AuthToken, error) {
	out, err := p.cc.RefreshToken(ctx, &authv1.RefreshTokenRequest{RefreshToken: refreshToken})
	if err != nil {
		return domain.AuthToken{}, domain.ErrInvalidToken
	}
	return domain.AuthToken{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresIn:    int(out.ExpiresIn),
	}, nil
}

// Logout revokes the access token via identity gRPC.
func (p *IdentityGRPCAuthProvider) Logout(ctx context.Context, accessToken string) error {
	out, err := p.cc.RevokeToken(ctx, &authv1.RevokeTokenRequest{AccessToken: accessToken})
	if err != nil {
		return domain.ErrInvalidToken
	}
	if !out.Success {
		return domain.ErrInvalidToken
	}
	return nil
}

// Validate delegates token validation to identity gRPC, returning enriched claims.
func (p *IdentityGRPCAuthProvider) Validate(ctx context.Context, token string) (domain.TokenClaims, error) {
	out, err := p.cc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: token})
	if err != nil {
		return domain.TokenClaims{}, domain.ErrInvalidToken
	}
	return domain.TokenClaims{
		Subject:      out.Subject,
		Issuer:       out.Issuer,
		Roles:        out.Roles,
		Wallet:       out.Wallet,
		Country:      out.Country,
		BankID:       out.BankId,
		PrivacyGroup: out.PrivacyGroup,
	}, nil
}

// IssueLoginNonce validates clientSecret (first factor) and requests a PKI login nonce (step 1).
func (p *IdentityGRPCAuthProvider) IssueLoginNonce(ctx context.Context, userID, clientSecret string) (string, error) {
	out, err := p.cc.IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: userID, ClientSecret: clientSecret})
	if err != nil {
		return "", err
	}
	return out.Nonce, nil
}

// ChangeClientSecret rotates the clientSecret for the given user.
func (p *IdentityGRPCAuthProvider) ChangeClientSecret(ctx context.Context, userID, currentClientSecret, newClientSecret string) error {
	_, err := p.cc.ChangeClientSecret(ctx, &authv1.ChangeClientSecretRequest{
		UserId:        userID,
		CurrentSecret: currentClientSecret,
		NewSecret:     newClientSecret,
	})
	return err
}

// VerifyPKILogin completes PKI login step 2: validates the signed nonce + X.509 cert.
func (p *IdentityGRPCAuthProvider) VerifyPKILogin(ctx context.Context, userID, nonceSignatureHex, certPEM string) (domain.AuthToken, error) {
	out, err := p.cc.VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
		UserId:            userID,
		NonceSignatureHex: nonceSignatureHex,
		CertPem:           certPEM,
	})
	if err != nil {
		return domain.AuthToken{}, err
	}
	return domain.AuthToken{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		ExpiresIn:    int(out.ExpiresIn),
	}, nil
}
