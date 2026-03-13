package identityprovider

import "context"

// LoginRequest is the normalized login payload for identity providers.
type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// TokenResponse is the normalized auth token payload returned by providers.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// WalletResponse is the normalized wallet creation response.
type WalletResponse struct {
	DID       string `json:"did"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
}

// WalletBinding represents a user-wallet association.
type WalletBinding struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

// TokenClaims is the normalized token claims payload.
type TokenClaims struct {
	Subject string   `json:"subject"`
	Issuer  string   `json:"issuer"`
	Roles   []string `json:"roles"`
}

// Provider defines external identity capabilities consumed by services.
type Provider interface {
	Name() string
	Login(ctx context.Context, req LoginRequest) (TokenResponse, error)
	CreateWallet(ctx context.Context, accessToken string) (WalletResponse, error)
	ValidateToken(ctx context.Context, accessToken string) (TokenClaims, error)
	BindWallet(ctx context.Context, userID, walletAddress string) (WalletBinding, error)
	GetByUser(ctx context.Context, userID string) (WalletBinding, bool, error)
}
