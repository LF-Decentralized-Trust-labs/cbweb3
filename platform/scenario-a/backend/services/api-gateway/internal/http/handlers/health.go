// This file exposes a lightweight health check endpoint handler.
package handlers

import "github.com/gofiber/fiber/v2"

// Health returns a simple status payload used by health checks.
func Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "ok",
	})
}

