// Package handlers provides the SupervisorHandler for read-only audit log access (FR-SUP-001)
// and regulatory transaction decryption (FR-SUP-002).
package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	besuscanner "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/besu"
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

// htlcLookup resolves an HTLC contract ID to its on-chain scan result.
type htlcLookup interface {
	GetByContractID(ctx context.Context, contractID string) (*besuscanner.HTLCScanResult, error)
}

// SupervisorHandler exposes supervisor-only endpoints (ROLE_SUPERVISOR required at the router level).
type SupervisorHandler struct {
	lister  ComplianceAuditLister
	paladin *paladinadapter.Client // nil if PALADIN_URL not configured
	gate    decryptGate            // nil if oversight DB not available
	scanner htlcLookup             // nil if Besu scanner not configured
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

// SetHTLCScanner wires the on-chain HTLC scanner used by DecryptTransaction to resolve
// an HTLC contract ID to the Paladin transaction UUID stored in its zeto_lock_ref field.
func (h *SupervisorHandler) SetHTLCScanner(s htlcLookup) {
	h.scanner = s
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
// Accepts the HTLC contract ID (hex, as shown in the supervisor dashboard) as tx_hash.
// Gate: requires a QUORUM_REACHED disclosure request for that contract ID.
// Resolves the on-chain zeto_lock_ref to the Paladin tx UUID, then calls ptx_getStateReceipt
// to read the private Zeto state and writes an immutable audit log entry.
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
	if h.scanner == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "transaction decryption not available (HTLC scanner not configured)",
		})
	}

	var req struct {
		TxHash  string `json:"tx_hash"`  // HTLC contract ID (0x...), same as shown in the dashboard
		ViewKey string `json:"view_key"`
		Reason  string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.TxHash == "" || req.ViewKey == "" || req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tx_hash, view_key, and reason are required"})
	}

	// Quorum gate: the disclosure tx_ref must match this HTLC contract ID.
	if err := h.gate.CheckDisclosureQuorum(c.UserContext(), req.TxHash); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fmt.Sprintf("disclosure quorum not reached for tx %s: open a disclosure request and collect 2 signatures first", req.TxHash),
		})
	}

	// Look up the HTLC on-chain to extract the Paladin UUID from zeto_lock_ref.
	htlc, err := h.scanner.GetByContractID(c.UserContext(), req.TxHash)
	if err != nil {
		log.Printf("[supervisor] HTLC on-chain lookup failed for %s: %v", req.TxHash, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "HTLC not found on-chain: " + err.Error(),
		})
	}

	paladinUUID, err := paladinadapter.UUIDFromBytes32(htlc.ZetoLockRef)
	if err != nil {
		log.Printf("[supervisor] cannot extract Paladin UUID from zeto_lock_ref %s: %v", htlc.ZetoLockRef, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "cannot resolve Paladin transaction reference — this HTLC may have been created before the supervisor decrypt feature was deployed",
		})
	}

	// Fetch decrypted state from Paladin.
	dec, err := h.paladin.GetDecryptedTx(c.UserContext(), paladinUUID)
	if err != nil {
		log.Printf("[supervisor] decrypt paladin UUID %s (HTLC %s) failed: %v", paladinUUID, req.TxHash, err)
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
		Details:       fmt.Sprintf(`{"reason":%q,"view_key_hint":%q,"htlc":%q}`, req.Reason, req.ViewKey[:min(8, len(req.ViewKey))], req.TxHash),
	})
	if auditErr != nil {
		log.Printf("[supervisor] audit log write failed (non-fatal): %v", auditErr)
	}

	// Sender: use the on-chain msg.sender (EVM address of the initiating operator).
	// Receiver: the HTLC contract requires onlyVerified(receiver) against the local
	// IdentityRegistry. Cross-spoke counterparties are not registered locally, so the
	// payment-orchestrator falls back to its own address as the receiver parameter.
	// When on-chain sender == receiver it signals this fallback — suppress the duplicate
	// and fall back to the Zeto private state receiver (empty for locked UTXOs by design).
	sender := htlc.Sender
	if sender == "" {
		sender = dec.Sender
	}
	receiver := htlc.Receiver
	if receiver == "" || receiver == htlc.Sender {
		receiver = dec.Receiver // empty for cross-spoke locks; shown as "—" in the UI
	}

	return c.JSON(fiber.Map{
		"tx_hash":      req.TxHash, // HTLC contract ID — consistent with the supervisor dashboard
		"amount":       dec.Amount,
		"currency":     dec.Currency,
		"sender":       sender,
		"receiver":     receiver,
		"decrypted_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
