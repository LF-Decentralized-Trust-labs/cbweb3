// SPDX-License-Identifier: Apache-2.0

package server

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// The party identities of the live LNET deploy, verbatim. Four of them used to collapse onto the
// single address 0x00...000F, because common.HexToAddress left-pads an odd-length string, decodes
// the leading "0f" of "0funded_operator@..." and stops at the first non-hex byte — and the old
// predicate (`HexToAddress(v) != Address{}`) accepted that non-zero garbage as a real address.
//
// The roster is the case table on purpose. The bug was invisible under the synthetic
// "alice@spoke-a-bank-a" fixtures the other tests use, and surfaced only in Costa Rica, where the
// two-word country name pushes the central bank's node name to an odd length.
var lnetIdentities = []struct {
	identity string
	oddLen   bool // odd length ⇒ hit by the old bug
}{
	{"funded_operator@spoke-chile-cb", false},       // 30
	{"funded_operator@spoke-chile-cb3", true},       // 31
	{"funded_operator@spoke-chile-cb4", true},       // 31
	{"funded_operator@spoke-costa-rica-cb", true},   // 35 — central bank
	{"funded_operator@spoke-costa-rica-cb1", false}, // 36
	{"funded_operator@spoke-costa-rica-cb2", false}, // 36
	{"funded_operator@spoke-peru-cb", true},         // 29 — central bank
	{"funded_operator@spoke-peru-cb5", false},       // 30
	{"funded_operator@spoke-peru-cb6", false},       // 30
	// The sample deploy's hyphenated bank ids, so both live naming conventions are covered.
	// They are hit too: parity is a property of the whole string, so nothing about the bank-id
	// convention protects a spoke. 6 of these 13 identities collapsed before the fix.
	{"funded_operator@spoke-brl-cb", false},               // 28
	{"funded_operator@spoke-brl-bank-itau", true},         // 35
	{"funded_operator@spoke-cop-bank-bancolombia", false}, // 42
	{"funded_operator@spoke-cop-bank-davivienda", true},   // 41
}

// collapsedAddr is what every odd-length identity used to produce.
var collapsedAddr = common.HexToAddress("0x000000000000000000000000000000000000000F")

// TestLNETIdentityTable_ParityMarkersAreCorrect checks the fixture against itself. The oddLen
// column is hand-written documentation of which identities the bug hit, and a wrong marker
// silently weakens TestPartyAddress_NoOddLengthCollapse by excluding a real case from it — which
// is exactly what happened while writing this table (davivienda, 41 chars, was marked even).
func TestLNETIdentityTable_ParityMarkersAreCorrect(t *testing.T) {
	for _, tc := range lnetIdentities {
		if want := len(tc.identity)%2 == 1; tc.oddLen != want {
			t.Errorf("%q has length %d (odd=%v) but is marked oddLen=%v",
				tc.identity, len(tc.identity), want, tc.oddLen)
		}
	}
}

func TestPartyAddress_DistinctPerIdentity(t *testing.T) {
	seen := map[common.Address]string{}
	for _, tc := range lnetIdentities {
		got := partyAddress(tc.identity)
		if prev, dup := seen[got]; dup {
			t.Errorf("collision: %q and %q both map to %s", prev, tc.identity, got)
			continue
		}
		seen[got] = tc.identity
	}
	if len(seen) != len(lnetIdentities) {
		t.Errorf("got %d distinct addresses for %d identities", len(seen), len(lnetIdentities))
	}
}

// TestPartyAddress_NoOddLengthCollapse pins the specific failure. It is the mutation guard: revert
// partyAddress to the zero-address check and the four odd-length identities fail here.
func TestPartyAddress_NoOddLengthCollapse(t *testing.T) {
	var oddSeen int
	for _, tc := range lnetIdentities {
		if !tc.oddLen {
			continue
		}
		oddSeen++
		if len(tc.identity)%2 == 0 {
			t.Errorf("table wrong: %q is marked odd but has even length %d", tc.identity, len(tc.identity))
		}
		if got := partyAddress(tc.identity); got == collapsedAddr {
			t.Errorf("%q collapsed to %s — partyAddress is accepting a partial hex decode", tc.identity, got)
		}
	}
	if oddSeen == 0 {
		t.Fatal("the table no longer contains an odd-length identity; the guard proves nothing")
	}
}

func TestPartyAddress_RealAddressUsedVerbatim(t *testing.T) {
	// A well-formed address must survive untouched — it is the party's actual signer key, and
	// hashing it would silently replace a real address with a placeholder.
	for _, in := range []string{
		"0xabcabcabcabcabcabcabcabcabcabcabcabcabca",
		"0xAd6E7a6De8d8591C3F5F2a2Ba56383A012566912", // Itaú, resolved on its own node
		"0xb1a5c59cfde8fff05b134d384f739def4193aafc", // Bancolombia, resolved on its own node
	} {
		if got := partyAddress(in); got != common.HexToAddress(in) {
			t.Errorf("partyAddress(%q) = %s, want %s", in, got, common.HexToAddress(in))
		}
	}
}

func TestPartyAddress_EmptyIsZero(t *testing.T) {
	// Zero is meaningful: FXAgreement rejects a zero counterpartyB, so an absent optional party
	// must stay zero rather than become a hash of "".
	for _, in := range []string{"", "   ", "\t\n"} {
		if got := partyAddress(in); got != (common.Address{}) {
			t.Errorf("partyAddress(%q) = %s, want the zero address", in, got)
		}
	}
}

func TestPartyAddress_NonZeroForEveryIdentity(t *testing.T) {
	// propose validations require counterpartyB != 0, so no identity may derive to zero.
	for _, tc := range lnetIdentities {
		if partyAddress(tc.identity) == (common.Address{}) {
			t.Errorf("partyAddress(%q) is the zero address", tc.identity)
		}
	}
}

func TestPartyAddress_Deterministic(t *testing.T) {
	// The derived address is written on-chain, so it must not drift between calls or processes.
	for _, tc := range lnetIdentities {
		if a, b := partyAddress(tc.identity), partyAddress(" "+tc.identity+" "); a != b {
			t.Errorf("%q: not stable under surrounding whitespace: %s vs %s", tc.identity, a, b)
		}
	}
}

// TestPartyAddress_RejectsTruncatedHex covers the behaviour change this fix makes deliberately.
// A short "0x…" string is NOT an address; the old code accepted it because the partial decode was
// non-zero. Deriving from it is the safe reading — a truncated address is a caller error, and
// treating it as real would authorise the wrong key.
func TestPartyAddress_RejectsTruncatedHex(t *testing.T) {
	for _, in := range []string{"0xabc", "0x0", "0xAd6E7a6De8d8591C3F5F2a2Ba56383A0125669"} {
		got := partyAddress(in)
		if got == common.HexToAddress(in) {
			t.Errorf("partyAddress(%q) = %s — a truncated hex string was accepted as an address", in, got)
		}
		if got == (common.Address{}) {
			t.Errorf("partyAddress(%q) is the zero address; it must still derive a usable placeholder", in)
		}
	}
}
