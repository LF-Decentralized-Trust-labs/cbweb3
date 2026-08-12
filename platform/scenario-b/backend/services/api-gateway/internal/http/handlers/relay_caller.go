// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// relayCallerRefusal is a refusal to let the caller act for the institution it named: the HTTP
// status, a stable code, and text an operator can act on. Nil means the request may proceed.
type relayCallerRefusal struct {
	Status  int
	Code    string
	Message string
}

// decideRelayCaller authorizes a request that acts on behalf of claimedBank.
//
// The endpoints behind this — the delegated hub swap, bridge-in, the residue return — spend a
// central bank's own Hub balance for a named commercial bank. The signature layer proves WHICH peer
// is calling; without this, authorization then happened against the bank id in the request body, so
// any authenticated peer could name another bank (plus that bank's position id, which is a
// correlator and not a permission) and have the CB spend against it.
//
// verifiedCaller is empty when the request was authenticated by the legacy shared secret. That
// secret is identical in every entity, so it cannot say who is calling and these routes refuse it —
// deliberately stricter than RELAY_REQUIRE_SIGNATURE, which is about the deployment as a whole. A
// bank whose signing key fails to load gets a loud 401 here instead of silently acquiring the
// ability to act as any other bank.
//
// Pure over the two strings so the rule is tested directly rather than through an HTTP fixture.
func decideRelayCaller(verifiedCaller, claimedBank string) *relayCallerRefusal {
	caller := strings.TrimSpace(verifiedCaller)
	claimed := strings.TrimSpace(claimedBank)
	if claimed == "" {
		// Nothing to bind to. Handlers validate the field's presence themselves and answer 400,
		// which is the more precise message; refusing here too would only mask it.
		return nil
	}
	if caller == "" {
		return &relayCallerRefusal{
			Status: fiber.StatusUnauthorized,
			Code:   "RELAY_CALLER_IDENTITY_REQUIRED",
			Message: "this endpoint acts on behalf of the calling institution, so the request must carry a " +
				"per-entity relay signature (X-Relay-Key-Id/-Timestamp/-Signature): the shared secret is " +
				"identical in every entity and does not identify a caller",
		}
	}
	if !strings.EqualFold(caller, claimed) {
		return &relayCallerRefusal{
			Status: fiber.StatusForbidden,
			Code:   "RELAY_CALLER_BANK_MISMATCH",
			Message: "the verified caller may only act for itself: this request is signed by " +
				caller + " and names " + claimed,
		}
	}
	return nil
}

// authorizeRelayCallerFor is the one call sites make. It reports whether the verified caller may act
// for claimedBank; when it may not, the refusal is already written and its error is what the handler
// must return.
//
// Two results rather than one error: Fiber's JSON() returns nil on success, so a single error return
// would make "refused" indistinguishable from "allowed" at every call site.
func authorizeRelayCallerFor(c *fiber.Ctx, claimedBank string) (bool, error) {
	d := decideRelayCaller(middleware.VerifiedRelayCaller(c), claimedBank)
	if d == nil {
		return true, nil
	}
	return false, c.Status(d.Status).JSON(fiber.Map{"error": d.Message, "code": d.Code})
}
