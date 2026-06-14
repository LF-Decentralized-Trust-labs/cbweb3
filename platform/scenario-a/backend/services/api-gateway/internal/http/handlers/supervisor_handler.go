// Package handlers provides the SupervisorHandler for read-only audit log access (FR-SUP-001)
// and regulatory transaction decryption (FR-SUP-002).
package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	paladinadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/paladin"
	"github.com/gofiber/fiber/v2"
)

const maxAuditLimit = 100

// ComplianceAuditLister fetches and writes compliance audit log entries.
type ComplianceAuditLister interface {
	GetAuditLogs(ctx context.Context, category, severity, fromDate, toDate string, page, limit int) ([]complianceadapter.AuditRecord, error)
	CreateAuditLog(ctx context.Context, entry complianceadapter.AuditEntry) error
}

// decryptGate checks whether an approved (QUORUM_REACHED) disclosure exists for a tx reference.
// An error means no approved disclosure was found.
type decryptGate interface {
	CheckDisclosureQuorum(ctx context.Context, txRef string) error
}

// SupervisorHandler exposes supervisor-only endpoints (ROLE_SUPERVISOR required at the router level).
type SupervisorHandler struct {
	lister  ComplianceAuditLister
	paladin *paladinadapter.Client // nil if PALADIN_URL not configured
	gate    decryptGate            // nil if oversight DB not available
}

// NewSupervisorHandler creates a SupervisorHandler.
func NewSupervisorHandler(lister ComplianceAuditLister) *SupervisorHandler {
	return &SupervisorHandler{lister: lister}
}

// SetDecryptDeps wires the Paladin client and disclosure gate used by DecryptTransaction.
func (h *SupervisorHandler) SetDecryptDeps(paladin *paladinadapter.Client, gate decryptGate) {
	h.paladin = paladin
	h.gate = gate
}

// GetAuditLogs handles GET /api/v1/compliance/audit/logs.
//
// Query params: category, severity, from_date, to_date, page (default 1), limit (default 20, max 100).
func (h *SupervisorHandler) GetAuditLogs(c *fiber.Ctx) error {
	category := c.Query("category")
	severity := c.Query("severity")
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > maxAuditLimit {
		limit = maxAuditLimit
	}

	logs, err := h.lister.GetAuditLogs(c.UserContext(), category, severity, fromDate, toDate, page, limit)
	if err != nil {
		log.Printf("[supervisor] GetAuditLogs failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	if logs == nil {
		logs = []complianceadapter.AuditRecord{}
	}

	return c.JSON(fiber.Map{
		"logs":  logs,
		"page":  page,
		"limit": limit,
	})
}

// DecryptTransaction handles POST /api/v1/compliance/decrypt-transaction.
//
// Gate: requires a QUORUM_REACHED disclosure request for the given tx_hash.
// Calls Paladin ptx_getStateReceipt to read the private Zeto state,
// then writes an immutable audit log entry recording the access.
func (h *SupervisorHandler) DecryptTransaction(c *fiber.Ctx) error {
	if h.paladin == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "transaction decryption not available (PALADIN_URL not configured)",
		})
	}
	if h.gate == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "transaction decryption not available (oversight DB not configured)",
		})
	}

	var req struct {
		TxHash  string `json:"tx_hash"`
		ViewKey string `json:"view_key"`
		Reason  string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.TxHash == "" || req.ViewKey == "" || req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tx_hash, view_key, and reason are required"})
	}

	// Quorum gate: only proceed if a QUORUM_REACHED disclosure exists for this tx.
	if err := h.gate.CheckDisclosureQuorum(c.UserContext(), req.TxHash); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fmt.Sprintf("disclosure quorum not reached for tx %s: open a disclosure request and collect 2 signatures first", req.TxHash),
		})
	}

	// Fetch decrypted state from Paladin.
	dec, err := h.paladin.GetDecryptedTx(c.UserContext(), req.TxHash)
	if err != nil {
		log.Printf("[supervisor] decrypt tx %s failed: %v", req.TxHash, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "failed to decrypt transaction: " + err.Error(),
		})
	}

	// Immutable audit trail — best effort; log but don't block the response.
	actor := c.Locals("subject")
	actorStr := ""
	if actor != nil {
		actorStr = fmt.Sprintf("%v", actor)
	}
	auditErr := h.lister.CreateAuditLog(c.UserContext(), complianceadapter.AuditEntry{
		ActorSubject:  actorStr,
		ActorAddress:  c.IP(),
		ActionType:    "DECRYPT_TRANSACTION",
		TargetSubject: req.TxHash,
		Result:        "SUCCESS",
		Category:      "COMPLIANCE",
		Severity:      "HIGH",
		Details:       fmt.Sprintf(`{"reason":%q,"view_key_hint":%q}`, req.Reason, req.ViewKey[:min(8, len(req.ViewKey))]),
	})
	if auditErr != nil {
		log.Printf("[supervisor] audit log write failed (non-fatal): %v", auditErr)
	}

	return c.JSON(fiber.Map{
		"tx_hash":      req.TxHash,
		"amount":       dec.Amount,
		"currency":     dec.Currency,
		"sender":       dec.Sender,
		"receiver":     dec.Receiver,
		"decrypted_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
