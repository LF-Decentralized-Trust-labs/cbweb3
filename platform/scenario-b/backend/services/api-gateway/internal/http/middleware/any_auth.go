// Package middleware provides a dual-auth middleware that accepts either an
// HttpOnly cookie (browser flows) or an Authorization: Bearer header (M2M/CB
// sovereign flows such as spec-007 client_credentials tokens).
package middleware

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// RequireAnyAuth accepts authentication via:
//  1. HttpOnly cookie "access_token" (browser / Scenario B commercial-bank flows), or
//  2. Authorization: Bearer <token> header (M2M / CB sovereign flows — spec-007).
//
// Exactly one method must succeed; both absent → 401, invalid token → 401.
// Validated claims are stored in Locals["claims"] for downstream role middleware.
func RequireAnyAuth(authProvider interfaces.IAuthProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Try Bearer header first (M2M preference).
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				claims, err := authProvider.Validate(c.Context(), parts[1])
				if err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error":      "invalid or expired Bearer token",
						"error_code": "UNAUTHENTICATED",
					})
				}
				c.Locals("claims", claims)
				return c.Next()
			}
		}

		// 2. Fall back to cookie.
		cookie := c.Cookies("access_token")
		if cookie == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":      "authentication required: provide access_token cookie or Authorization: Bearer header",
				"error_code": "UNAUTHENTICATED",
			})
		}
		claims, err := authProvider.Validate(c.Context(), cookie)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":      "invalid access_token cookie",
				"error_code": "UNAUTHENTICATED",
			})
		}
		c.Locals("claims", claims)
		return c.Next()
	}
}
