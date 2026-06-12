package middleware

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

// RequireSupervisorRole enforces ROLE_SUPERVISOR on the request.
func RequireSupervisorRole() fiber.Handler {
	return RequireRole(domain.RoleSupervisor)
}

// RequireRole returns a middleware that enforces the caller has at least one
// of the specified roles (extracted from the token claims in Locals["claims"]).
// RequireBearerToken must run BEFORE this middleware to populate the claims.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawClaims := c.Locals("claims")
		claims, ok := rawClaims.(domain.TokenClaims)
		if !ok || claims.Subject == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "authentication required",
			})
		}
		for _, required := range roles {
			for _, actual := range claims.Roles {
				if actual == required {
					return c.Next()
				}
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":         "insufficient permissions",
			"required_role": roles,
		})
	}
}
