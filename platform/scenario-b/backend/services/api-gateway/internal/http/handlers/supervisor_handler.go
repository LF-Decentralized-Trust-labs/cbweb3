// SPDX-License-Identifier: Apache-2.0

// Package handlers provides the SupervisorHandler for read-only audit log access (FR-SUP-001/FR-SUP-002).
package handlers

import (
	"context"
	"errors"
	"log"
	"time"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/gofiber/fiber/v2"
)

const (
	maxAuditLimit = 100
	// defaultAuditLimit is the page size used when the client asks for none or asks for
	// something unusable. It matches the compliance repository's own default.
	defaultAuditLimit = 50
)

// ComplianceAuditLister fetches immutable audit log entries from the compliance service.
type ComplianceAuditLister interface {
	GetAuditLogs(ctx context.Context, category, severity, fromDate, toDate string, page, limit int) ([]complianceadapter.AuditRecord, error)
}

// ZKPointerLookup holds the fields returned by a ZK-pointer record query.
type ZKPointerLookup struct {
	PointerID      string
	CommitmentHash string
	State          string
	ExpiresAt      *time.Time
}

// ErrZKPointerNotFound is the sentinel returned when no record matches the query.
var ErrZKPointerNotFound = errors.New("no ZK-Pointer record found")

// ZKPointerVerifier performs a ZK-pointer record lookup for supervisor verification (FR-SUP-002).
type ZKPointerVerifier interface {
	GetZKPointerRecord(ctx context.Context, bankID, commitmentHash string) (*ZKPointerLookup, error)
}

// SupervisorHandler exposes supervisor-only endpoints (ROLE_SUPERVISOR required at the router level).
type SupervisorHandler struct {
	lister     ComplianceAuditLister
	zkVerifier ZKPointerVerifier // optional; nil disables /zk-pointer/verify
}

// NewSupervisorHandler creates a SupervisorHandler. zkVerifier may be nil when the ZK gate is unavailable.
func NewSupervisorHandler(lister ComplianceAuditLister, zkVerifier ZKPointerVerifier) *SupervisorHandler {
	return &SupervisorHandler{lister: lister, zkVerifier: zkVerifier}
}

// VerifyZKPointer handles GET /api/v1/compliance/zk-pointer/verify.
//
// Query params: bank_id (required), commitment_hash (required).
func (h *SupervisorHandler) VerifyZKPointer(c *fiber.Ctx) error {
	if h.zkVerifier == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "ZK pointer verification not available"})
	}

	bankID := c.Query("bank_id")
	commitmentHash := c.Query("commitment_hash")
	if bankID == "" || commitmentHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bank_id and commitment_hash are required"})
	}

	record, err := h.zkVerifier.GetZKPointerRecord(c.UserContext(), bankID, commitmentHash)
	if err != nil {
		if errors.Is(err, ErrZKPointerNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no valid ZK-Pointer found for this bank and commitment"})
		}
		log.Printf("[supervisor] VerifyZKPointer failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	var expiresAt any
	if record.ExpiresAt != nil {
		expiresAt = record.ExpiresAt.UTC().Format(time.RFC3339)
	}

	return c.JSON(fiber.Map{
		"bank_id":         bankID,
		"pointer_id":      record.PointerID,
		"commitment_hash": record.CommitmentHash,
		"state":           record.State,
		"expires_at":      expiresAt,
	})
}

// GetAuditLogs handles GET /api/v1/compliance/audit/logs.
//
// Query params: category, severity, from_date, to_date, page (default 1),
// limit (default defaultAuditLimit, max maxAuditLimit).
//
// Page and limit go through the same auditPageNumber/auditPageSize helpers the governance route
// uses. Two routes reading one audit log must agree on what a legal page is, and before this they
// did not: this one defaulted to 20 where governance defaulted to 50, and it mapped `limit=0` to
// the MAXIMUM page rather than the default, so asking for nothing returned the most (R2-M-14).
func (h *SupervisorHandler) GetAuditLogs(c *fiber.Ctx) error {
	if h.lister == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "audit log service not available"})
	}

	category := c.Query("category")
	severity := c.Query("severity")
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")
	page := auditPageNumber(c.Query("page"))
	limit := auditPageSize(c.Query("limit"))

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
