package providers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider"
	"github.com/golang-jwt/jwt/v5"
)

type localUser struct {
	Username string
	Password string
	Role     string
}

// LocalConfig contains in-memory provider options for local development.
type LocalConfig struct {
	JWTSecret     string
	AccessTokenTT time.Duration
}

// LocalProvider is an in-memory implementation for local development.
type LocalProvider struct {
	cfg          LocalConfig
	mu           sync.RWMutex
	users        map[string]localUser
	walletByUser map[string]identityprovider.WalletResponse
	userByWallet map[string]string
}

// NewLocalProvider creates a local provider without network calls.
func NewLocalProvider(cfg LocalConfig) *LocalProvider {
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		cfg.JWTSecret = "local-identity-secret"
	}
	if cfg.AccessTokenTT <= 0 {
		cfg.AccessTokenTT = time.Hour
	}
	return &LocalProvider{
		cfg: cfg,
		users: map[string]localUser{
			"issueruser":   {Username: "issueruser", Password: "1234", Role: "issuer"},
			"holderuser":   {Username: "holderuser", Password: "1234", Role: "holder"},
			"verifieruser": {Username: "verifieruser", Password: "1234", Role: "verifier"},
			"bank-a":       {Username: "bank-a", Password: "secret-a", Role: "issuer"},
			"bank-b":       {Username: "bank-b", Password: "secret-b", Role: "issuer"},
			"bank-z":       {Username: "bank-z", Password: "secret-z", Role: "issuer"},
		},
		walletByUser: map[string]identityprovider.WalletResponse{},
		userByWallet: map[string]string{},
	}
}

func (p *LocalProvider) Name() string { return ProviderLocal }

// Login validates local credentials and issues an HS256 token.
func (p *LocalProvider) Login(_ context.Context, req identityprovider.LoginRequest) (identityprovider.TokenResponse, error) {
	p.mu.RLock()
	u, ok := p.users[req.User]
	p.mu.RUnlock()
	if !ok || u.Password != req.Password {
		return identityprovider.TokenResponse{}, errors.New("invalid credentials")
	}

	accessToken, err := p.issueAccessToken(u)
	if err != nil {
		return identityprovider.TokenResponse{}, err
	}
	refreshToken, err := randomHex(32)
	if err != nil {
		return identityprovider.TokenResponse{}, err
	}
	return identityprovider.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(p.cfg.AccessTokenTT.Seconds()),
	}, nil
}

// CreateWallet creates or returns the existing wallet for a token subject.
func (p *LocalProvider) CreateWallet(ctx context.Context, accessToken string) (identityprovider.WalletResponse, error) {
	claims, err := p.ValidateToken(ctx, accessToken)
	if err != nil {
		return identityprovider.WalletResponse{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.walletByUser[claims.Subject]; ok {
		return existing, nil
	}

	address, err := randomAddress()
	if err != nil {
		return identityprovider.WalletResponse{}, err
	}
	w := identityprovider.WalletResponse{
		DID:       "did:lac:openprotest:" + address,
		Address:   address,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	p.walletByUser[claims.Subject] = w
	p.userByWallet[strings.ToLower(w.Address)] = claims.Subject
	return w, nil
}

// ValidateToken validates local HS256 tokens and extracts normalized claims.
func (p *LocalProvider) ValidateToken(_ context.Context, accessToken string) (identityprovider.TokenClaims, error) {
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		return []byte(p.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return identityprovider.TokenClaims{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return identityprovider.TokenClaims{}, errors.New("invalid token claims")
	}
	subject, _ := claims["sub"].(string)
	if strings.TrimSpace(subject) == "" {
		subject, _ = claims["preferred_username"].(string)
	}
	issuer, _ := claims["iss"].(string)
	roles := extractRoles(claims["roles"])
	if strings.TrimSpace(subject) == "" {
		return identityprovider.TokenClaims{}, errors.New("missing subject claim")
	}
	return identityprovider.TokenClaims{
		Subject: subject,
		Issuer:  issuer,
		Roles:   roles,
	}, nil
}

func (p *LocalProvider) issueAccessToken(u localUser) (string, error) {
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":                u.Username,
		"preferred_username": u.Username,
		"roles":              []string{u.Role},
		"iss":                "identity-local-provider",
		"iat":                now.Unix(),
		"exp":                now.Add(p.cfg.AccessTokenTT).Unix(),
	})
	return token.SignedString([]byte(p.cfg.JWTSecret))
}

func extractRoles(raw any) []string {
	if direct, ok := raw.([]string); ok {
		out := make([]string, 0, len(direct))
		for _, role := range direct {
			if strings.TrimSpace(role) != "" {
				out = append(out, role)
			}
		}
		return out
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if role, ok := item.(string); ok && strings.TrimSpace(role) != "" {
			out = append(out, role)
		}
	}
	return out
}

func randomAddress() (string, error) {
	raw, err := randomHex(20)
	if err != nil {
		return "", err
	}
	return "0x" + raw, nil
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (p *LocalProvider) BindWallet(_ context.Context, userID, walletAddress string) (identityprovider.WalletBinding, error) {
	userID = strings.TrimSpace(userID)
	walletAddress = strings.ToLower(strings.TrimSpace(walletAddress))
	if userID == "" || walletAddress == "" {
		return identityprovider.WalletBinding{}, errors.New("userID and walletAddress are required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existingUser, ok := p.userByWallet[walletAddress]; ok && existingUser != userID {
		return identityprovider.WalletBinding{}, errors.New("wallet already bound")
	}
	if existingWallet, ok := p.walletByUser[userID]; ok && !strings.EqualFold(existingWallet.Address, walletAddress) {
		return identityprovider.WalletBinding{}, errors.New("user already bound")
	}

	wallet := identityprovider.WalletResponse{
		DID:       "did:lac:openprotest:" + walletAddress,
		Address:   walletAddress,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	p.walletByUser[userID] = wallet
	p.userByWallet[walletAddress] = userID
	return identityprovider.WalletBinding{
		UserID:        userID,
		WalletAddress: walletAddress,
	}, nil
}

func (p *LocalProvider) GetByUser(_ context.Context, userID string) (identityprovider.WalletBinding, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return identityprovider.WalletBinding{}, false, nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	wallet, ok := p.walletByUser[userID]
	if !ok {
		return identityprovider.WalletBinding{}, false, nil
	}
	return identityprovider.WalletBinding{
		UserID:        userID,
		WalletAddress: wallet.Address,
	}, true, nil
}
