// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestInstitutionIDFromString_DeterministicAndNonZero(t *testing.T) {
	t.Parallel()
	first := InstitutionIDFromString("central-bank-a")
	second := InstitutionIDFromString("central-bank-a")
	if first != second {
		t.Fatalf("same input must yield the same id: %x vs %x", first, second)
	}
	// The contract rejects a zero institutionId, so a derivation that could produce one
	// would fail at registration rather than at the quorum, but fail either way.
	if first == ([32]byte{}) {
		t.Fatal("institution id must be non-zero")
	}
}

func TestInstitutionIDFromString_DistinctInputsDistinctIDs(t *testing.T) {
	t.Parallel()
	if InstitutionIDFromString("central-bank-a") == InstitutionIDFromString("central-bank-b") {
		t.Fatal("two central banks must not collide onto one institution id")
	}
}

// The Go services, the Foundry seed script and the provisioning toolkit each derive the id
// independently. They agree only because all three are keccak256 over the bank code — pin
// that here, so a change in one of them fails a test instead of silently splitting one
// institution into two on-chain identities.
func TestInstitutionIDFromString_IsKeccak256OfTheCode(t *testing.T) {
	t.Parallel()
	var want [32]byte
	copy(want[:], crypto.Keccak256([]byte("central-bank-a")))
	if got := InstitutionIDFromString("central-bank-a"); got != want {
		t.Fatalf("derivation drifted from keccak256(bankCode): got %x want %x", got, want)
	}
}

func TestInstitutionIDForParticipant_BankCodeWinsOverName(t *testing.T) {
	t.Parallel()
	got := InstitutionIDForParticipant("bank-a", "Bank A")
	if want := InstitutionIDFromString("bank-a"); got != want {
		t.Fatalf("bank code must be the source: got %x want %x", got, want)
	}
}

func TestInstitutionIDForParticipant_FallsBackToNameWhenCodeBlank(t *testing.T) {
	t.Parallel()
	got := InstitutionIDForParticipant("   ", "Bank A")
	if want := InstitutionIDFromString("Bank A"); got != want {
		t.Fatalf("blank bank code must fall back to the name: got %x want %x", got, want)
	}
}

// The property the AMM resume quorum rests on: two wallets of one institution, onboarded
// through different paths and under different display names, must still be one institution.
func TestInstitutionIDForParticipant_TwoWalletsOneInstitution(t *testing.T) {
	t.Parallel()
	operational := InstitutionIDForParticipant("central-bank-a", "Central Bank A")
	governance := InstitutionIDForParticipant("central-bank-a", "CB-A Governance Signer")
	if operational != governance {
		t.Fatalf("one bank code must yield one institution id: %x vs %x", operational, governance)
	}
}

// The same literals the toolkit and the seed script pin.
//
// The existing test above proves this implementation is keccak256 of the code. That is not
// enough on its own: three independent implementations derive this id — this one, the
// provisioning toolkit (a separate Go module that cannot import this package), and
// contracts/script/RegisterParticipants.s.sol — and each recomputing keccak256 in its own
// test would let all three drift together without a failure.
//
// Pinning the VALUE ties them. If this list and the toolkit's disagree, one of the two
// changed, and the AMM resume quorum would read two wallets of one institution as two
// institutions — the failure the quorum exists to prevent, reached silently.
func TestInstitutionIDFromString_MatchesThePinnedValues(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]string{
		"central-bank-a": "1581556895c0bf3377dffd4c68bd1ada3f1f3d6aac4d828859fd4c862e9a2769",
		"bank-a":         "ee8ed86961a76066712cc2d2c7c9faed0a887ade4278e8d358d4044bc91f5834",
		"central-bank-b": "a7417e4e6b59702f4117b65b72e2d7022c272b3fafa8ca90d4962efbfe514619",
		"bank-b":         "85b692ac840718c4c20a3acce04167775b2adbf21f59bcf5bd69abd16c1f9512",
	} {
		id := InstitutionIDFromString(code)
		if got := hex.EncodeToString(id[:]); got != want {
			t.Errorf("InstitutionIDFromString(%q) = %s, want %s\n"+
				"  The toolkit and the seed script pin the same value; this derivation has drifted "+
				"from them.", code, got, want)
		}
	}
}
