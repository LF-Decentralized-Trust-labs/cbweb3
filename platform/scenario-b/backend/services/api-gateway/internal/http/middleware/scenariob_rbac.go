// SPDX-License-Identifier: Apache-2.0

// Package middleware provides Scenario B role-based access control middleware (FR-056).
package middleware

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

// RequireCentralBankRole enforces that the caller holds the Scenario B central_bank realm role.
// Rejects with HTTP 403 + error_code: INSUFFICIENT_ROLE on failure (FR-056 / SC-016).
func RequireCentralBankRole() fiber.Handler {
	return requireScenarioBRole(domain.RoleCentralBankScenarioB)
}

// RequireCommercialBankRole enforces that the caller holds the Scenario B commercial_bank realm role.
func RequireCommercialBankRole() fiber.Handler {
	return requireScenarioBRole(domain.RoleCommercialBankScenarioB)
}

// requireScenarioBRole inspects realm_access.roles inside the JWT claims
// and rejects if the required role is absent.
func requireScenarioBRole(requiredRole string) fiber.Handler {
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
			if role == requiredRole {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":         "insufficient role for this operation",
			"error_code":    "INSUFFICIENT_ROLE",
			"required_role": requiredRole,
		})
	}
}
