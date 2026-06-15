// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// OnboardingProxyHandler forwards onboarding requests from a commercial bank
// gateway to the Central Bank's public onboarding endpoints. When a KeyManager
// and PKI directory are configured, the proxy enriches the payload with CSR
// and blockchain key data before forwarding (smart proxy mode).
type OnboardingProxyHandler struct {
	client   *http.Client
	baseURL  string // Central Bank API base URL
	pkiDir   string // path to PKI files (CSR, keys)
	bankCode string // commercial bank identifier (e.g. "bank-a")
	keyMgr   interfaces.OnboardingKeyManager
}

// NewOnboardingProxyHandler creates a proxy handler targeting the given Central
// Bank URL. When pkiDir, bankCode and keyMgr are provided, the proxy operates
// in smart mode: it injects CSR and blockchain keys into the payload
// automatically, simplifying the frontend's burden.
func NewOnboardingProxyHandler(centralBankURL, pkiDir, bankCode string, keyMgr interfaces.OnboardingKeyManager) *OnboardingProxyHandler {
	return &OnboardingProxyHandler{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  strings.TrimRight(centralBankURL, "/"),
		pkiDir:   pkiDir,
		bankCode: bankCode,
		keyMgr:   keyMgr,
	}
}

// isSmartMode returns true when the proxy can enrich payloads.
func (h *OnboardingProxyHandler) isSmartMode() bool {
	return h.keyMgr != nil && h.pkiDir != "" && h.bankCode != ""
}

// InitiateCredentialRequest proxies POST /onboarding/initiate to the Central
// Bank's POST /api/v1/onboarding/credential-request.
//
// In smart mode the proxy:
//  1. Reads the CSR from PKI_DIR/{bankCode}.csr
//  2. Calls KMS to create/retrieve a secp256k1 key pair
//  3. Injects csr_pem and blockchain_pub_key_hex into the payload
//
// The frontend only needs to send: institution_name, bank_code, country, role,
// email, username.
func (h *OnboardingProxyHandler) InitiateCredentialRequest(c *fiber.Ctx) error {
	if !h.isSmartMode() {
		return h.proxy(c, http.MethodPost, "/api/v1/onboarding/credential-request", c.Body())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	// Read CSR from filesystem.
	csrPath := filepath.Join(h.pkiDir, h.bankCode+".csr")
	csrBytes, err := os.ReadFile(csrPath) // #nosec G304 -- path built from service config (pkiDir + bankCode), not user input
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("read CSR (%s): %v", csrPath, err),
		})
	}

	// Create/retrieve blockchain key via KMS.
	pubKeyHex, _, err := h.keyMgr.CreateOnboardingKey(c.UserContext(), h.bankCode)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("create onboarding key: %v", err),
		})
	}

	body["csr_pem"] = string(csrBytes)
	body["blockchain_pub_key_hex"] = pubKeyHex
	body["bank_code"] = h.bankCode

	enriched, err := json.Marshal(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "marshal enriched payload"})
	}

	return h.proxy(c, http.MethodPost, "/api/v1/onboarding/credential-request", enriched)
}

// GetOnboardingStatus proxies GET /onboarding/status/:requestId to the Central
// Bank's GET /api/v1/onboarding/status/:requestId.
func (h *OnboardingProxyHandler) GetOnboardingStatus(c *fiber.Ctx) error {
	requestID := c.Params("requestId")
	if requestID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "requestId is required"})
	}
	return h.proxy(c, http.MethodGet, "/api/v1/onboarding/status/"+requestID, nil)
}

// CompleteOnboarding proxies POST /onboarding/complete to the Central Bank's
// POST /api/v1/onboarding/complete.
//
// In smart mode the proxy:
//  1. Fetches onboarding status from CB to retrieve the pop_nonce
//  2. Calls KMS to sign the nonce (PoP)
//  3. Injects pop_signature_hex and blockchain_pub_key_hex into the payload
//
// The frontend only needs to send: request_id, user_id.
func (h *OnboardingProxyHandler) CompleteOnboarding(c *fiber.Ctx) error {
	if !h.isSmartMode() {
		return h.proxy(c, http.MethodPost, "/api/v1/onboarding/complete", c.Body())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	requestID, _ := body["request_id"].(string)
	userID, _ := body["user_id"].(string)
	if requestID == "" || userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "request_id and user_id are required",
		})
	}

	// 1. Fetch pop_nonce from CB onboarding status.
	statusResp, statusCode, err := h.fetchJSON(c, http.MethodGet, "/api/v1/onboarding/status/"+requestID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": fmt.Sprintf("fetch onboarding status: %v", err),
		})
	}
	if statusCode != http.StatusOK {
		return c.Status(statusCode).Send(statusResp)
	}

	var statusBody struct {
		PopNonce string `json:"pop_nonce"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(statusResp, &statusBody); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "parse status response"})
	}
	if statusBody.PopNonce == "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": fmt.Sprintf("pop_nonce not available; current status: %s", statusBody.Status),
		})
	}

	// 2. Sign PoP nonce via KMS.
	sigHex, pubKeyHex, err := h.keyMgr.SignOnboardingPoP(c.UserContext(), h.bankCode, statusBody.PopNonce)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("sign PoP nonce: %v", err),
		})
	}

	// 3. Inject signature and public key.
	body["pop_signature_hex"] = sigHex
	body["blockchain_pub_key_hex"] = pubKeyHex

	enriched, err := json.Marshal(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "marshal enriched payload"})
	}

	// Forward to CB.
	cbResp, cbCode, cbErr := h.postJSON(c, "/api/v1/onboarding/complete", enriched)
	if cbErr != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": fmt.Sprintf("proxy: central bank unreachable: %v", cbErr),
		})
	}
	if cbCode != http.StatusOK {
		c.Set("Content-Type", "application/json")
		return c.Status(cbCode).Send(cbResp)
	}

	// Parse CB response to extract cert_pem and client_secret for PKI login.
	var cbBody map[string]interface{}
	if err := json.Unmarshal(cbResp, &cbBody); err != nil {
		c.Set("Content-Type", "application/json")
		return c.Status(fiber.StatusOK).Send(cbResp)
	}

	certPEM, _ := cbBody["cert_pem"].(string)
	clientSecret, _ := cbBody["client_secret"].(string)
	cbUserID, _ := cbBody["user_id"].(string)
	if cbUserID == "" {
		cbUserID = userID
	}

	// Save cert_pem to disk for future re-logins.
	if certPEM != "" {
		certPath := filepath.Join(h.pkiDir, h.bankCode+"-participant.crt")
		if writeErr := os.WriteFile(certPath, []byte(certPEM), 0600); writeErr != nil {
			log.Printf("[onboarding-proxy] WARNING: failed to save cert to %s: %v", certPath, writeErr)
		}
	}

	// Chain PKI login (best-effort).
	if certPEM != "" && clientSecret != "" {
		accessToken, loginErr := h.performPKILogin(c, cbUserID, clientSecret, certPEM)
		if loginErr != nil {
			log.Printf("[onboarding-proxy] WARNING: chained PKI login failed: %v", loginErr)
			cbBody["pki_login_error"] = loginErr.Error()
		} else {
			cbBody["access_token"] = accessToken
		}
	}

	finalResp, _ := json.Marshal(cbBody)
	c.Set("Content-Type", "application/json")
	return c.Status(fiber.StatusOK).Send(finalResp)
}

func (h *OnboardingProxyHandler) proxy(c *fiber.Ctx, method, path string, body []byte) error {
	url := h.baseURL + path

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(c.UserContext(), method, url, bodyReader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("proxy: build request: %v", err),
		})
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if corrID := c.Get("X-Correlation-Id"); corrID != "" {
		req.Header.Set("X-Correlation-Id", corrID)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": fmt.Sprintf("proxy: central bank unreachable: %v", err),
		})
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "proxy: failed to read central bank response",
		})
	}

	c.Set("Content-Type", resp.Header.Get("Content-Type"))
	return c.Status(resp.StatusCode).Send(respBody)
}

// fetchJSON performs an HTTP request to the CB and returns the raw response body and status code.
func (h *OnboardingProxyHandler) fetchJSON(c *fiber.Ctx, method, path string) ([]byte, int, error) {
	url := h.baseURL + path

	req, err := http.NewRequestWithContext(c.UserContext(), method, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	if corrID := c.Get("X-Correlation-Id"); corrID != "" {
		req.Header.Set("X-Correlation-Id", corrID)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("central bank unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// postJSON performs an HTTP POST with a JSON body to the CB and returns the raw response body and status code.
func (h *OnboardingProxyHandler) postJSON(c *fiber.Ctx, path string, body []byte) ([]byte, int, error) {
	url := h.baseURL + path

	req, err := http.NewRequestWithContext(c.UserContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if corrID := c.Get("X-Correlation-Id"); corrID != "" {
		req.Header.Set("X-Correlation-Id", corrID)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("central bank unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// signNonceP256 signs a hex-encoded nonce using the P-256 private key found in
// PKI_DIR/{bankCode}.key and returns the DER-encoded signature as hex.
func (h *OnboardingProxyHandler) signNonceP256(nonceHex string) (string, error) {
	keyPath := filepath.Join(h.pkiDir, h.bankCode+".key")
	keyData, err := os.ReadFile(keyPath) // #nosec G304 -- path built from service config (pkiDir + bankCode), not user input
	if err != nil {
		return "", fmt.Errorf("read key %s: %w", keyPath, err)
	}

	var keyBlock *pem.Block
	remaining := keyData
	for {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type == "EC PRIVATE KEY" {
			keyBlock = block
			break
		}
	}
	if keyBlock == nil {
		return "", fmt.Errorf("EC PRIVATE KEY block not found in %s", keyPath)
	}

	privKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse EC private key: %w", err)
	}

	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil {
		return "", fmt.Errorf("nonce is not valid hex: %w", err)
	}

	digest := sha256.Sum256(nonceBytes)
	r, s, err := ecdsa.Sign(rand.Reader, privKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("ecdsa sign: %w", err)
	}

	sigDER, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	if err != nil {
		return "", fmt.Errorf("marshal DER signature: %w", err)
	}

	return hex.EncodeToString(sigDER), nil
}

// performPKILogin executes the two-step PKI login against the Central Bank:
//  1. POST /auth/login with userID + clientSecret to get a nonce
//  2. Sign the nonce with P-256 key
//  3. POST /auth/wallet/bind with signed nonce + cert_pem to get an accessToken
func (h *OnboardingProxyHandler) performPKILogin(c *fiber.Ctx, userID, clientSecret, certPEM string) (string, error) {
	// Step 1: obtain nonce
	loginPayload, _ := json.Marshal(map[string]string{
		"clientId":     userID,
		"clientSecret": clientSecret,
	})
	loginResp, loginCode, err := h.postJSON(c, "/api/v1/auth/login", loginPayload)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	if loginCode != http.StatusOK {
		return "", fmt.Errorf("login returned HTTP %d: %s", loginCode, string(loginResp))
	}

	var loginBody struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(loginResp, &loginBody); err != nil {
		return "", fmt.Errorf("parse login response: %w", err)
	}
	if loginBody.Nonce == "" {
		return "", fmt.Errorf("no nonce in login response")
	}

	// Step 2: sign the nonce with P-256
	sigHex, err := h.signNonceP256(loginBody.Nonce)
	if err != nil {
		return "", fmt.Errorf("sign nonce: %w", err)
	}

	// Step 3: wallet/bind
	bindPayload, _ := json.Marshal(map[string]string{
		"user_id":             userID,
		"nonce_signature_hex": sigHex,
		"cert_pem":            certPEM,
	})
	bindResp, bindCode, err := h.postJSON(c, "/api/v1/auth/wallet/bind", bindPayload)
	if err != nil {
		return "", fmt.Errorf("wallet/bind request: %w", err)
	}
	if bindCode != http.StatusOK {
		return "", fmt.Errorf("wallet/bind returned HTTP %d: %s", bindCode, string(bindResp))
	}

	var bindBody struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(bindResp, &bindBody); err != nil {
		return "", fmt.Errorf("parse wallet/bind response: %w", err)
	}
	if bindBody.AccessToken == "" {
		return "", fmt.Errorf("no accessToken in wallet/bind response")
	}

	return bindBody.AccessToken, nil
}

// PKILogin handles POST /auth/pki-login for re-login after onboarding.
// It reads the saved participant certificate from PKI_DIR/{bankCode}-participant.crt,
// authenticates against the Central Bank via the two-step PKI flow, and returns
// the access token.
//
// Request:  { "user_id": "<uuid>", "client_secret": "<secret>" }
// Response: { "accessToken": "<jwt>" }
func (h *OnboardingProxyHandler) PKILogin(c *fiber.Ctx) error {
	var req struct {
		UserID       string `json:"user_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.UserID == "" || req.ClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id and client_secret are required"})
	}

	// Read saved participant certificate.
	certPath := filepath.Join(h.pkiDir, h.bankCode+"-participant.crt")
	certData, err := os.ReadFile(certPath) // #nosec G304 -- path built from service config (pkiDir + bankCode), not user input
	if err != nil {
		return c.Status(fiber.StatusPreconditionFailed).JSON(fiber.Map{
			"error": fmt.Sprintf("participant certificate not found at %s; complete onboarding first", certPath),
		})
	}

	accessToken, err := h.performPKILogin(c, req.UserID, req.ClientSecret, string(certData))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("PKI login failed: %v", err),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"accessToken": accessToken,
	})
}

// GetMyOnboardingStatus handles GET /api/v1/onboarding/my-status.
// Resolves the bank identity from the JWT session (BankID claim) and proxies
// to the Central Bank's /api/v1/onboarding/my-status?bank_code=<resolved>.
// Falls back to the configured bankCode when BankID is absent from the token.
func (h *OnboardingProxyHandler) GetMyOnboardingStatus(c *fiber.Ctx) error {
	bankCode := h.bankCode
	if claims, ok := c.Locals("claims").(domain.TokenClaims); ok && claims.BankID != "" {
		bankCode = claims.BankID
	}
	if bankCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to determine bank identity from session",
		})
	}
	return h.proxy(c, http.MethodGet, "/api/v1/onboarding/my-status?bank_code="+bankCode, nil)
}
