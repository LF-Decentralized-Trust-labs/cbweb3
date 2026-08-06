// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// asVerifiedCaller stamps the identity the relay-auth middleware writes after verifying a signature.
// Tests of the delegated endpoints need it because those endpoints act ON BEHALF OF the caller, so a
// request that carries no verified caller is refused before anything else is looked at.
func asVerifiedCaller(id string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals(middleware.RelayCallerLocal, id)
		return c.Next()
	}
}
