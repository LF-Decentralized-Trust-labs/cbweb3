// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// RequireRequesterScope refuses a listing that names no requester.
//
// The payment listing handlers filter by the requester_id query parameter and return every record when
// it is absent. That is correct on the central bank's own routes, where an operator is meant to see the
// whole book. On /internal/v1/payments/* it is a tenant boundary: the caller is one commercial bank,
// and an absent parameter silently hands it every other bank's issuance requests, tokenisations and
// redemptions.
//
// The calling bank's proxy sets the parameter from its own identity, so a legitimate request always
// carries it. This exists so the central bank does not depend on the caller having done that — an
// unscoped listing is refused here rather than answered from the whole table.
func RequireRequesterScope() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if strings.TrimSpace(c.Query("requester_id")) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "requester_id is required: an internal listing must name the institution it is for",
				"code":  "REQUESTER_SCOPE_REQUIRED",
			})
		}
		return c.Next()
	}
}
