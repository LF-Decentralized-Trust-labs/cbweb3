// This file defines bearer-token middleware and injects validated claims into context.
package middleware

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// RequireBearerToken validates Authorization bearer tokens and stores claims in context.
func RequireBearerToken(authProvider interfaces.IAuthProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
		}

		token := strings.TrimSpace(authHeader[7:])
		claims, err := authProvider.Validate(c.Context(), token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}

		c.Locals("claims", claims)
		return c.Next()
	}
}

