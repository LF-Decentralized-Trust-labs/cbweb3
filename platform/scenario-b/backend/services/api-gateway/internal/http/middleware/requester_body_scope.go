// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// requesterBesuAddressField is the body field naming the institution a payment record is created
// for. On /internal/v1/payments/* it is the tenant boundary, exactly as requester_id is on the
// listings — and, like it, it arrived from the caller.
const requesterBesuAddressField = "requester_besu_address"

// DepositOwnership reports whether a deposit was registered by a given institution.
//
// Needed because one creation route names no address to overwrite: the fiat exchange carries a
// deposit id, and the mint that follows credits whoever registered THAT deposit. Binding it means
// asking whose deposit it is.
type DepositOwnership interface {
	OwnsDeposit(ctx context.Context, requesterAddress, depositID string) (bool, error)
}

// ScopeRequesterBodyToCaller sets requester_besu_address from the VERIFIED caller and discards
// whatever the body asked for. It is the write-side counterpart of ScopeRequesterToCaller.
//
// Scoping the listings closed the read half of the tenant boundary and left the write half open. The
// creation routes — deposits, escrows, redeems — read the address straight from the body, and the
// bank proxy that injects it honestly protects only honest proxy traffic. An onboarded bank holds a
// pinned signing identity precisely so it can call /internal, so it passes the enforcement middleware
// and the handler then trusts the body: it can create a deposit, an escrow or a redeem against
// ANOTHER bank's address. No value moves to the caller, but the victim's records grow acts it never
// requested, and an operator approving one burns the victim's fCeBM or converts its tCeBM.
//
// The value is overwritten rather than compared, for the same reason as on the read side: two
// spellings of an address (checksummed or not) are one identity, and a mismatch check would reject
// legitimate traffic over capitalisation.
//
// Fails closed at every step — no verified caller, no resolver, an unresolvable caller, or a body
// whose field cannot be replaced. Passing any of those through would let the body decide the tenant,
// which is the whole finding.
func ScopeRequesterBodyToCaller(resolver RequesterScopeResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		addr, ok, refusal := callerAddress(c, resolver)
		if !ok {
			return refusal
		}
		fields, ok, refusal := bodyObject(c)
		if !ok {
			return refusal
		}

		if supplied := stringField(fields, requesterBesuAddressField); supplied != "" && !strings.EqualFold(supplied, addr) {
			// The value is replaced either way, but a caller naming someone else is the fingerprint
			// of the attack this closes and must not pass unrecorded.
			log.Printf("[requester-scope] caller %q named requester_besu_address=%q; binding it to its own address instead",
				sanitizeCaller(VerifiedRelayCaller(c)), sanitizeCaller(supplied))
		}

		encoded, err := json.Marshal(addr)
		if err != nil { // a string always marshals; the branch exists so a future change cannot slip through
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "could not bind the request to the calling institution",
				"code":  "REQUESTER_SCOPE_FAILED",
			})
		}
		fields[requesterBesuAddressField] = encoded

		rewritten, err := json.Marshal(fields)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "could not bind the request to the calling institution",
				"code":  "REQUESTER_SCOPE_FAILED",
			})
		}
		// Rewriting after verification is safe and has to be: the signature covers the bytes the
		// caller sent, so it is verified upstream, over the original body.
		c.Request().SetBody(rewritten)
		c.Request().Header.SetContentLength(len(rewritten))
		return c.Next()
	}
}

// BindDepositToCaller refuses a request that names a deposit belonging to another institution.
//
// The fiat exchange is the creation route with no address in its body: it names a deposit id and
// mints fCeBM to whoever registered that deposit. Overwriting a field would bind nothing here, so
// the binding is the ownership question itself. A deposit id is a correlator, not a permission.
//
// Value does not move to the caller — the mint credits the rightful owner — but driving another
// bank's deposit through its exchange is an act on the victim's record that its operator never
// authorized, and it consumes the one-mint-per-deposit guard the owner was relying on.
func BindDepositToCaller(resolver RequesterScopeResolver, owner DepositOwnership) fiber.Handler {
	return func(c *fiber.Ctx) error {
		addr, ok, refusal := callerAddress(c, resolver)
		if !ok {
			return refusal
		}
		fields, ok, refusal := bodyObject(c)
		if !ok {
			return refusal
		}
		depositID := stringField(fields, "deposit_id")
		if depositID == "" {
			// Nothing to bind to. The handler validates presence itself and answers 400, which is the
			// message an operator can act on; refusing here would only mask it as a 403.
			return c.Next()
		}
		if owner == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "this gateway cannot establish who owns the named deposit, so the request cannot be " +
					"bound to the calling institution",
				"code": "DEPOSIT_OWNERSHIP_UNAVAILABLE",
			})
		}
		owns, err := owner.OwnsDeposit(c.Context(), addr, depositID)
		if err != nil {
			log.Printf("[requester-scope] could not establish ownership of deposit %q for %q: %v",
				sanitizeCaller(depositID), sanitizeCaller(addr), err)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "could not establish who owns the named deposit",
				"code":  "DEPOSIT_OWNERSHIP_UNAVAILABLE",
			})
		}
		if !owns {
			log.Printf("[requester-scope] caller %q named deposit %q, which is not its own",
				sanitizeCaller(VerifiedRelayCaller(c)), sanitizeCaller(depositID))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "the named deposit does not belong to the calling institution",
				"code":  "REQUESTER_NOT_DEPOSIT_OWNER",
			})
		}
		return c.Next()
	}
}

// callerAddress resolves the on-chain address of the institution whose signature authenticated this
// request. When it cannot, the refusal is already written and the returned error is what the
// middleware must return.
//
// Three results rather than an error alone: Fiber's JSON() returns nil on success, so "refused"
// would be indistinguishable from "allowed" at the call site.
func callerAddress(c *fiber.Ctx, resolver RequesterScopeResolver) (string, bool, error) {
	caller := strings.TrimSpace(VerifiedRelayCaller(c))
	if caller == "" {
		return "", false, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "this endpoint answers for the calling institution only, so the request must carry a " +
				"per-entity relay signature: the shared secret is identical in every entity and names no caller",
			"code": "RELAY_CALLER_IDENTITY_REQUIRED",
		})
	}
	if resolver == nil {
		return "", false, c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "this gateway cannot resolve the calling institution's address, so the request cannot be " +
				"scoped to it (participant lookup unavailable)",
			"code": "REQUESTER_SCOPE_UNAVAILABLE",
		})
	}
	addr, err := resolver.ResolveWalletAddress(c.Context(), caller)
	if err != nil || strings.TrimSpace(addr) == "" {
		log.Printf("[requester-scope] refusing to act for %q: %v", sanitizeCaller(caller), err)
		return "", false, c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "the calling institution is not an active participant of this central bank",
			"code":  "REQUESTER_NOT_A_PARTICIPANT",
		})
	}
	return strings.TrimSpace(addr), true, nil
}

// bodyObject parses the request body as a JSON object, preserving every field it does not touch.
//
// A body that is not an object is refused rather than passed through: the field could not be bound,
// and forwarding the caller's original bytes is exactly the state these middlewares exist to
// prevent. An empty body is an empty object — the handler's own validation then reports what is
// missing.
func bodyObject(c *fiber.Ctx) (map[string]json.RawMessage, bool, error) {
	raw := c.Body()
	fields := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fields, true, nil
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
			"code":  "INVALID_REQUEST_BODY",
		})
	}
	return fields, true, nil
}

// stringField reads one string field, treating any other JSON type as absent — a caller cannot make
// a number or an object mean an address.
func stringField(fields map[string]json.RawMessage, name string) string {
	raw, ok := fields[name]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}
