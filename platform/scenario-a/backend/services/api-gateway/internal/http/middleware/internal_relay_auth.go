// SPDX-License-Identifier: Apache-2.0

// Package middleware provides Fiber middleware for the API Gateway.
package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
)

// RequireRelayAuth enforces X-Relay-Auth header validation for internal service-to-service
// endpoints used by the Cacti relay. The expected secret is compared in constant time to
// prevent timing-based enumeration attacks.
//
// Usage:
//
//	app.Group("/internal/v1/...", middleware.RequireRelayAuth(os.Getenv("INTERNAL_RELAY_AUTH_SECRET")))
func RequireRelayAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if secret == "" {
			// Secret not configured — fail closed to prevent open access in production.
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "relay auth not configured on server",
			})
		}
		provided := c.Get("X-Relay-Auth")
		if provided == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "X-Relay-Auth header is required",
			})
		}
		// Constant-time comparison prevents timing attacks.
		if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid relay auth secret",
			})
		}
		return c.Next()
	}
}
