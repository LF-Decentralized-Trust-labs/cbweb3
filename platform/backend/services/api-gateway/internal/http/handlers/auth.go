// This file handles login, refresh, logout, onboarding and wallet binding endpoints.
package handlers

import (
	"errors"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// AuthHandler implements authentication and wallet-binding endpoints.
type AuthHandler struct {
	authProvider        interfaces.IAuthProvider
	identityManager     interfaces.IIdentityManager
	kycChecker          interfaces.KYCChecker
	kycManager          interfaces.KYCManager
	participantRegistrar interfaces.ParticipantRegistrar
}

type loginRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type walletBindRequest struct {
	WalletAddress string `json:"walletAddress"`
	Signature     string `json:"signature"`
}

type onboardingRequest struct {
	Country         string `json:"country"`
	BankCode        string `json:"bank_code"`
	Role            string `json:"role"`
	InstitutionName string `json:"institution_name"`
	WalletType      string `json:"wallet_type"`
}

// NewAuthHandler builds an AuthHandler with its required dependencies.
func NewAuthHandler(
	authProvider interfaces.IAuthProvider,
	identityManager interfaces.IIdentityManager,
	kycChecker interfaces.KYCChecker,
) *AuthHandler {
	// Wire optional interfaces via type assertion for backward compat in tests
	var kycMgr interfaces.KYCManager
	var participantReg interfaces.ParticipantRegistrar
	if mgr, ok := identityManager.(interfaces.KYCManager); ok {
		kycMgr = mgr
	}
	if reg, ok := identityManager.(interfaces.ParticipantRegistrar); ok {
		participantReg = reg
	}
	return &AuthHandler{
		authProvider:        authProvider,
		identityManager:     identityManager,
		kycChecker:          kycChecker,
		kycManager:          kycMgr,
		participantRegistrar: participantReg,
	}
}

// Login authenticates a client and returns access + refresh tokens.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.ClientID == "" || req.ClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientId and clientSecret are required"})
	}

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

// Onboarding registers a new participant. It enforces the KYC gate for non-Central Bank actors.
//
// KYC Gate logic (REQ-COM-007):
//   - CENTRAL_BANK role → bypass (they ARE the KYC authority)
//   - APPROVED        → proceed with registration
//   - PENDING         → 202 Accepted (awaiting central bank approval)
//   - FROZEN/REVOKED/REJECTED → 403 Forbidden
func (h *AuthHandler) Onboarding(c *fiber.Ctx) error {
	rawClaims := c.Locals("claims")
	claims, ok := rawClaims.(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token claims"})
	}

	var req onboardingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	isCentralBank := containsRole(claims.Roles, domain.RoleCentralBank)

	if !isCentralBank && h.kycManager != nil {
		kycStatus, err := h.kycManager.GetKYCStatus(c.UserContext(), claims.Subject)
		if err != nil {
			kycStatus = domain.KYCStatus(h.kycChecker.GetStatus(claims.Subject))
		}
		switch kycStatus {
		case domain.KYCApproved:
			// proceed
		case domain.KYCPending:
			return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
				"message": "awaiting central bank approval",
				"subject": claims.Subject,
			})
		default:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":  "participant is not eligible for onboarding",
				"status": string(kycStatus),
			})
		}
	}

	if h.participantRegistrar == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "registration not available"})
	}

	result, err := h.participantRegistrar.RegisterParticipant(
		c.UserContext(),
		strings.TrimPrefix(c.Get("Authorization"), "Bearer "),
		req.Country,
		req.BankCode,
		req.Role,
		req.InstitutionName,
		req.WalletType,
	)
	if err != nil {
		if errors.Is(err, domain.ErrWalletAlreadyBound) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "onboarding failed"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"userId":         result.UserID,
		"did":            result.DID,
		"walletAddress":  result.WalletAddress,
		"signerProvider": result.SignerProvider,
	})
}

// WalletBind validates a signed wallet binding request for the authenticated subject.
func (h *AuthHandler) WalletBind(c *fiber.Ctx) error {
	rawClaims := c.Locals("claims")
	claims, ok := rawClaims.(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token claims"})
	}

	var req walletBindRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.WalletAddress == "" || req.Signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "walletAddress and signature are required"})
	}

	kycStatus := domain.KYCStatus(h.kycChecker.GetStatus(claims.Subject))
	if kycStatus != domain.KYCApproved {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "kyc status does not allow wallet binding"})
	}

	if err := auth.VerifyWalletSignature(claims.Subject, req.WalletAddress, req.Signature); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid signature"})
	}

	binding, err := h.identityManager.BindWallet(c.UserContext(), claims.Subject, req.WalletAddress)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrWalletAlreadyBound), errors.Is(err, domain.ErrUserAlreadyBound):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unable to bind wallet"})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"userId":        binding.UserID,
		"walletAddress": binding.WalletAddress,
		"status":        "BOUND",
	})
}

// containsRole checks if a role is in the claims roles list.
func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
