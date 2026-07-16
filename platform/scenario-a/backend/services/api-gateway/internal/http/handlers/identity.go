// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// IdentityRosterProvider yields the live FX-party roster: the distinct Paladin
// identities that are actual members of the node's Pente groups. Implemented by
// the payment-orchestrator adapter (which queries pgroup_queryGroups).
type IdentityRosterProvider interface {
	ListParticipantIdentities(ctx context.Context) ([]string, error)
}

// IdentityRoster resolves the Paladin identities offered as FX agreement party
// choices. The roster is the UNION of two sources:
//
//  1. A configured roster (PALADIN_IDENTITIES). This is the consortium-wide list
//     and is the only source that can carry cross-spoke identities: FX groups are
//     bilateral and node-local, so a spoke's Paladin node cannot enumerate the
//     identities of a remote spoke it must trade with (the destination party).
//  2. Live local Pente group membership, which keeps the local spoke's entries
//     current as banks join/leave without a config change.
//
// There is deliberately no built-in demo fallback: when both sources are empty
// the roster is empty and the caller surfaces "roster not configured", instead
// of silently offering identities from another network.
type IdentityRoster struct {
	override []string
	live     IdentityRosterProvider
}

// NewIdentityRoster builds a roster from an optional static override and an
// optional live provider. Either may be empty/nil.
func NewIdentityRoster(override []string, live IdentityRosterProvider) *IdentityRoster {
	cp := make([]string, len(override))
	copy(cp, override)
	return &IdentityRoster{override: cp, live: live}
}

// Identities returns the union of the configured roster and live Pente
// membership, de-duplicated, with configured entries first (order preserved).
// The configured roster is authoritative: if it is non-empty, a live-membership
// fetch error is tolerated (logged, best-effort) rather than failing the whole
// roster. When there is no configured roster, a live error is returned so the
// caller can fail closed.
func (r *IdentityRoster) Identities(ctx context.Context) ([]string, error) {
	seen := make(map[string]struct{}, len(r.override))
	out := make([]string, 0, len(r.override))
	add := func(ids []string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	add(r.override)

	if r.live != nil {
		live, err := r.live.ListParticipantIdentities(ctx)
		if err != nil {
			if len(out) > 0 {
				log.Printf("warning: live Pente membership unavailable, using configured roster only: %v", err)
				return out, nil
			}
			return nil, err
		}
		add(live)
	}
	return out, nil
}

// IdentityHandler exposes the Paladin identities available as party choices in
// the FX agreement form. The list reflects real Pente membership (see
// IdentityRoster) rather than a hand-maintained static list.
//
// The default roster is network-wide (federated across every spoke). localRoster,
// when set, is this spoke's LOCAL-only roster and is served for ?scope=local — the
// endpoint peer gateways call during federation. Serving the local roster there is
// the recursion guard: a peer lookup must not itself fan out across the network.
type IdentityHandler struct {
	roster      *IdentityRoster
	localRoster *IdentityRoster
}

// NewIdentityHandler creates an IdentityHandler over the given roster. The same
// roster answers both the default and the ?scope=local request (used when the
// roster is not federated, so local and network-wide are identical).
func NewIdentityHandler(roster *IdentityRoster) *IdentityHandler {
	return &IdentityHandler{roster: roster, localRoster: roster}
}

// NewFederatedIdentityHandler creates a handler whose default request returns the
// network-wide (federated) roster and whose ?scope=local request returns the
// local-only roster.
func NewFederatedIdentityHandler(federated, local *IdentityRoster) *IdentityHandler {
	return &IdentityHandler{roster: federated, localRoster: local}
}

// ListIdentities returns the available Paladin identities.
//
//	GET /api/v1/identities              -> network-wide (federated) roster
//	GET /api/v1/identities?scope=local  -> this spoke's local-only roster
//
//	{ "identities": ["funded_operator@spoke-a-cb", ...], "configured": true }
//
// When the roster is empty, "configured" is false so the UI can surface
// "roster not configured" rather than presenting a misleading empty dropdown.
func (h *IdentityHandler) ListIdentities(c *fiber.Ctx) error {
	roster := h.roster
	if c.Query("scope") == "local" && h.localRoster != nil {
		roster = h.localRoster
	}
	return h.respond(c, roster)
}

// ListLocalIdentities serves this spoke's LOCAL-only roster. It backs the
// internal gateway-to-gateway endpoint peers call during federation (protected
// by X-Relay-Auth, not a user cookie). Returning the local roster is the
// recursion guard: a network-wide lookup fans out to peers' local rosters, which
// must not themselves federate.
//
//	GET /internal/v1/identities -> { "identities": [...], "configured": true }
func (h *IdentityHandler) ListLocalIdentities(c *fiber.Ctx) error {
	roster := h.localRoster
	if roster == nil {
		roster = h.roster
	}
	return h.respond(c, roster)
}

func (h *IdentityHandler) respond(c *fiber.Ctx, roster *IdentityRoster) error {
	identities, err := roster.Identities(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "list identities: " + err.Error()})
	}
	if len(identities) == 0 {
		log.Printf("warning: Paladin identity roster is empty — roster not configured (set PALADIN_IDENTITIES or enable Pente membership)")
		return c.JSON(fiber.Map{"identities": []string{}, "configured": false})
	}
	return c.JSON(fiber.Map{"identities": identities, "configured": true})
}
