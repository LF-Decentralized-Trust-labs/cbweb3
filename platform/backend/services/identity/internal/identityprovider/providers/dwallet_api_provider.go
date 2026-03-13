package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider"
	"github.com/golang-jwt/jwt/v5"
)

// DWalletAPIProvider implements Provider using D-Wallet API endpoints.
// It keeps the same level-1 contract used by other providers.
type DWalletAPIProvider struct {
	baseURL    string
	jwtSecret  string
	httpClient *http.Client
	mu         sync.RWMutex
	byUser     map[string]string
	byWallet   map[string]string
}

// NewDWalletAPIProvider creates a D-Wallet specific provider implementation.
func NewDWalletAPIProvider(baseURL, jwtSecret string, timeout time.Duration) *DWalletAPIProvider {
	return &DWalletAPIProvider{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		jwtSecret: strings.TrimSpace(jwtSecret),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		byUser:   map[string]string{},
		byWallet: map[string]string{},
	}
}

func (p *DWalletAPIProvider) Name() string { return ProviderDWalletAPI }

func (p *DWalletAPIProvider) Login(ctx context.Context, req identityprovider.LoginRequest) (identityprovider.TokenResponse, error) {
	var out identityprovider.TokenResponse
	url := p.baseURL + "/wallet/login"
	if err := p.postJSON(ctx, url, req, "", &out); err != nil {
		return identityprovider.TokenResponse{}, err
	}
	return out, nil
}

func (p *DWalletAPIProvider) CreateWallet(ctx context.Context, accessToken string) (identityprovider.WalletResponse, error) {
	var out identityprovider.WalletResponse
	url := p.baseURL + "/wallet"
	if err := p.postJSON(ctx, url, map[string]any{}, accessToken, &out); err != nil {
		return identityprovider.WalletResponse{}, err
	}
	return out, nil
}

// ValidateToken validates token signature when a secret is configured.
// If there is no secret, token is parsed without signature validation.
func (p *DWalletAPIProvider) ValidateToken(_ context.Context, accessToken string) (identityprovider.TokenClaims, error) {
	claims := jwt.MapClaims{}

	if p.jwtSecret != "" {
		token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (any, error) {
			return []byte(p.jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return identityprovider.TokenClaims{}, fmt.Errorf("validate token: %w", err)
		}
	} else {
		parser := jwt.NewParser()
		if _, _, err := parser.ParseUnverified(accessToken, claims); err != nil {
			return identityprovider.TokenClaims{}, fmt.Errorf("parse token: %w", err)
		}
	}

	subject, _ := claims["sub"].(string)
	if strings.TrimSpace(subject) == "" {
		subject, _ = claims["preferred_username"].(string)
	}
	if strings.TrimSpace(subject) == "" {
		return identityprovider.TokenClaims{}, fmt.Errorf("missing subject claim")
	}
	issuer, _ := claims["iss"].(string)
	return identityprovider.TokenClaims{
		Subject: subject,
		Issuer:  issuer,
		Roles:   extractRoles(claims["roles"]),
	}, nil
}

func (p *DWalletAPIProvider) postJSON(
	ctx context.Context,
	url string,
	payload any,
	accessToken string,
	out any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider status %d: %s", resp.StatusCode, string(respBody))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

func (p *DWalletAPIProvider) BindWallet(_ context.Context, userID, walletAddress string) (identityprovider.WalletBinding, error) {
	userID = strings.TrimSpace(userID)
	walletAddress = strings.ToLower(strings.TrimSpace(walletAddress))
	if userID == "" || walletAddress == "" {
		return identityprovider.WalletBinding{}, errors.New("userID and walletAddress are required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if existingUser, ok := p.byWallet[walletAddress]; ok && existingUser != userID {
		return identityprovider.WalletBinding{}, errors.New("wallet already bound")
	}
	if existingWallet, ok := p.byUser[userID]; ok && !strings.EqualFold(existingWallet, walletAddress) {
		return identityprovider.WalletBinding{}, errors.New("user already bound")
	}

	p.byUser[userID] = walletAddress
	p.byWallet[walletAddress] = userID
	return identityprovider.WalletBinding{
		UserID:        userID,
		WalletAddress: walletAddress,
	}, nil
}

func (p *DWalletAPIProvider) GetByUser(_ context.Context, userID string) (identityprovider.WalletBinding, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return identityprovider.WalletBinding{}, false, nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	walletAddress, ok := p.byUser[userID]
	if !ok {
		return identityprovider.WalletBinding{}, false, nil
	}
	return identityprovider.WalletBinding{
		UserID:        userID,
		WalletAddress: walletAddress,
	}, true, nil
}
