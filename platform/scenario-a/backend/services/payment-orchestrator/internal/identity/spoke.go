// SPDX-License-Identifier: Apache-2.0

// Package identity provides utilities for parsing Paladin identity strings.
package identity

import (
	"fmt"
	"strings"
)

// SpokePrefix guesses the spoke identifier from a Paladin identity string by
// taking the first two hyphen-separated segments.
// e.g. "funded_operator@spoke-a-bank-a" → "spoke-a"
// Returns empty string if the format is unexpected.
//
// FALLBACK ONLY. The guess is wrong whenever the spoke id carries more than two
// segments ("spoke-costa-rica-cb1" → "spoke-costa") and it cannot be made right:
// a node name is <spokeId>-<bankId> with hyphens allowed in both halves, so the
// boundary is not recoverable. The authoritative source is the SPOKE_ID
// environment variable, which the toolkit exports; main.go prefers it and warns
// when it has to come here instead.
func SpokePrefix(paladinIdentity string) string {
	parts := strings.SplitN(paladinIdentity, "@", 2)
	if len(parts) < 2 {
		return ""
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 2 {
		return ""
	}
	return segs[0] + "-" + segs[1]
}

// BankID extracts the institution identifier from a Paladin identity string,
// assuming the spoke id is exactly two hyphen-separated segments.
//
// DO NOT USE FOR AUTHORIZATION — use BelongsToBank. That assumption is false:
// a spoke id may carry more segments ("spoke-costa-rica"), and this function then
// returns part of the spoke name glued to the bank id ("rica-cb1") with NO error,
// because the 3-way split still yields three pieces. Every authorization check
// built on it silently failed open or closed on such a spoke.
//
// It survives only for audit and log lines, where an approximate label is
// harmless and an outright wrong decision is not.
func BankID(paladinIdentity string) (string, error) {
	parts := strings.SplitN(paladinIdentity, "@", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("identity.BankID: missing '@' in %q — expected format {name}@{spoke-word}-{letter}-{bankID}", paladinIdentity)
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 3 {
		return "", fmt.Errorf("identity.BankID: fewer than three dash-segments in %q — expected format spoke-{letter}-{bankID}", parts[1])
	}
	return segs[2], nil
}

// BelongsToSpoke reports whether a Paladin identity lives on spokeID.
//
// This is the spoke counterpart of BelongsToBank, and it exists for the same
// reason: comparing two EXTRACTED prefixes is not a test of membership. Every
// node on a spoke is named <spokeId>-<suffix> (the toolkit's cbNodeName appends
// "-cb", bankNodeName appends "-"+bankId), so membership is a prefix test at that
// boundary — exact where extraction is a guess.
//
// The bug this replaces is worth stating, because it was invisible until a spoke
// id carried a hyphen. isLocalReceiver used to compare SpokePrefix(receiver)
// against the orchestrator's own spoke prefix, and BOTH sides were computed by the
// same wrong split, so they agreed and the check passed:
//
//	receiver spoke-costa-rica-cb2   SpokePrefix -> "spoke-costa"   (wrong)
//	local    spoke-costa-rica-cb1   SpokePrefix -> "spoke-costa"   (wrong)
//	                                              equal -> accepted
//
// Reading the local side from SPOKE_ID made it correct ("spoke-costa-rica") and so
// broke the symmetry that had been hiding the error: the two stopped agreeing and
// a bank could no longer lock an HTLC to its own neighbour on the same spoke. Two
// wrongs had been making a right.
//
// KNOWN LIMIT, and it is the same ambiguity that defeats extraction. A node name
// is <spokeId>-<bankId> with hyphens allowed in both halves, so
// "spoke-costa-rica-cb1" is genuinely ambiguous from the string alone:
//
//	spokeId=spoke-costa-rica  bankId=cb1        both structurally valid
//	spokeId=spoke-costa       bankId=rica-cb1
//
// So an orchestrator whose own spoke is "spoke-costa" accepts a receiver on
// "spoke-costa-rica" as local. No string test can separate those, and this one
// resolves the ambiguity by ACCEPTING — the fail-open direction, which is the wrong
// one for a validation gate. It is tolerated here only because it needs two live
// spokes whose ids share a hyphen boundary ("spoke-costa" alongside
// "spoke-costa-rica"), which no environment has today.
//
// The exact test is membership in the spoke's participant set, not the shape of a
// name: the gateway already federates the identity roster over the relay, so the
// orchestrator could ask instead of guessing. That is a network call on the lock
// path and a separate decision; see the card referenced in the PR.
func BelongsToSpoke(paladinIdentity, spokeID string) bool {
	spokeID = strings.TrimSpace(spokeID)
	if spokeID == "" {
		return false
	}
	node := strings.TrimSpace(paladinIdentity)
	if at := strings.Index(node, "@"); at >= 0 {
		node = node[at+1:]
	}
	if node == "" {
		return false
	}
	return strings.HasPrefix(node, spokeID+"-")
}

// BelongsToBank reports whether a Paladin identity belongs to bankID.
//
// This is the authorization primitive. Prefer it over BankID: it TESTS
// membership instead of EXTRACTING an id, and extraction is not possible.
//
// A node name is `<spokeId>-<bankId>` with both halves free to contain hyphens,
// so no split recovers the two parts:
//
//	spoke-costa-rica-cb1   spokeId=spoke-costa-rica  bankId=cb1
//	spoke-brl-bank-itau    spokeId=spoke-brl         bankId=bank-itau
//
// Both shapes are live — LNET manifests use cb1…cb6, the samples use bank-itau.
// BankID's fixed 3-way split reads the first as bankId="rica-cb1", which silently
// defeated every authorization check on a two-word country: the FX "originator
// cannot accept their own agreement" guard compared "rica-cb1" to "cb1", found no
// match, and let the originator self-approve.
//
// Testing is exact where extracting is not: the caller already knows the bank id
// it is asking about, so the identity only has to end at that boundary. The
// leading "-" is what prevents substring spoofing — bankId "bank" must not match
// a node ending in "-bank-abc", and "cb1" must not match one ending in "-cb11".
//
// Parties may also be stored as a bare bank id ("cb1") rather than a full
// identity; that form is accepted by exact equality.
//
// Kept in sync with identityBelongsToBank in the api-gateway's payment handler
// and identityBelongsToBank in the bank portal's features/fx/identity.ts. All
// three carry the same case table in their tests; change them together.
func BelongsToBank(paladinIdentity, bankID string) bool {
	bankID = strings.TrimSpace(bankID)
	if bankID == "" {
		return false
	}
	node := strings.TrimSpace(paladinIdentity)
	if at := strings.Index(node, "@"); at >= 0 {
		node = node[at+1:]
	}
	if node == "" {
		return false
	}
	return node == bankID || strings.HasSuffix(node, "-"+bankID)
}
