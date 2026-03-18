// This file handles all compliance endpoints: KYC status, credential issuance,
// proof verification, AML screening, participant provisioning, and account freeze.
package handlers

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// ComplianceHandler exposes compliance-related HTTP endpoints.
type ComplianceHandler struct {
	kyc    interfaces.KYCChecker
	kycMgr interfaces.KYCManager
}

// NewComplianceHandler builds a ComplianceHandler.
func NewComplianceHandler(kyc interfaces.KYCChecker) *ComplianceHandler {
	var kycMgr interfaces.KYCManager
	if mgr, ok := kyc.(interfaces.KYCManager); ok {
		kycMgr = mgr
	}
	return &ComplianceHandler{kyc: kyc, kycMgr: kycMgr}
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

// IssueKYCCredential is not yet available — credential issuance is handled internally by the identity service.
func (h *ComplianceHandler) IssueKYCCredential(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "KYC credential issuance is not yet available",
		"code":  "NOT_IMPLEMENTED",
	})
}

// VerifyKYCProof is not yet available — ZKP verification is handled internally by the identity service.
func (h *ComplianceHandler) VerifyKYCProof(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "KYC proof verification is not yet available",
		"code":  "NOT_IMPLEMENTED",
	})
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

// UnfreezeAccount restores APPROVED status for a frozen account (CENTRAL_BANK only).
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

	if err := h.kycMgr.ProvisionParticipant(c.UserContext(), body.Subject, domain.KYCApproved); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unfreeze failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"subject": body.Subject, "status": string(domain.KYCApproved)})
}
