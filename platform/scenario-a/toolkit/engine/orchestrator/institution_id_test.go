// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"encoding/hex"
	"testing"
)

// The AMM resume quorum counts distinct institutions, and that only works if every wallet
// of one institution resolves to the SAME institutionId. The id is keccak256(bankCode),
// computed in three independent implementations:
//
//   - this toolkit (institutionIDFromCode), which onboards entities in real deployments;
//   - the Go services (backend/shared/blockchain/registry.InstitutionIDFromString);
//   - the seed script (contracts/script/RegisterParticipants.s.sol).
//
// The toolkit is a separate Go module and cannot import the services' copy, so agreement
// cannot be enforced by construction. It is enforced by value instead: all three assert the
// SAME literals below. Recomputing keccak256 inside the test would only prove the test and
// the code agree — two implementations that both drifted the same way would still pass.
//
// Getting this wrong fails in the quiet direction. If one path derives a different id for a
// bank's second governance wallet, the contract sees two institutions and that bank can
// resume the circuit breaker on its own — the exact outcome the quorum exists to prevent,
// reached without anything erroring.
var pinnedInstitutionIDs = map[string]string{
	"central-bank-a": "1581556895c0bf3377dffd4c68bd1ada3f1f3d6aac4d828859fd4c862e9a2769",
	"bank-a":         "ee8ed86961a76066712cc2d2c7c9faed0a887ade4278e8d358d4044bc91f5834",
	"central-bank-b": "a7417e4e6b59702f4117b65b72e2d7022c272b3fafa8ca90d4962efbfe514619",
	"bank-b":         "85b692ac840718c4c20a3acce04167775b2adbf21f59bcf5bd69abd16c1f9512",
}

func TestInstitutionIDFromCode_MatchesThePinnedValues(t *testing.T) {
	t.Parallel()
	for code, want := range pinnedInstitutionIDs {
		id := institutionIDFromCode(code)
		got := hex.EncodeToString(id[:])
		if got != want {
			t.Errorf("institutionIDFromCode(%q) = %s, want %s\n"+
				"  The Go services and the seed script pin the same value. A toolkit-registered\n"+
				"  wallet and a service-registered wallet of one institution now resolve to\n"+
				"  different ids, which the AMM reads as two institutions.", code, got, want)
		}
	}
}

// A code that differs must produce a different id, or every institution collides into one
// and the quorum can never be met by two genuinely distinct banks.
func TestInstitutionIDFromCode_DistinctCodesDistinctIDs(t *testing.T) {
	t.Parallel()
	if institutionIDFromCode("central-bank-a") == institutionIDFromCode("bank-a") {
		t.Fatal("distinct bank codes collided onto one institutionId")
	}
}

// bytes32(0) is what an unregistered address returns, and both the registry and the AMM
// reject it. The toolkit must never produce it for a real code.
func TestInstitutionIDFromCode_IsNeverZero(t *testing.T) {
	t.Parallel()
	var zero [32]byte
	for code := range pinnedInstitutionIDs {
		if institutionIDFromCode(code) == zero {
			t.Errorf("institutionIDFromCode(%q) produced the zero id, which the registry rejects", code)
		}
	}
}
