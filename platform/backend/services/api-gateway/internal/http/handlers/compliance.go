// This file handles all compliance endpoints: KYC status, credential issuance,
// proof verification, AML screening, participant provisioning, and account freeze.
package handlers

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// ComplianceHandler exposes compliance-related HTTP endpoints.
type ComplianceHandler struct {
	kyc        interfaces.KYCChecker
	kycMgr     interfaces.KYCManager
	onboarder  interfaces.ParticipantOnboarder
}

// NewComplianceHandler builds a ComplianceHandler.
func NewComplianceHandler(kyc interfaces.KYCChecker) *ComplianceHandler {
	var kycMgr interfaces.KYCManager
	if mgr, ok := kyc.(interfaces.KYCManager); ok {
		kycMgr = mgr
	}
	var onboarder interfaces.ParticipantOnboarder
	if ob, ok := kyc.(interfaces.ParticipantOnboarder); ok {
		onboarder = ob
	}
	return &ComplianceHandler{kyc: kyc, kycMgr: kycMgr, onboarder: onboarder}
}

// GetKYCStatus returns the KYC lifecycle status for the requested subject.
func (h *ComplianceHandler) GetKYCStatus(c *fiber.Ctx) error {
	subject := c.Params("subject")
	if subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}
	if h.kycMgr != nil {
		kycStatus, err := h.kycMgr.GetKYCStatus(c.UserContext(), subject)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get kyc status"})
		}
		return c.JSON(fiber.Map{"subject": subject, "status": kycStatus})
	}
	return c.JSON(fiber.Map{"subject": subject, "status": h.kyc.GetStatus(subject)})
}

// AMLScreen checks the KYC status as an AML/CFT gate (REQ-COM-007).
func (h *ComplianceHandler) AMLScreen(c *fiber.Ctx) error {
	var body struct {
		Subject string `json:"subject"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}

	var kycStatus domain.KYCStatus
	if h.kycMgr != nil {
		s, err := h.kycMgr.GetKYCStatus(c.UserContext(), body.Subject)
		if err != nil {
			s = domain.KYCStatus(h.kyc.GetStatus(body.Subject))
		}
		kycStatus = s
	} else {
		kycStatus = domain.KYCStatus(h.kyc.GetStatus(body.Subject))
	}

	sanctioned := kycStatus == domain.KYCRevoked || kycStatus == domain.KYCRejected || kycStatus == domain.KYCFrozen
	resp := fiber.Map{
		"subject":    body.Subject,
		"status":     kycStatus,
		"sanctioned": sanctioned,
	}
	if sanctioned {
		return c.Status(fiber.StatusForbidden).JSON(resp)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// ProvisionParticipant sets the KYC status for a participant (CENTRAL_BANK only).
func (h *ComplianceHandler) ProvisionParticipant(c *fiber.Ctx) error {
	var body struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Subject == "" || body.Status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject and status are required"})
	}
	if h.kycMgr == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "kyc manager not available"})
	}

	if err := h.kycMgr.ProvisionParticipant(c.UserContext(), body.Subject, domain.KYCStatus(body.Status)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "provisioning failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"subject": body.Subject, "status": body.Status})
}

// FreezeAccount sets FROZEN status for an account (CENTRAL_BANK only, REQ-COM-003).
func (h *ComplianceHandler) FreezeAccount(c *fiber.Ctx) error {
	var body struct {
		Subject string `json:"subject"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}
	if h.kycMgr == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "kyc manager not available"})
	}

	if err := h.kycMgr.ProvisionParticipant(c.UserContext(), body.Subject, domain.KYCFrozen); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "freeze failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"subject": body.Subject, "status": string(domain.KYCFrozen)})
}

// RegisterParticipant is called by the Central Bank to onboard a new
// Commercial Bank or Treasury user (POST /compliance/register).
// The CB must be authenticated (Bearer token validated upstream).
func (h *ComplianceHandler) RegisterParticipant(c *fiber.Ctx) error {
	if h.onboarder == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "participant onboarding not available",
			"code":  "NOT_IMPLEMENTED",
		})
	}

	var body struct {
		Username        string `json:"username"`
		Email           string `json:"email"`
		Role            string `json:"role"`
		InstitutionName string `json:"institution_name"`
		Country         string `json:"country"`
		BankCode        string `json:"bank_code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Username == "" || body.Email == "" || body.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username, email, and role are required",
		})
	}
	if !domain.IsAdminRole(body.Role) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid role: must be one of ROLE_COMMERCIAL_BANK, ROLE_TREASURY, ROLE_NOC, ROLE_SUPERVISOR, ROLE_GOVERNANCE_OFFICER",
		})
	}

	result, err := h.onboarder.OnboardParticipant(c.UserContext(), interfaces.OnboardParticipantRequest{
		Username:        body.Username,
		Email:           body.Email,
		Role:            body.Role,
		InstitutionName: body.InstitutionName,
		Country:         body.Country,
		BankCode:        body.BankCode,
	})
	if err != nil {
		if isConflictError(err) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "onboarding failed"})
	}

	resp := fiber.Map{
		"userId": result.UserID,
		"role":   body.Role,
		"status": "PENDING",
	}
	if result.WalletAddress != "" {
		resp["walletAddress"] = result.WalletAddress
	}
	if result.TxHash != "" {
		resp["txHash"] = result.TxHash
	}
	if result.ClientSecret != "" {
		// Exposed only once. The caller must store this value securely.
		resp["clientSecret"] = result.ClientSecret
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// isConflictError returns true if the error indicates a duplicate resource.
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"already exists", "conflict", "duplicate"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// UnfreezeAccount restores ACTIVE status for a frozen account (CENTRAL_BANK only).
func (h *ComplianceHandler) UnfreezeAccount(c *fiber.Ctx) error {
	var body struct {
		Subject string `json:"subject"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}
	if h.kycMgr == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "kyc manager not available"})
	}

	if err := h.kycMgr.ProvisionParticipant(c.UserContext(), body.Subject, domain.KYCActive); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unfreeze failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"subject": body.Subject, "status": string(domain.KYCActive)})
}
