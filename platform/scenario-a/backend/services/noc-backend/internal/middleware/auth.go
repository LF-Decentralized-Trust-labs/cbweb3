// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/keycloak"
)

const claimsKey = "noc_claims"

// RequireAuth validates the Bearer JWT from Keycloak.
// On success it stores TokenClaims in Locals for downstream handlers.
func RequireAuth(kc keycloak.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing or invalid Authorization header"})
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := kc.ValidateToken(c.Context(), token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}
		c.Locals(claimsKey, claims)
		return c.Next()
	}
}

// RequireRole rejects requests where the authenticated user does not hold roleName.
func RequireRole(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals(claimsKey).(keycloak.TokenClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
		}
		for _, r := range claims.Roles {
			if r == role {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient role"})
	}
}

// GetClaims retrieves TokenClaims stored by RequireAuth.
func GetClaims(c *fiber.Ctx) (keycloak.TokenClaims, bool) {
	claims, ok := c.Locals(claimsKey).(keycloak.TokenClaims)
	return claims, ok
}
