// This file handles login, refresh, logout, and onboarding endpoints.
package handlers

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/status"
)

// AuthHandler implements authentication endpoints.
type AuthHandler struct {
	authProvider    interfaces.IAuthProvider
	pkiAuthProvider     interfaces.IPKIAuthProvider     // optional; nil if PKI is not enabled
	clientSecretChanger interfaces.IClientSecretChanger // optional; nil if not supported
	kycChecker      interfaces.KYCChecker
	kycManager      interfaces.KYCManager
}

type loginRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// NewAuthHandler builds an AuthHandler with its required dependencies.
func NewAuthHandler(
	authProvider interfaces.IAuthProvider,
	kycChecker interfaces.KYCChecker,
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
	}
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
		if st, ok := status.FromError(err); ok {
			msg := st.Message()
			if msg != "participant not found" && msg != "PKI_NOT_REQUIRED" {
				// Auth failure (invalid credentials, internal error) — reject.
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
			}
		}
		// participant not found or PKI not required → fall through to direct login.
	}

	// Direct login: Central Bank (client_credentials via Keycloak service account)
	// or other non-PKI roles (ROLE_SUPERVISOR, ROLE_NOC, ROLE_GOVERNANCE_OFFICER).
	token, err := h.authProvider.Authenticate(c.UserContext(), req.ClientID, req.ClientSecret)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}
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
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"accessToken":  token.AccessToken,
		"refreshToken": token.RefreshToken,
		"expiresIn":    token.ExpiresIn,
		"tokenType":    token.TokenType,
	})
}

// Logout revokes the current access token (Bearer from header).
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bearer token is required"})
	}
	if err := h.authProvider.Logout(c.UserContext(), token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "logout failed"})
	}
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

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"accessToken":  token.AccessToken,
		"refreshToken": token.RefreshToken,
		"tokenType":    token.TokenType,
		"expiresIn":    token.ExpiresIn,
	})
}

// ChangeClientSecret allows an authenticated user to rotate their clientSecret.
// Requires a valid Bearer token; the user ID is extracted from the token claims.
func (h *AuthHandler) ChangeClientSecret(c *fiber.Ctx) error {
	if h.clientSecretChanger == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "client secret rotation not available"})
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

	authHeader := c.Get("Authorization")
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")
	if rawToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "bearer token required"})
	}
	claims, err := h.authProvider.Validate(c.UserContext(), rawToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}

	if chErr := h.clientSecretChanger.ChangeClientSecret(c.UserContext(), claims.Subject, body.CurrentClientSecret, body.NewClientSecret); chErr != nil {
		if st, ok := status.FromError(chErr); ok && st.Message() == "invalid current client secret" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid current client secret"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "client secret rotation failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "client secret changed successfully"})
}

func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
