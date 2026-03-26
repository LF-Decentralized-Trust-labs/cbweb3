// This file handles login, refresh, logout, and onboarding endpoints.
package handlers

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/status"
)

// AuthHandler implements authentication endpoints.
type AuthHandler struct {
	authProvider        interfaces.IAuthProvider
	pkiAuthProvider     interfaces.IPKIAuthProvider     // optional; nil if PKI is not enabled
	clientSecretChanger interfaces.IClientSecretChanger // optional; nil if not supported
	kycChecker          interfaces.KYCChecker
	kycManager          interfaces.KYCManager
	cookieSecure        bool // mirrors COOKIE_SECURE env var; true = HTTPS only
}

type loginRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// NewAuthHandler builds an AuthHandler with its required dependencies.
// cookieSecure should be true when the gateway is served over HTTPS so that
// auth cookies are sent with the Secure flag; use false for plain HTTP (local dev).
func NewAuthHandler(
	authProvider interfaces.IAuthProvider,
	kycChecker interfaces.KYCChecker,
	cookieSecure bool,
) *AuthHandler {
	var kycMgr interfaces.KYCManager
	var pkiProvider interfaces.IPKIAuthProvider
	var secretChanger interfaces.IClientSecretChanger
	if mgr, ok := kycChecker.(interfaces.KYCManager); ok {
		kycMgr = mgr
	}
	if pki, ok := authProvider.(interfaces.IPKIAuthProvider); ok {
		pkiProvider = pki
	}
	if sc, ok := authProvider.(interfaces.IClientSecretChanger); ok {
		secretChanger = sc
	}
	return &AuthHandler{
		authProvider:        authProvider,
		pkiAuthProvider:     pkiProvider,
		clientSecretChanger: secretChanger,
		kycChecker:          kycChecker,
		kycManager:          kycMgr,
		cookieSecure:        cookieSecure,
	}
}

// setAuthCookies injects HttpOnly auth cookies for the access and refresh tokens.
// Both cookies share the same security attributes; refresh cookie is only set when
// the token is non-empty.
func setAuthCookies(c *fiber.Ctx, accessToken, refreshToken string, expiresIn int, secure bool) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   expiresIn,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: "Strict",
	})
	if refreshToken != "" {
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Path:     "/",
			MaxAge:   expiresIn,
			HTTPOnly: true,
			Secure:   secure,
			SameSite: "Strict",
		})
	}
}

// clearAuthCookies removes the auth cookies from the browser by expiring them immediately.
func clearAuthCookies(c *fiber.Ctx, secure bool) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: "Strict",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: "Strict",
	})
}

// Login authenticates a client and returns tokens or a PKI nonce challenge.
//
// Two flows are supported:
//
//  1. PKI nonce request (ROLE_COMMERCIAL_BANK / ROLE_TREASURY):
//     clientSecret is validated as the first factor, then a short-lived nonce
//     is issued. Returns { "nonce": "hex-64-chars" }.
//     The client must sign the nonce with its X.509 private key and submit it
//     via POST /auth/wallet/bind to complete PKI 2FA (step 2).
//
//  2. Direct login (ROLE_GOVERNANCE, ROLE_SUPERVISOR, ROLE_NOC, etc.):
//     clientId + clientSecret are forwarded to the identity service which
//     delegates to Keycloak. Returns { "accessToken", "tokenType", "expiresIn" }.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.ClientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientId is required"})
	}
	if req.ClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientSecret is required"})
	}

	// Attempt PKI nonce flow first (ROLE_COMMERCIAL_BANK / ROLE_TREASURY).
	// IssueLoginNonce validates the clientSecret and checks PKI role eligibility.
	// If the participant is not found or does not require PKI, fall through to
	// normal direct login.
	if h.pkiAuthProvider != nil {
		nonce, err := h.pkiAuthProvider.IssueLoginNonce(c.UserContext(), req.ClientID, req.ClientSecret)
		if err == nil {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"nonce": nonce})
		}
		// Determine whether to fall through or reject hard.
		st, ok := status.FromError(err)
		if !ok {
			// Non-gRPC error (network, timeout, context cancelled) — reject.
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "authentication service unavailable"})
		}
		msg := st.Message()
		if msg != "participant not found" && msg != "PKI_NOT_REQUIRED" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}
		// participant not found or PKI not required → fall through to direct login.
	}

	// Direct login: Central Bank (client_credentials via Keycloak service account)
	// or other non-PKI roles (ROLE_SUPERVISOR, ROLE_NOC, ROLE_GOVERNANCE_OFFICER).
	token, err := h.authProvider.Authenticate(c.UserContext(), req.ClientID, req.ClientSecret)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}
	setAuthCookies(c, token.AccessToken, token.RefreshToken, token.ExpiresIn, h.cookieSecure)
	resp := fiber.Map{
		"accessToken": token.AccessToken,
		"expiresIn":   token.ExpiresIn,
		"tokenType":   token.TokenType,
	}
	if token.RefreshToken != "" {
		resp["refreshToken"] = token.RefreshToken
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// Refresh issues a new access token from a valid refresh token.
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "refreshToken is required"})
	}

	token, err := h.authProvider.RefreshToken(c.UserContext(), body.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired refresh token"})
	}
	setAuthCookies(c, token.AccessToken, token.RefreshToken, token.ExpiresIn, h.cookieSecure)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"accessToken":  token.AccessToken,
		"refreshToken": token.RefreshToken,
		"expiresIn":    token.ExpiresIn,
		"tokenType":    token.TokenType,
	})
}

// Logout revokes the current access token. The token is read from the
// access_token HttpOnly cookie set by login/refresh endpoints.
// On success the auth cookies are cleared.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	rawToken := c.Cookies("access_token")
	if rawToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "access_token cookie is required"})
	}
	if err := h.authProvider.Logout(c.UserContext(), rawToken); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "logout failed"})
	}
	clearAuthCookies(c, h.cookieSecure)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "logged out successfully"})
}

// containsRole checks if a role is in the claims roles list.
// WalletBind handles PKI login step 2 (POST /api/v1/auth/wallet/bind).
// The client submits the DER-encoded ECDSA signature of their nonce (hex) plus
// their X.509 certificate PEM issued by the Central Bank CA.
// On success, a Keycloak JWT is returned.
func (h *AuthHandler) WalletBind(c *fiber.Ctx) error {
	if h.pkiAuthProvider == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "PKI authentication not configured"})
	}

	type walletBindRequest struct {
		UserID            string `json:"user_id"`
		NonceSignatureHex string `json:"nonce_signature_hex"`
		CertPEM           string `json:"cert_pem"`
	}
	var req walletBindRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.UserID == "" || req.NonceSignatureHex == "" || req.CertPEM == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id, nonce_signature_hex, and cert_pem are required",
		})
	}

	token, err := h.pkiAuthProvider.VerifyPKILogin(c.UserContext(), req.UserID, req.NonceSignatureHex, req.CertPEM)
	if err != nil {
		errMsg := "PKI verification failed"
		if st, ok := status.FromError(err); ok {
			msg := st.Message()
			if msg == "INVALID_CERTIFICATE_CHAIN" || msg == "NONCE_SIGNATURE_MISMATCH" {
				errMsg = msg
			}
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": errMsg})
	}

	setAuthCookies(c, token.AccessToken, token.RefreshToken, token.ExpiresIn, h.cookieSecure)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"accessToken":  token.AccessToken,
		"refreshToken": token.RefreshToken,
		"tokenType":    token.TokenType,
		"expiresIn":    token.ExpiresIn,
	})
}

// ChangeClientSecret allows an authenticated user to rotate their clientSecret.
// The user ID is read from the claims injected by RequireCookieAuth middleware.
func (h *AuthHandler) ChangeClientSecret(c *fiber.Ctx) error {
	if h.clientSecretChanger == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "client secret rotation not available"})
	}

	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var body struct {
		CurrentClientSecret string `json:"current_client_secret"`
		NewClientSecret     string `json:"new_client_secret"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.CurrentClientSecret == "" || body.NewClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_client_secret and new_client_secret are required"})
	}

	if chErr := h.clientSecretChanger.ChangeClientSecret(c.UserContext(), claims.Subject, body.CurrentClientSecret, body.NewClientSecret); chErr != nil {
		if st, ok := status.FromError(chErr); ok && st.Message() == "invalid current client secret" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid current client secret"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "client secret rotation failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "client secret changed successfully"})
}

// Me returns the authenticated user's profile from the claims injected by RequireCookieAuth middleware.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	resp := fiber.Map{
		"subject": claims.Subject,
		"issuer":  claims.Issuer,
		"roles":   claims.Roles,
	}
	if claims.Wallet != "" {
		resp["wallet"] = claims.Wallet
	}
	if claims.Country != "" {
		resp["country"] = claims.Country
	}
	if claims.BankID != "" {
		resp["bankId"] = claims.BankID
	}
	if claims.PrivacyGroup != "" {
		resp["privacyGroup"] = claims.PrivacyGroup
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// ResolveChallenger signs a PKI login nonce using the EC private key found in
// the PKI_DIR directory. Intended for MVP use only —
// remove this endpoint before deploying to production.
//
// cert_file can be:
//   - a combined .pem file containing both CERTIFICATE and EC PRIVATE KEY blocks
//   - a .crt file; in this case the matching .key file (same base name) is loaded
//     automatically from PKI_DIR (e.g. "bank-a.crt" → also reads "bank-a.key")
//
// Request: { "nonce": "<64-char hex>", "cert_file": "bank-a.crt" }
// Response: { "nonce_signature_hex": "<hex DER>", "cert_pem": "<PEM>" }
func (h *AuthHandler) ResolveChallenger(c *fiber.Ctx) error { // MVP-only
	var req struct {
		Nonce    string `json:"nonce"`
		CertFile string `json:"cert_file"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Nonce == "" || req.CertFile == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nonce and cert_file are required"})
	}
	// Reject path traversal attempts.
	if strings.Contains(req.CertFile, "/") || strings.Contains(req.CertFile, "..") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cert_file must be a plain filename without path separators"})
	}

	pkiDir := os.Getenv("PKI_DIR")
	if pkiDir == "" {
		pkiDir = "./config/pki"
	}
	certPath := filepath.Join(pkiDir, req.CertFile)

	data, err := os.ReadFile(certPath)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "cert file not found: " + req.CertFile})
	}

	var certPEMBlock, keyPEMBlock *pem.Block
	remaining := data
	for {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			certPEMBlock = block
		case "EC PRIVATE KEY":
			keyPEMBlock = block
		}
	}
	if certPEMBlock == nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "CERTIFICATE block not found in " + req.CertFile})
	}

	// If the private key was not found in the cert file, look for a sibling .key
	// file with the same base name (e.g. "bank-a.crt" → "bank-a.key").
	// This matches the file layout produced by `make pki.gen-commercial-banks`.
	if keyPEMBlock == nil {
		ext := filepath.Ext(req.CertFile)
		keyFileName := req.CertFile[:len(req.CertFile)-len(ext)] + ".key"
		keyPath := filepath.Join(pkiDir, keyFileName)
		keyData, keyErr := os.ReadFile(keyPath)
		if keyErr != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "EC PRIVATE KEY not found in " + req.CertFile + " and no matching key file: " + keyFileName,
			})
		}
		rem := keyData
		for {
			var block *pem.Block
			block, rem = pem.Decode(rem)
			if block == nil {
				break
			}
			if block.Type == "EC PRIVATE KEY" {
				keyPEMBlock = block
				break
			}
		}
		if keyPEMBlock == nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "EC PRIVATE KEY block not found in " + keyFileName})
		}
	}

	privKey, err := x509.ParseECPrivateKey(keyPEMBlock.Bytes)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "failed to parse EC private key: " + err.Error()})
	}

	nonceBytes, err := hex.DecodeString(req.Nonce)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nonce is not valid hex"})
	}
	digest := sha256.Sum256(nonceBytes)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, digest[:])
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "signing failed"})
	}
	sigDER, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encode signature"})
	}

	certPEM := string(pem.EncodeToMemory(certPEMBlock))

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"nonce_signature_hex": hex.EncodeToString(sigDER),
		"cert_pem":            certPEM,
	})
}

func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
