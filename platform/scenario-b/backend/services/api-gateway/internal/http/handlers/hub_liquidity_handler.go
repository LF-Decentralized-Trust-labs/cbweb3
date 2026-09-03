// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// GetHubLiquidityConfig exposes sovereign Hub AMM addresses for commercial bank gateways (T054).
// Only CB gateways with SOVEREIGN_AMM_ADDRESS configured return 200.
func GetHubLiquidityConfig(c *fiber.Ctx) error {
	cfg := services.HubLiquidityConfigFromEnv()
	if cfg == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":      "sovereign hub liquidity is not configured on this gateway",
			"error_code": "HUB_LIQUIDITY_NOT_CONFIGURED",
		})
	}
	return c.JSON(cfg)
}
