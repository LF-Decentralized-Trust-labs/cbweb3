// Package cbclient provides an HTTP client for commercial bank → central bank
// communication in the 3-phase onboarding flow.
//
// The commercial bank uses this client to:
//   - Submit a credential request (Phase 1)
//   - Poll onboarding status to discover KYC approval + PoP nonce (Phase 2.5)
//   - Complete onboarding with PoP signature (Phase 3)
package cbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds the connection parameters for the central bank API.
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// Client communicates with the central bank's onboarding endpoints.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// New creates a Client from the given Config.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    cfg.BaseURL,
	}
}

// CredentialRequest is the Phase 1 request body.
type CredentialRequest struct {
	CsrPem              string `json:"csr_pem"`
	BlockchainPubKeyHex string `json:"blockchain_pub_key_hex"`
	InstitutionName     string `json:"institution_name"`
	CNPJ                string `json:"cnpj,omitempty"`
	BankCode            string `json:"bank_code"`
	Country             string `json:"country"`
	Role                string `json:"role"`
	Email               string `json:"email"`
	Username            string `json:"username"`
}

// CredentialResponse is the Phase 1 response.
type CredentialResponse struct {
	RequestID     string `json:"request_id"`
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	Status        string `json:"status"`
}

// OnboardingStatusResponse is the Phase 2.5 polling response.
type OnboardingStatusResponse struct {
	RequestID     string `json:"request_id"`
	UserID        string `json:"user_id"`
	Status        string `json:"status"`
	PopNonce      string `json:"pop_nonce,omitempty"`
	WalletAddress string `json:"wallet_address"`
}

// CompleteOnboardingRequest is the Phase 3 request body.
type CompleteOnboardingRequest struct {
	RequestID           string `json:"request_id"`
	UserID              string `json:"user_id"`
	PopSignatureHex     string `json:"pop_signature_hex"`
	BlockchainPubKeyHex string `json:"blockchain_pub_key_hex"`
}

// CompleteOnboardingResponse is the Phase 3 response.
type CompleteOnboardingResponse struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	CertPEM       string `json:"cert_pem"`
	TxHash        string `json:"tx_hash"`
	ClientSecret  string `json:"client_secret"`
	Status        string `json:"status"`
}

// SubmitCredentialRequest sends a Phase 1 credential request to the central bank.
func (c *Client) SubmitCredentialRequest(ctx context.Context, req CredentialRequest) (CredentialResponse, error) {
	var resp CredentialResponse
	if err := c.post(ctx, "/api/v1/onboarding/credential-request", req, &resp); err != nil {
		return CredentialResponse{}, err
	}
	return resp, nil
}

// GetOnboardingStatus polls the central bank for the current onboarding status.
func (c *Client) GetOnboardingStatus(ctx context.Context, requestID string) (OnboardingStatusResponse, error) {
	var resp OnboardingStatusResponse
	if err := c.get(ctx, "/api/v1/onboarding/status/"+requestID, &resp); err != nil {
		return OnboardingStatusResponse{}, err
	}
	return resp, nil
}

// CompleteOnboarding sends the Phase 3 PoP signature to the central bank.
func (c *Client) CompleteOnboarding(ctx context.Context, req CompleteOnboardingRequest) (CompleteOnboardingResponse, error) {
	var resp CompleteOnboardingResponse
	if err := c.post(ctx, "/api/v1/onboarding/complete", req, &resp); err != nil {
		return CompleteOnboardingResponse{}, err
	}
	return resp, nil
}

func (c *Client) post(ctx context.Context, path string, body, target interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("cbclient: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("cbclient: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	return c.doRequest(httpReq, target)
}

func (c *Client) get(ctx context.Context, path string, target interface{}) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("cbclient: build request: %w", err)
	}

	return c.doRequest(httpReq, target)
}

func (c *Client) doRequest(httpReq *http.Request, target interface{}) error {
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("cbclient: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cbclient: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("cbclient: %s %s returned %d: %s",
			httpReq.Method, httpReq.URL.Path, resp.StatusCode, string(respBody))
	}

	if target != nil {
		if err := json.Unmarshal(respBody, target); err != nil {
			return fmt.Errorf("cbclient: unmarshal response: %w", err)
		}
	}
	return nil
}
