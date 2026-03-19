// Package handlers contains the governance portal handlers for the Central Bank role.
// These handlers expose participant management, credential issuance, account control,
// circuit breaker, global parameters, and audit log endpoints.
package handlers

import (
	"context"
	"strconv"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

// GovernanceCompliance defines compliance operations used by governance HTTP handlers.
type GovernanceCompliance interface {
	RegisterParticipant(ctx context.Context, p complianceadapter.Participant) error
	ListParticipants(ctx context.Context, statusFilter, search string) ([]complianceadapter.Participant, error)
	IssueParticipantCertificate(ctx context.Context, userID, role, institutionName, cnpj string) (complianceadapter.IssuedCertificate, error)
	ApproveKYC(ctx context.Context, subject, actorSubject, reason string) (complianceadapter.ApproveKYCResult, error)
	ManageParticipantStatus(ctx context.Context, subject, statusVal, reason string) error
	GetAuditLogs(ctx context.Context, category, severity, fromDate, toDate string, page, limit int) ([]complianceadapter.AuditRecord, error)
	GetCircuitBreakerStatus(ctx context.Context) (complianceadapter.CircuitBreakerStatus, error)
	ToggleCircuitBreaker(ctx context.Context, pause bool, reason string) (bool, error)
	GetSystemParameters(ctx context.Context) (complianceadapter.SystemParameters, error)
	UpdateSystemParameters(ctx context.Context, params complianceadapter.SystemParameters, reason, actorSubject string) error
}

// GovernanceHandler exposes governance portal operations for ROLE_GOVERNANCE users.
type GovernanceHandler struct {
	compliance GovernanceCompliance
}

// NewGovernanceHandler constructs a GovernanceHandler.
func NewGovernanceHandler(c GovernanceCompliance) *GovernanceHandler {
	return &GovernanceHandler{compliance: c}
}

// RegisterParticipant handles POST /api/v1/governance/participants.
// The Central Bank registers a new participant (Commercial Bank, Treasury, NOC, etc.).
func (h *GovernanceHandler) RegisterParticipant(c *fiber.Ctx) error {
	var p complianceadapter.Participant
	if err := c.BodyParser(&p); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if p.UserID == "" || p.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id and role are required"})
	}
	if p.Status == "" {
		p.Status = "PENDING"
	}
	if err := h.compliance.RegisterParticipant(c.UserContext(), p); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user_id": p.UserID, "status": p.Status})
}

// GetRegistry handles GET /api/v1/governance/registry.
// Returns all registered participants, optionally filtered by status or search.
func (h *GovernanceHandler) GetRegistry(c *fiber.Ctx) error {
	statusFilter := c.Query("status")
	search := c.Query("search")

	participants, err := h.compliance.ListParticipants(c.UserContext(), statusFilter, search)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"participants": participants})
}

// IssueCredential handles POST /api/v1/governance/registry/credential.
// Issues an X.509 certificate for a registered participant.
func (h *GovernanceHandler) IssueCredential(c *fiber.Ctx) error {
	var req struct {
		UserID          string `json:"user_id"`
		Role            string `json:"role"`
		InstitutionName string `json:"institution_name"`
		CNPJ            string `json:"cnpj"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.UserID == "" || req.Role == "" || req.InstitutionName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id, role, and institution_name are required"})
	}

	issued, err := h.compliance.IssueParticipantCertificate(c.UserContext(), req.UserID, req.Role, req.InstitutionName, req.CNPJ)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user_id":     req.UserID,
		"cert_pem":    issued.CertPEM,
		"priv_key_pem": issued.PrivKeyPEM,
		"expires_at":  issued.ExpiresAt,
	})
}

// ApproveKYC handles POST /api/v1/compliance/approve-kyc.
// Activates the participant in PostgreSQL and calls setParticipant on the Besu contract.
// Allowed roles: ROLE_GOVERNANCE, ROLE_SUPERVISOR.
func (h *GovernanceHandler) ApproveKYC(c *fiber.Ctx) error {
	var req struct {
		Subject string `json:"subject"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}

	result, err := h.compliance.ApproveKYC(c.UserContext(), req.Subject, actorFromClaims(c), req.Reason)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	resp := fiber.Map{
		"subject": result.Subject,
		"status":  result.Status,
	}
	if result.TxHash != "" {
		resp["tx_hash"] = result.TxHash
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// actorFromClaims extracts the subject from the JWT claims stored in Locals.
func actorFromClaims(c *fiber.Ctx) string {
	if claims, ok := c.Locals("claims").(domain.TokenClaims); ok {
		return claims.Subject
	}
	return ""
}

// GetAccounts handles GET /api/v1/governance/accounts.
// Lists participants with account status information.
func (h *GovernanceHandler) GetAccounts(c *fiber.Ctx) error {
	participants, err := h.compliance.ListParticipants(c.UserContext(), "", "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"accounts": participants})
}

// FreezeAccount handles POST /api/v1/governance/accounts/freeze.
func (h *GovernanceHandler) FreezeAccount(c *fiber.Ctx) error {
	var req struct {
		Subject string `json:"subject"`
		Reason  string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}
	if err := h.compliance.ManageParticipantStatus(c.UserContext(), req.Subject, "FROZEN", req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"subject": req.Subject, "status": "FROZEN"})
}

// UnfreezeAccount handles POST /api/v1/governance/accounts/unfreeze.
func (h *GovernanceHandler) UnfreezeAccount(c *fiber.Ctx) error {
	var req struct {
		Subject string `json:"subject"`
		Reason  string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}
	if err := h.compliance.ManageParticipantStatus(c.UserContext(), req.Subject, "ACTIVE", req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"subject": req.Subject, "status": "ACTIVE"})
}

// GetCircuitBreakerStatus handles GET /api/v1/governance/circuit-breaker/status.
func (h *GovernanceHandler) GetCircuitBreakerStatus(c *fiber.Ctx) error {
	st, err := h.compliance.GetCircuitBreakerStatus(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(st)
}

// ToggleCircuitBreaker handles POST /api/v1/governance/circuit-breaker/toggle.
func (h *GovernanceHandler) ToggleCircuitBreaker(c *fiber.Ctx) error {
	var req struct {
		Pause  bool   `json:"pause"`
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "reason is required"})
	}
	isPaused, err := h.compliance.ToggleCircuitBreaker(c.UserContext(), req.Pause, req.Reason)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"is_paused": isPaused})
}

// GetParameters handles GET /api/v1/governance/parameters.
func (h *GovernanceHandler) GetParameters(c *fiber.Ctx) error {
	params, err := h.compliance.GetSystemParameters(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(params)
}

// UpdateParameters handles PUT /api/v1/governance/parameters.
func (h *GovernanceHandler) UpdateParameters(c *fiber.Ctx) error {
	var req struct {
		complianceadapter.SystemParameters
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "reason is required"})
	}

	actorSubject := actorFromClaims(c)
	if err := h.compliance.UpdateSystemParameters(c.UserContext(), req.SystemParameters, req.Reason, actorSubject); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

// GetAuditLogs handles GET /api/v1/governance/audit/logs.
func (h *GovernanceHandler) GetAuditLogs(c *fiber.Ctx) error {
	category := c.Query("category")
	severity := c.Query("severity")
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	logs, err := h.compliance.GetAuditLogs(c.UserContext(), category, severity, fromDate, toDate, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"logs": logs})
}
