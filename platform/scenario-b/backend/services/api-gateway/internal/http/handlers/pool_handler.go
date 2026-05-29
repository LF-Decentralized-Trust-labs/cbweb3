// Package handlers provides the AMM pool status HTTP handler for Scenario B (FR-028 / SC-013).
package handlers

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// PoolStatusServiceIface is the interface consumed by PoolHandler.
type PoolStatusServiceIface interface {
	GetPoolStatus(ctx context.Context, pair string) (*services.PoolStatusResponse, error)
}

// PoolHandler handles GET /api/v2/amm/pool/:pair/status.
type PoolHandler struct {
	svc PoolStatusServiceIface
}

// NewPoolHandler creates a PoolHandler.
func NewPoolHandler(svc PoolStatusServiceIface) *PoolHandler {
	return &PoolHandler{svc: svc}
}

// GetPoolStatus returns the cooperative pool state: reserves, pool_status, fee_rate_bps,
// total_lp_count, pending_commits[] and imbalance flag (T018 / FR-028 / FR-005 / FR-011).
func (h *PoolHandler) GetPoolStatus(c *fiber.Ctx) error {
	pair := c.Params("pair")
	if pair == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "pair path parameter is required",
			"error_code": "INVALID_REQUEST",
		})
	}

	resp, err := h.svc.GetPoolStatus(c.Context(), pair)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      err.Error(),
			"error_code": "POOL_STATUS_UNAVAILABLE",
		})
	}

	return c.JSON(resp)
}
