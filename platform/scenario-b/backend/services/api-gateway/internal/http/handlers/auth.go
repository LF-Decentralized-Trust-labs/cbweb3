// SPDX-License-Identifier: Apache-2.0

// This file handles login, refresh, logout, and onboarding endpoints.
package handlers

import (
	"fmt"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"strings"
	"time"
)

// AuthHandler implements authentication endpoints.
type AuthHandler struct {
	authProvider        interfaces.IAuthProvider
	pkiAuthProvider     interfaces.IPKIAuthProvider     // optional; nil if PKI is not enabled
	clientSecretChanger interfaces.IClientSecretChanger // optional; nil if not supported
	kycChecker          interfaces.KYCChecker
	kycManager          interfaces.KYCManager
	cookieSecure        bool   // mirrors COOKIE_SECURE env var; true = HTTPS only
	csrfSecret          []byte // keys the HMAC binding a CSRF token to its session
	// entityWallet is this gateway's own on-chain address (ENTITY_BESU_ADDRESS), served on
	// /auth/me when the identity provider issues no wallet claim — which is always, today.
	//
	// The portal renders this field on the issuance screen so an operator can confirm which
	// wallet money will be created against. With nothing to render it said "Not available in
	// session", minutes after onboarding had shown the address on its own success screen. The
	// address was never missing: it is what every deposit this gateway creates carries as
	// requester_besu_address.
	entityWallet string
}

type loginRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// Stable error codes for the auth routes.
//
// The five portals used to render axios's own `error.message`, so an operator saw "Request failed
// with status code 400" and could not tell a wrong secret from an empty field from a gateway that
// was down. Keying the frontend on the prose instead would break the moment a message is reworded,
// and keying on the status is not enough: 400 covers both a missing field and a malformed body.
//
// So the body carries a code as well as the message — the same convention the relay-rejection
// classifier already relies on. `error` is unchanged: it is what logs and existing clients read.
//
// Scoped to the login and refresh routes, the ones the portals key on. The other handlers in this
// file still answer with `error` alone; extending them is a separate change with its own callers.
const (
	// CodeInvalidRequest is a request the gateway could not parse. A client bug, not something the
	// operator can fix by typing more carefully.
	CodeInvalidRequest = "INVALID_REQUEST"
	// CodeMissingCredentials is an absent clientId or clientSecret. One code for either field: the
	// message may name which, but no caller should have to parse prose to find out.
	CodeMissingCredentials = "MISSING_CREDENTIALS"
	// CodeInvalidCredentials is a credential the identity provider refused. One code for both an
	// unknown client and a wrong secret — distinguishing them is user enumeration.
	CodeInvalidCredentials = "INVALID_CREDENTIALS" //#nosec G101 -- not a secret; a wire error code, no credential material
	// CodeAuthServiceUnavailable is the auth service being unreachable or not ready. Deliberately
	// not an auth failure: telling an operator their credentials are wrong when the service is down
	// sends them to rotate a secret that was fine.
	CodeAuthServiceUnavailable = "AUTH_SERVICE_UNAVAILABLE"
	// CodeMissingRefreshToken means no refresh token was presented — usually a session-restore probe
	// on a page that has no session yet. A caller that recognises this can stay silent instead of
	// rendering "refreshToken is required" as a login failure, which is what an operator reported
	// seeing on a login screen.
	CodeMissingRefreshToken = "MISSING_REFRESH_TOKEN"
	// CodeInvalidRefreshToken is a refresh token the identity provider rejected: a real expiry.
	CodeInvalidRefreshToken = "INVALID_REFRESH_TOKEN"
)

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

// WithCSRFSecret sets the key that binds CSRF tokens to their session.
//
// A setter rather than a constructor parameter because NewAuthHandler has a dozen
// call sites across tests; the wiring that matters is in main.go, and a test that
// does not exercise CSRF should not have to name a secret.
func (h *AuthHandler) WithCSRFSecret(secret []byte) *AuthHandler {
	h.csrfSecret = secret
	return h
}

// setAuthCookies injects HttpOnly auth cookies for the access and refresh tokens.
// The refresh cookie uses its own MaxAge (refreshExpiresIn) so it outlives the
// access token, enabling silent refresh. When refreshExpiresIn is 0 the access
// token lifetime is used as fallback.
// The CSRF token is minted HERE, alongside the session, rather than at login only.
// It is bound to the access token, and a refresh issues a new access token — so a
// token minted at login stops validating the moment the session is refreshed. Both
// paths already funnel through this function, which is why it is the right place.
func setAuthCookies(c *fiber.Ctx, accessToken, refreshToken string, expiresIn, refreshExpiresIn int, secure bool, csrfSecret []byte) error {
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
		refMaxAge := refreshExpiresIn
		if refMaxAge <= 0 {
			refMaxAge = expiresIn
		}
		c.Cookie(&fiber.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Path:     "/",
			MaxAge:   refMaxAge,
			HTTPOnly: true,
			Secure:   secure,
			SameSite: "Strict",
		})
	}

	csrfToken, err := middleware.NewCSRFToken(csrfSecret, accessToken)
	if err != nil {
		// Propagated, never swallowed: emitting an empty XSRF-TOKEN cookie would
		// lock the browser out of every mutating request, and the only clue would
		// be a 403 with no cause.
		return fmt.Errorf("mint csrf token: %w", err)
	}
	// Deliberately NOT HttpOnly: the browser has to read this one to echo it in the
	// X-XSRF-TOKEN header. That is the whole double-submit mechanism, and it is safe
	// precisely because the session cookie beside it stays HttpOnly.
	c.Cookie(&fiber.Cookie{
		Name:     middleware.CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   expiresIn,
		HTTPOnly: false,
		Secure:   secure,
		SameSite: "Strict",
	})
	return nil
}

// clearAuthCookies deletes the auth cookies from the browser.
//
// Expires-in-the-past, NOT MaxAge: -1. Fiber writes a Max-Age attribute only when the value is
// positive, so a negative one is dropped without warning and the browser receives a cookie with
// an empty value and NO expiry — cleared in effect, since an empty session fails authentication,
// but still sitting in the jar until the browser closes. Setting Expires is what deletes it.
//
// The bug was invisible from Go: a test that reads MaxAge back sees the value the handler set,
// not what Fiber wrote, so it passes both before and after. logout_cookie_wire_test.go asserts
// the Set-Cookie header instead, which is the only place the difference exists.
//
// The NOC backend already did it this way (noc-backend/internal/api/auth.go); this gateway is
// the half that had not caught up. Both scenarios carry the same fix — the same hole was on
// each side, so it is not drift.
func clearAuthCookies(c *fiber.Ctx, secure bool) {
	past := time.Now().Add(-time.Hour)
	for _, name := range []string{middleware.CSRFCookieName, "access_token", "refresh_token"} {
		c.Cookie(&fiber.Cookie{
			Name:  name,
			Value: "",
			Path:  "/",
			// Expires, not MaxAge — see above.
			Expires: past,
			// The CSRF cookie is read by the browser's JS to echo the token back; the session
			// cookies are not. Unchanged from before this fix.
			HTTPOnly: name != middleware.CSRFCookieName,
			Secure:   secure,
			SameSite: "Strict",
		})
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body", "code": CodeInvalidRequest})
	}
	if req.ClientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientId is required", "code": CodeMissingCredentials})
	}
	if req.ClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientSecret is required", "code": CodeMissingCredentials})
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
			// Non-gRPC error (network, timeout, context cancelled) — treat as service unavailable.
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "authentication service unavailable", "code": CodeAuthServiceUnavailable})
		}
		switch st.Code() {
		case codes.NotFound, codes.PermissionDenied:
			// "participant not found" or "PKI_NOT_REQUIRED" — fall through to direct login.
		case codes.Unauthenticated:
			// PKI first factor failed (wrong clientSecret).
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials", "code": CodeInvalidCredentials})
		default:
			// Internal error, Unavailable (service not ready), etc. — do not leak as auth failure.
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "authentication service unavailable", "code": CodeAuthServiceUnavailable})
		}
	}

	// Direct login: Central Bank (client_credentials via Keycloak service account)
	// or other non-PKI roles (ROLE_SUPERVISOR, ROLE_NOC, ROLE_GOVERNANCE_OFFICER).
	token, err := h.authProvider.Authenticate(c.UserContext(), req.ClientID, req.ClientSecret)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials", "code": CodeInvalidCredentials})
	}
	if err := setAuthCookies(c, token.AccessToken, token.RefreshToken, token.ExpiresIn, token.RefreshExpiresIn, h.cookieSecure, h.csrfSecret); err != nil {
		log.Printf("[auth] %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "could not establish a session",
			"code":  CodeAuthServiceUnavailable,
		})
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
// The refresh token can be supplied in the JSON body or read from the
// HttpOnly refresh_token cookie (preferred for browser clients).
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = c.BodyParser(&body)

	rt := body.RefreshToken
	if rt == "" {
		rt = c.Cookies("refresh_token")
	}
	if rt == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "refreshToken is required", "code": CodeMissingRefreshToken})
	}

	token, err := h.authProvider.RefreshToken(c.UserContext(), rt)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired refresh token", "code": CodeInvalidRefreshToken})
	}
	if err := setAuthCookies(c, token.AccessToken, token.RefreshToken, token.ExpiresIn, token.RefreshExpiresIn, h.cookieSecure, h.csrfSecret); err != nil {
		log.Printf("[auth] %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "could not establish a session",
			"code":  CodeAuthServiceUnavailable,
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"accessToken":  token.AccessToken,
		"refreshToken": token.RefreshToken,
		"expiresIn":    token.ExpiresIn,
		"tokenType":    token.TokenType,
	})
}

// Logout revokes the current session. The refresh_token HttpOnly cookie is
// sent to Keycloak for proper session revocation. Falls back to access_token
// for backward compatibility.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	revokeToken := c.Cookies("refresh_token")
	if revokeToken == "" {
		revokeToken = c.Cookies("access_token")
	}
	if revokeToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth cookie is required"})
	}
	if err := h.authProvider.Logout(c.UserContext(), revokeToken); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "logout failed"})
	}
	clearAuthCookies(c, h.cookieSecure)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "logged out successfully"})
}

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
		httpStatus := fiber.StatusUnauthorized
		if st, ok := status.FromError(err); ok {
			msg := st.Message()
			switch {
			case msg == "INVALID_CERTIFICATE_CHAIN" || msg == "NONCE_SIGNATURE_MISMATCH":
				errMsg = msg
			case st.Code() == codes.PermissionDenied:
				errMsg = msg
				httpStatus = fiber.StatusForbidden
			case st.Code() == codes.Internal:
				log.Printf("WARN: WalletBind: internal error for user %s: %s", req.UserID, msg)
				httpStatus = fiber.StatusInternalServerError
			}
		}
		return c.Status(httpStatus).JSON(fiber.Map{"error": errMsg})
	}

	if err := setAuthCookies(c, token.AccessToken, token.RefreshToken, token.ExpiresIn, token.RefreshExpiresIn, h.cookieSecure, h.csrfSecret); err != nil {
		log.Printf("[auth] %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "could not establish a session",
			"code":  CodeAuthServiceUnavailable,
		})
	}
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

// WithEntityWallet sets this gateway's own on-chain address, published on /auth/me when the
// token carries no wallet claim. Optional: a gateway without one omits the field entirely
// rather than answering with an empty string, which a portal would render as a wallet.
func (h *AuthHandler) WithEntityWallet(address string) *AuthHandler {
	h.entityWallet = strings.TrimSpace(address)
	return h
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
	// The claim wins when present — it is the caller's own identity, while entityWallet is the
	// gateway's. They coincide on a bank gateway, where the operator acts as the institution,
	// and a future provider that does issue the claim must not be overridden by configuration.
	if claims.Wallet != "" {
		resp["wallet"] = claims.Wallet
	} else if h.entityWallet != "" {
		resp["wallet"] = h.entityWallet
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
