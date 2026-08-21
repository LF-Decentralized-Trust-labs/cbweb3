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
// carries the same bankCode, and it matches the strings the on-chain seed script uses
// (keccak256("central-bank-a"), see contracts/script/RegisterParticipants.s.sol). Deriving
// the id from bankCode is therefore what guarantees that all of an institution's wallets
// resolve to the same institutionId regardless of which onboarding path registered them.
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
func InstitutionIDForParticipant(bankCode, name string) [32]byte {
	if strings.TrimSpace(bankCode) != "" {
		return InstitutionIDFromString(bankCode)
	}
	return InstitutionIDFromString(name)
}
