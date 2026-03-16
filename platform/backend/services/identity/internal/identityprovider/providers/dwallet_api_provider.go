package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider"
	"github.com/golang-jwt/jwt/v5"
)

// DWalletAPIProvider implements the full identityprovider.Provider interface
// by delegating to the LNET stack:
//   - Auth + Wallet operations → D-Wallet API
//   - KYC Credentials         → SSI-VC-API
//   - ZK Proof verification   → Claims Verifier smart contract (via RPC)
//
// BindWallet and GetByUser are NOT persisted locally — they rely on the
// data-access service for persistence (handled at the gRPC server layer).
type DWalletAPIProvider struct {
	baseURL    string
	jwtSecret  string
	httpClient *http.Client
}

// NewDWalletAPIProvider creates a provider that delegates to the LNET D-Wallet API.
func NewDWalletAPIProvider(baseURL, jwtSecret string, timeout time.Duration) *DWalletAPIProvider {
	return &DWalletAPIProvider{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		jwtSecret: strings.TrimSpace(jwtSecret),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *DWalletAPIProvider) Name() string { return ProviderDWalletAPI }

// Login authenticates via D-Wallet API POST /wallet/login.
func (p *DWalletAPIProvider) Login(ctx context.Context, req identityprovider.LoginRequest) (identityprovider.TokenResponse, error) {
	var out identityprovider.TokenResponse
	if err := p.postJSON(ctx, p.baseURL+"/wallet/login", req, "", &out); err != nil {
		return identityprovider.TokenResponse{}, err
	}
	return out, nil
}

// RefreshToken issues a new access token via D-Wallet API POST /wallet/refresh.
func (p *DWalletAPIProvider) RefreshToken(ctx context.Context, refreshToken string) (identityprovider.TokenResponse, error) {
	var out identityprovider.TokenResponse
	body := map[string]string{"refresh_token": refreshToken}
	if err := p.postJSON(ctx, p.baseURL+"/wallet/refresh", body, "", &out); err != nil {
		return identityprovider.TokenResponse{}, err
	}
	return out, nil
}

// RevokeToken invalidates the session via D-Wallet API POST /wallet/logout.
func (p *DWalletAPIProvider) RevokeToken(ctx context.Context, accessToken string) error {
	return p.postJSON(ctx, p.baseURL+"/wallet/logout", map[string]any{}, accessToken, &map[string]any{})
}

// ValidateToken validates token signature when a secret is configured.
// If there is no secret, the token is parsed without signature validation.
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
	did, _ := claims["did"].(string)
	wallet, _ := claims["wallet"].(string)
	country, _ := claims["country"].(string)
	bankID, _ := claims["bank_id"].(string)
	privacyGroup, _ := claims["privacy_group"].(string)

	return identityprovider.TokenClaims{
		Subject:      subject,
		Issuer:       issuer,
		Roles:        extractRoles(claims["roles"]),
		DID:          did,
		Wallet:       wallet,
		Country:      country,
		BankID:       bankID,
		PrivacyGroup: privacyGroup,
	}, nil
}

// CreateWallet provisions a DID + EVM wallet via D-Wallet API POST /wallet.
func (p *DWalletAPIProvider) CreateWallet(ctx context.Context, accessToken string) (identityprovider.WalletResponse, error) {
	var out identityprovider.WalletResponse
	if err := p.postJSON(ctx, p.baseURL+"/wallet", map[string]any{}, accessToken, &out); err != nil {
		return identityprovider.WalletResponse{}, err
	}
	return out, nil
}

// BindWallet is a logical operation — the actual binding is persisted via
// data-access at the gRPC server layer, not stored locally here.
func (p *DWalletAPIProvider) BindWallet(_ context.Context, userID, walletAddress string) (identityprovider.WalletBinding, error) {
	userID = strings.TrimSpace(userID)
	walletAddress = strings.TrimSpace(walletAddress)
	if userID == "" || walletAddress == "" {
		return identityprovider.WalletBinding{}, fmt.Errorf("userID and walletAddress are required")
	}
	return identityprovider.WalletBinding{
		UserID:        userID,
		WalletAddress: walletAddress,
	}, nil
}

// GetByUser returns a WalletBinding. The actual lookup is handled by the gRPC
// server via data-access; this implementation delegates to the server layer.
func (p *DWalletAPIProvider) GetByUser(_ context.Context, _ string) (identityprovider.WalletBinding, bool, error) {
	// Persistence is managed by data-access at the server layer.
	return identityprovider.WalletBinding{}, false, nil
}

// SignTransaction delegates signing to D-Wallet API POST /wallet/sign.
// The private key never leaves the LNET custody infrastructure.
func (p *DWalletAPIProvider) SignTransaction(ctx context.Context, req identityprovider.SignRequest) (identityprovider.SignResponse, error) {
	var out identityprovider.SignResponse
	body := map[string]string{
		"user_id":    req.UserID,
		"digest_hex": req.DigestHex,
	}
	if err := p.postJSON(ctx, p.baseURL+"/wallet/sign", body, "", &out); err != nil {
		return identityprovider.SignResponse{}, err
	}
	out.SignerProvider = ProviderDWalletAPI
	return out, nil
}

// IssueKYCCredential issues a W3C VC via SSI-VC-API POST /vc/issue.
func (p *DWalletAPIProvider) IssueKYCCredential(ctx context.Context, req identityprovider.KYCCredentialRequest) (identityprovider.KYCCredentialResponse, error) {
	var out identityprovider.KYCCredentialResponse
	if err := p.postJSON(ctx, p.baseURL+"/vc/issue", req, "", &out); err != nil {
		return identityprovider.KYCCredentialResponse{}, err
	}
	return out, nil
}

// VerifyKYCProof checks a ZKP pointer via SSI-VC-API GET /vc/verify/:pointer.
func (p *DWalletAPIProvider) VerifyKYCProof(ctx context.Context, zkpPointer string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.baseURL+"/vc/verify/"+zkpPointer, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("provider status %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}
	return result.Valid, nil
}

// GetKYCStatus retrieves the KYC status from SSI-VC-API GET /vc/status/:subject.
func (p *DWalletAPIProvider) GetKYCStatus(ctx context.Context, subject string) (identityprovider.KYCStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.baseURL+"/vc/status/"+subject, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return identityprovider.KYCStatusPending, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("provider status %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return identityprovider.KYCStatus(result.Status), nil
}

// ProvisionParticipant sets KYC status via SSI-VC-API POST /vc/provision.
func (p *DWalletAPIProvider) ProvisionParticipant(ctx context.Context, subject string, status identityprovider.KYCStatus) error {
	body := map[string]string{
		"subject": subject,
		"status":  string(status),
	}
	return p.postJSON(ctx, p.baseURL+"/vc/provision", body, "", &map[string]any{})
}

// --- HTTP helpers ---

func (p *DWalletAPIProvider) postJSON(ctx context.Context, url string, payload any, accessToken string, out any) error {
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
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}
