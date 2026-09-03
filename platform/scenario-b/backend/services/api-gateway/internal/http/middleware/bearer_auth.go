// SPDX-License-Identifier: Apache-2.0

// Package middleware provides bearer-token authentication middleware for Scenario B API v2.
package middleware

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// RequireBearerAuth validates the Authorization: Bearer <token> header and stores claims in context.
// This middleware is used by Scenario B API v2 endpoints.
func RequireBearerAuth(authProvider interfaces.IAuthProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":      "missing Authorization header",
				"error_code": "UNAUTHENTICATED",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":      "invalid Authorization header format, expected: Bearer <token>",
				"error_code": "UNAUTHENTICATED",
			})
		}

		token := parts[1]
		claims, err := authProvider.Validate(c.Context(), token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":      "invalid or expired token",
				"error_code": "UNAUTHENTICATED",
			})
		}

		c.Locals("claims", claims)
		return c.Next()
	}
}
