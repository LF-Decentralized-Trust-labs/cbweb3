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

// NOCPortalRoles are the roles that may read the NOC portal. The portal-read routes used to
// take RequireAuth alone: their comment said "any NOC role", but nothing checked for one, so
// every authenticated token — a treasury operator's included — was served the dashboard.
// Local stacks run with NOC_SKIP_AUTH=true and hide this, because the no-op Keycloak client
// hands back all three roles for any token.
var NOCPortalRoles = []string{"ROLE_NOC_ADMIN", "ROLE_NOC_OPERATOR", "ROLE_NOC_VIEWER"}

// RequireAnyRole rejects requests where the authenticated user holds none of roles. It is the
// multi-role form of RequireRole, for a group open to a family of roles rather than one.
func RequireAnyRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals(claimsKey).(keycloak.TokenClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
		}
		for _, want := range roles {
			for _, held := range claims.Roles {
				if held == want {
					return c.Next()
				}
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
