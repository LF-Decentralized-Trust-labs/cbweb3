// Package middleware provides LP/CB role middleware for cooperative liquidity routes (T020 / FR-009).
package middleware

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

// RequireLiquidityProviderRole enforces that the caller holds either the central_bank
// or mlp Scenario B realm role. Used to gate all liquidity provision endpoints.
// Returns HTTP 403 with error_code NOT_AUTHORIZED_LP on failure (FR-009).
func RequireLiquidityProviderRole() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawClaims := c.Locals("claims")
		claims, ok := rawClaims.(domain.TokenClaims)
		if !ok || claims.Subject == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":      "authentication required",
				"error_code": "UNAUTHENTICATED",
			})
		}
		for _, role := range claims.Roles {
			if role == domain.RoleCentralBankScenarioB || role == domain.RoleMLPScenarioB {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":         "liquidity provider role required (central_bank or mlp)",
			"error_code":    "NOT_AUTHORIZED_LP",
			"required_role": "central_bank or mlp",
		})
	}
}
