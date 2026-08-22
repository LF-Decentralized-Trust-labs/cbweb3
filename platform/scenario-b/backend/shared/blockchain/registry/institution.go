// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// InstitutionIDFromString derives a stable on-chain institution identifier from an
// opaque string (a bank code, or a legal name as the last-resort fallback).
func InstitutionIDFromString(s string) [32]byte {
	var out [32]byte
	copy(out[:], crypto.Keccak256([]byte(s)))
	return out
}

// InstitutionIDForParticipant derives the on-chain institutionId for a participant from
// a single canonical source: bankCode.
//
// bankCode is the per-institution identity — every wallet belonging to one institution
// carries the same bankCode. In Scenario B that code is the manifest entity id, which the
// compose template hands each service as BANK_CODE, and which the on-chain seed scripts
// hash the same way (keccak256("central-bank-a"), see contracts/script/SeedHub.s.sol).
// Deriving the id from bankCode is therefore what guarantees that all of an institution's
// wallets resolve to the same institutionId regardless of which path registered them.
//
// That property is load-bearing for the AMM circuit-breaker resume quorum, which counts
// distinct institutions rather than distinct addresses: one central bank operating two
// governance wallets must not satisfy the 2-of-N on its own. Mixing in any other
// identifier (a legal entity id, a display name) would break it in the quiet direction —
// two wallets of one institution receiving different ids depending on the path that
// onboarded them, which reads as two institutions to the contract.
//
// name is used only when bankCode is absent, and is a fallback rather than a second
// source: two wallets of one institution agree on it only if the caller spells the name
// identically both times.
//
// Deliberately duplicated from Scenario A rather than shared: the constitution forbids
// reaching across scenarios, and the two registries are separate deployments. The
// derivation must stay identical in both, so a change here belongs in both copies.
func InstitutionIDForParticipant(bankCode, name string) [32]byte {
	if strings.TrimSpace(bankCode) != "" {
		return InstitutionIDFromString(bankCode)
	}
	return InstitutionIDFromString(name)
}

// InstitutionCodeFromEnv resolves the code identifying THIS service's own institution, for
// the paths that register the entity's own wallet rather than a counterparty's.
//
// The order matters, and BANK_CODE is deliberately last. In Scenario B the compose template
// sets BANK_CODE to the entity ROLE (apply.go derives it from spec.topology.role), so every
// central bank in a deployment carries "central-bank". Hashing that into an institutionId
// would make every central bank ONE institution — and since the AMM resume quorum requires
// two distinct institutions, a paused AMM could then never be resumed. INSTITUTION_CODE is
// the value the toolkit renders per entity for exactly this reason.
//
// Returns the code and whether it came from a source that is unique per entity. Callers
// should log loudly when it is not, rather than register a wallet under a colliding id.
func InstitutionCodeFromEnv(getenv func(string) string) (code string, unique bool) {
	if c := strings.TrimSpace(getenv("INSTITUTION_CODE")); c != "" {
		return c, true
	}
	if c := strings.TrimSpace(getenv("GOVERNANCE_BANK_CODE")); c != "" {
		return c, true
	}
	// BANK_CODE is the entity role here, so it is shared by every entity of that role.
	return strings.TrimSpace(getenv("BANK_CODE")), false
}
