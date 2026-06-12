// Package handlers provides the SupervisorHandler for read-only audit log access (FR-SUP-001).
package handlers

import (
	"context"
	"log"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/gofiber/fiber/v2"
)

const maxAuditLimit = 100

// ComplianceAuditLister fetches immutable audit log entries from the compliance service.
type ComplianceAuditLister interface {
	GetAuditLogs(ctx context.Context, category, severity, fromDate, toDate string, page, limit int) ([]complianceadapter.AuditRecord, error)
}

// SupervisorHandler exposes supervisor-only endpoints (ROLE_SUPERVISOR required at the router level).
type SupervisorHandler struct {
	lister ComplianceAuditLister
}

// NewSupervisorHandler creates a SupervisorHandler.
func NewSupervisorHandler(lister ComplianceAuditLister) *SupervisorHandler {
	return &SupervisorHandler{lister: lister}
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
