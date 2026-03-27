package handlers

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// OnboardingHandler exposes the 3-phase PKI+Blockchain onboarding flow.
type OnboardingHandler struct {
	mgr interfaces.OnboardingManager
}

// NewOnboardingHandler constructs an OnboardingHandler.
func NewOnboardingHandler(mgr interfaces.OnboardingManager) *OnboardingHandler {
	return &OnboardingHandler{mgr: mgr}
}

// SubmitCredentialRequest handles POST /api/v1/onboarding/credential-request.
// Phase 1: Commercial bank submits CSR + blockchain public key for credentialing.
func (h *OnboardingHandler) SubmitCredentialRequest(c *fiber.Ctx) error {
	var req struct {
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
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CsrPem == "" || req.BlockchainPubKeyHex == "" || req.Role == "" ||
		req.Username == "" || req.Email == "" || req.InstitutionName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "csr_pem, blockchain_pub_key_hex, role, username, email, and institution_name are required",
		})
	}

	result, err := h.mgr.SubmitCredentialRequest(c.UserContext(), interfaces.CredentialRequest{
		CsrPem:              req.CsrPem,
		BlockchainPubKeyHex: req.BlockchainPubKeyHex,
		InstitutionName:     req.InstitutionName,
		CNPJ:                req.CNPJ,
		BankCode:            req.BankCode,
		Country:             req.Country,
		Role:                req.Role,
		Email:               req.Email,
		Username:            req.Username,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"request_id":     result.RequestID,
		"user_id":        result.UserID,
		"wallet_address": result.WalletAddress,
		"status":         result.Status,
	})
}

// GetOnboardingStatus handles GET /api/v1/onboarding/status/:requestId.
// Phase 2.5: Commercial bank polls to discover KYC approval and PoP nonce.
func (h *OnboardingHandler) GetOnboardingStatus(c *fiber.Ctx) error {
	requestID := c.Params("requestId")
	if requestID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "requestId is required"})
	}

	result, err := h.mgr.GetOnboardingStatus(c.UserContext(), requestID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "request not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	resp := fiber.Map{
		"request_id":     result.RequestID,
		"user_id":        result.UserID,
		"status":         result.Status,
		"wallet_address": result.WalletAddress,
	}
	if result.PopNonce != "" {
		resp["pop_nonce"] = result.PopNonce
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// CompleteOnboarding handles POST /api/v1/onboarding/complete.
// Phase 3: Commercial bank signs PoP nonce to prove wallet ownership and complete onboarding.
func (h *OnboardingHandler) CompleteOnboarding(c *fiber.Ctx) error {
	var req struct {
		RequestID           string `json:"request_id"`
		UserID              string `json:"user_id"`
		PopSignatureHex     string `json:"pop_signature_hex"`
		BlockchainPubKeyHex string `json:"blockchain_pub_key_hex"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.UserID == "" || req.PopSignatureHex == "" || req.BlockchainPubKeyHex == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id, pop_signature_hex, and blockchain_pub_key_hex are required",
		})
	}

	result, err := h.mgr.CompleteOnboarding(c.UserContext(), interfaces.CompleteOnboardingRequest{
		RequestID:           req.RequestID,
		UserID:              req.UserID,
		PopSignatureHex:     req.PopSignatureHex,
		BlockchainPubKeyHex: req.BlockchainPubKeyHex,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user_id":        result.UserID,
		"wallet_address": result.WalletAddress,
		"cert_pem":       result.CertPEM,
		"tx_hash":        result.TxHash,
		"client_secret":  result.ClientSecret,
		"status":         result.Status,
	})
}

// GetMyOnboardingStatus handles GET /api/v1/onboarding/my-status?bank_code=<code>.
// Allows the Central Bank to serve a status query keyed on bank_code instead of
// request_id, so that commercial banks can recover their onboarding state after
// a page reload without needing to persist the original request_id.
func (h *OnboardingHandler) GetMyOnboardingStatus(c *fiber.Ctx) error {
	bankCode := c.Query("bank_code")
	if bankCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bank_code query parameter is required"})
	}

	result, err := h.mgr.GetOnboardingStatusByBankCode(c.UserContext(), bankCode)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no onboarding request found for this bank"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	resp := fiber.Map{
		"request_id": result.RequestID,
		"user_id":    result.UserID,
		"status":     result.Status,
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}
