// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// RequesterScopeResolver maps a verified caller's entity id to the on-chain address its payment
// records are keyed by. Implemented by the participants table, which is the central bank's own
// record of who its member banks are and where they hold value.
type RequesterScopeResolver interface {
	ResolveWalletAddress(ctx context.Context, bankCode string) (string, error)
}

// ScopeRequesterToCaller sets requester_id from the VERIFIED caller and discards whatever the request
// asked for.
//
// Why the presence check above was not enough. The calling bank's proxy scopes the parameter to its
// own address, which protects honest proxy traffic — but the parameter still arrived from the caller,
// and the signature does not cover the query string. An onboarded bank, which section 6 deliberately
// gives a pinned signing identity precisely so it can call /internal, could therefore sign
// "GET /internal/v1/payments/deposits" and attach "?requester_id=<another bank's address>": the
// signature verifies over the path, the presence check passes, and the central bank answers with the
// other bank's records. Same leak the scoping was introduced to close, one step more deliberate.
//
// So the value is derived here instead of trusted. A supplied value is overwritten rather than
// compared, because the two spellings of an address (checksummed or not) are the same identity and a
// mismatch check would reject legitimate traffic over capitalisation.
//
// Fails closed: no verified caller, no resolver, or an unresolvable caller all refuse the listing.
// Answering unscoped would mean returning the whole book to a caller entitled to one bank's rows.
func ScopeRequesterToCaller(resolver RequesterScopeResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Shared with the write side (ScopeRequesterBodyToCaller): resolving the caller and refusing
		// when it cannot be resolved is the same decision whichever part of the request carries the
		// tenant, and one copy is what keeps the two halves of the boundary from drifting apart.
		addr, ok, refusal := callerAddress(c, resolver)
		if !ok {
			return refusal
		}
		caller := strings.TrimSpace(VerifiedRelayCaller(c))

		if supplied := strings.TrimSpace(c.Query("requester_id")); supplied != "" && !strings.EqualFold(supplied, addr) {
			// Not an error to answer — the value is replaced either way — but it is the fingerprint of
			// a caller asking about someone else, and it must not pass unrecorded.
			log.Printf("[requester-scope] caller %q asked for requester_id=%q; scoping to its own address instead",
				sanitizeCaller(caller), sanitizeCaller(supplied))
		}
		c.Request().URI().QueryArgs().Set("requester_id", addr)
		return c.Next()
	}
}

// sanitizeCaller keeps attacker-controlled text out of the log as anything but one flat token: a
// key-id and a query parameter both arrive from the request.
func sanitizeCaller(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > 128 {
		return s[:128]
	}
	return s
}
