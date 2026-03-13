// This file handles login and wallet binding HTTP endpoints.
package handlers

import (
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/auth"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// AuthHandler implements authentication and wallet-binding endpoints.
type AuthHandler struct {
	authProvider    interfaces.IAuthProvider
	identityManager interfaces.IIdentityManager
	kycChecker      interfaces.KYCChecker
}

type loginRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type walletBindRequest struct {
	WalletAddress string `json:"walletAddress"`
	Signature     string `json:"signature"`
}

// NewAuthHandler builds an AuthHandler with its required dependencies.
func NewAuthHandler(
	authProvider interfaces.IAuthProvider,
	identityManager interfaces.IIdentityManager,
	kycChecker interfaces.KYCChecker,
) *AuthHandler {
	return &AuthHandler{
		authProvider:    authProvider,
		identityManager: identityManager,
		kycChecker:      kycChecker,
	}
}

// Login authenticates a client and returns an access token.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.ClientID == "" || req.ClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "clientId and clientSecret are required"})
	}

	token, err := h.authProvider.Authenticate(c.Context(), req.ClientID, req.ClientSecret)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"accessToken": token.AccessToken,
		"expiresIn":   token.ExpiresIn,
		"tokenType":   token.TokenType,
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

	if h.kycChecker.GetStatus(claims.Subject) == domain.KYCRejected {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "kyc status rejected"})
	}

	if err := auth.VerifyWalletSignature(claims.Subject, req.WalletAddress, req.Signature); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid signature"})
	}

	binding, err := h.identityManager.BindWallet(claims.Subject, req.WalletAddress)
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

