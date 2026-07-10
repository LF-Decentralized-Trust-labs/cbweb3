// SPDX-License-Identifier: Apache-2.0

package handlers

import "github.com/gofiber/fiber/v2"

// IdentityHandler exposes the Paladin identities available as party choices in
// the FX agreement form. The list is configured at startup (PALADIN_IDENTITIES)
// rather than enumerated on-chain, since Paladin exposes no "list all" query.
type IdentityHandler struct {
	identities []string
}

// NewIdentityHandler creates an IdentityHandler serving the given identities.
func NewIdentityHandler(identities []string) *IdentityHandler {
	// Defensive copy so later mutation of the config slice cannot leak through.
	list := make([]string, len(identities))
	copy(list, identities)
	return &IdentityHandler{identities: list}
}

// ListIdentities returns the available Paladin identities.
//
//	GET /api/v1/identities -> { "identities": ["funded_operator@spoke-a-cb", ...] }
func (h *IdentityHandler) ListIdentities(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"identities": h.identities})
}
