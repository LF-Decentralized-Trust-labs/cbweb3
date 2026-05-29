// This file defines cookie-auth middleware and injects validated claims into context.
package middleware

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// RequireCookieAuth validates the access_token HttpOnly cookie and stores claims in context.
func RequireCookieAuth(authProvider interfaces.IAuthProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Cookies("access_token")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing access_token cookie"})
		}

		claims, err := authProvider.Validate(c.Context(), token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}

		c.Locals("claims", claims)
		return c.Next()
	}
}
