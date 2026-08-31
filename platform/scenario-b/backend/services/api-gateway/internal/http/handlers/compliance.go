// SPDX-License-Identifier: Apache-2.0

// This file handles all compliance endpoints: KYC status, credential issuance,
// proof verification, AML screening, participant provisioning, and account freeze.
package handlers

import (
	"context"
	"log/slog"
	"strings"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// ComplianceParticipantLister defines the participant listing capability
// used by the compliance handler. Satisfied by the compliance GRPCAdapter.
type ComplianceParticipantLister interface {
	ListParticipants(ctx context.Context, statusFilter, search string) ([]complianceadapter.Participant, error)
}

// ComplianceHandler exposes compliance-related HTTP endpoints.
type ComplianceHandler struct {
	kyc       interfaces.KYCChecker
	kycMgr    interfaces.KYCManager
	onboarder interfaces.ParticipantOnboarder
	lister    ComplianceParticipantLister
}

// NewComplianceHandler builds a ComplianceHandler.
func NewComplianceHandler(kyc interfaces.KYCChecker, lister ComplianceParticipantLister) *ComplianceHandler {
	var kycMgr interfaces.KYCManager
	if mgr, ok := kyc.(interfaces.KYCManager); ok {
		kycMgr = mgr
	}
	var onboarder interfaces.ParticipantOnboarder
	if ob, ok := kyc.(interfaces.ParticipantOnboarder); ok {
		onboarder = ob
	}
	return &ComplianceHandler{kyc: kyc, kycMgr: kycMgr, onboarder: onboarder, lister: lister}
}

// ListParticipants handles GET /api/v1/compliance/participants.
// Returns registered participants, optionally filtered by status or search term.
func (h *ComplianceHandler) ListParticipants(c *fiber.Ctx) error {
	statusFilter := c.Query("status")
	search := c.Query("search")

	participants, err := h.lister.ListParticipants(c.UserContext(), statusFilter, search)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"participants": participants})
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
			// Fail-closed parity with AMLScreen (R2-H-7): a compliance-dependency
			// failure is a 503, not a 500 — the subject's status is unknown, not a
			// bug in this gateway. Log it (silent swallowing is prohibited).
			slog.Error("get kyc status failed: compliance dependency unavailable",
				"service", "api-gateway",
				"event", "get_kyc_status",
				"correlation_id", middleware.CorrelationIDFromContext(c.UserContext()),
				"subject", subject,
				"error", err.Error(),
			)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "compliance dependency unavailable",
				"code":  "COMPLIANCE_UNAVAILABLE",
			})
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
			// REQ-COM-007 fail-closed (finding R2-H-7): a failure of the
			// compliance dependency must BLOCK the transaction. Never fall
			// back to the permissive in-memory checker, which would let an
			// unscreened subject through and return 200. Log the dependency
			// error (silent swallowing is prohibited) and fail closed with
			// 503 Service Unavailable.
			slog.Error("aml screen failed closed: compliance dependency unavailable",
				"service", "api-gateway",
				"event", "aml_screen",
				"correlation_id", middleware.CorrelationIDFromContext(c.UserContext()),
				"subject", body.Subject,
				"error", err.Error(),
			)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"subject":    body.Subject,
				"sanctioned": true,
				"error":      "compliance dependency unavailable; AML screening cannot be completed",
				"code":       "COMPLIANCE_UNAVAILABLE",
			})
		}
		kycStatus = s
	} else {
		kycStatus = domain.KYCStatus(h.kyc.GetStatus(body.Subject))
	}

	// REQ-COM-007 (finding R2-H-7): AML clearance is an ALLOWLIST, not a
	// denylist. Only an explicitly ACTIVE/APPROVED subject may proceed. Every
	// other status is blocked — including PENDING, which is what the identity
	// service returns for a subject it cannot find (unregistered or never
	// screened). A denylist that cleared everything except REVOKED/REJECTED/
	// FROZEN would let that unscreened PENDING subject through with a 200.
	cleared := kycStatus == domain.KYCActive || kycStatus == domain.KYCApproved
	resp := fiber.Map{
		"subject":    body.Subject,
		"status":     kycStatus,
		"sanctioned": !cleared,
	}
	if !cleared {
		resp["code"] = amlBlockCode(kycStatus)
		return c.Status(fiber.StatusForbidden).JSON(resp)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// amlBlockCode classifies why an AML screen blocked a subject, distinguishing a
// positive sanctions/enforcement match from a subject that simply has not
// cleared screening yet (PENDING, empty, or unknown status).
func amlBlockCode(s domain.KYCStatus) string {
	switch s {
	case domain.KYCFrozen, domain.KYCRevoked, domain.KYCRejected:
		return "SANCTIONED"
	default:
		return "NOT_CLEARED"
	}
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
