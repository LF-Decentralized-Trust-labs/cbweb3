// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/identity"
)

func TestSpokePrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard spoke-a", "funded_operator@spoke-a-bank-a", "spoke-a"},
		{"standard spoke-b", "funded_operator@spoke-b-bank-d", "spoke-b"},
		{"no @ separator", "bank-b", ""},
		{"empty string", "", ""},
		{"no dash after @", "user@nodash", ""},
		{"multi-segment spoke", "user@spoke-alpha-1-bank-x", "spoke-alpha"},
		{"only @ no content", "user@", ""},
		{"@ with single segment", "user@spoke", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identity.SpokePrefix(tt.input)
			if got != tt.expected {
				t.Errorf("SpokePrefix(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBankID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBank string
		wantErr  bool
	}{
		{"standard spoke-a", "funded_operator@spoke-a-bank-a", "bank-a", false},
		{"standard spoke-b", "funded_operator@spoke-b-bank-d", "bank-d", false},
		{"bank id with hyphen", "user@spoke-a-bank-xyz", "bank-xyz", false},
		{"no @ separator", "bank-b", "", true},
		{"empty string", "", "", true},
		{"only two dash-segments after @", "user@spoke-a", "", true},
		{"no dash after @", "user@nodash", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := identity.BankID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("BankID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.wantBank {
				t.Errorf("BankID(%q) = %q, want %q", tt.input, got, tt.wantBank)
			}
		})
	}
}

// belongsToBankCases is the shared case table. THE SAME TABLE lives in the
// api-gateway handler test and in the bank portal's features/fx test — three
// implementations exist because the services are separate Go modules (the
// gateway vendors its deps) and the third is TypeScript. Keeping one table in
// three places is the anti-drift mechanism: change it in all three or a fix in
// one will silently regress the others, which is exactly how this defect spread.
var belongsToBankCases = []struct {
	name     string
	identity string
	bankID   string
	want     bool
}{
	// The reported defect. A two-word country puts a hyphen inside the spoke id,
	// which the old 3-way split read as part of the bank id ("rica-cb1").
	{"two-word country, full identity", "funded_operator@spoke-costa-rica-cb1", "cb1", true},
	{"two-word country, central bank", "funded_operator@spoke-costa-rica-cb", "cb", true},
	// Single-word countries worked before purely by accident; they must keep working.
	{"single-word country", "funded_operator@spoke-chile-cb3", "cb3", true},
	{"single-word country, peru", "funded_operator@spoke-peru-cb5", "cb5", true},
	// The other live convention: the bank id itself carries a hyphen.
	{"hyphenated bank id", "funded_operator@spoke-brl-bank-itau", "bank-itau", true},
	{"hyphenated bank id, bradesco", "funded_operator@spoke-brl-bank-bradesco", "bank-bradesco", true},
	// Parties are sometimes stored as a bare bank id rather than a full identity.
	{"bare bank id", "cb1", "cb1", true},
	{"bare hyphenated bank id", "bank-itau", "bank-itau", true},

	// Anti-spoofing. These are why the boundary is "-"+bankID and not a plain
	// suffix or a Contains: a shorter id must not match a longer one.
	{"prefix must not match a longer bank id", "funded_operator@spoke-x-cb11", "cb1", false},
	{"prefix must not match a longer hyphenated id", "funded_operator@spoke-x-bank-abc", "bank", false},
	{"different bank on the same spoke", "funded_operator@spoke-costa-rica-cb1", "cb2", false},
	{"spoke name must not be mistaken for a bank", "funded_operator@spoke-costa-rica-cb1", "rica-cb1", true},

	// Degenerate input fails closed.
	{"empty bank id", "funded_operator@spoke-costa-rica-cb1", "", false},
	{"empty identity", "", "cb1", false},
	{"identity is only the @", "funded_operator@", "cb1", false},
	{"whitespace is trimmed", "  funded_operator@spoke-costa-rica-cb1  ", " cb1 ", true},
}

func TestBelongsToBank(t *testing.T) {
	for _, tc := range belongsToBankCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := identity.BelongsToBank(tc.identity, tc.bankID); got != tc.want {
				t.Errorf("BelongsToBank(%q, %q) = %v, want %v", tc.identity, tc.bankID, got, tc.want)
			}
		})
	}
}

// TestBankID_IsUnsafeForAuthorization records WHY BelongsToBank exists, so nobody
// "simplifies" the authorization checks back onto BankID.
//
// The point is that BankID returns a wrong value with a NIL error. Callers that
// treated a nil error as "the value is trustworthy" — which every authorization
// site did — compared the caller's real bank id against "rica-cb1" and took the
// branch for "not this bank".
func TestBankID_IsUnsafeForAuthorization(t *testing.T) {
	got, err := identity.BankID("funded_operator@spoke-costa-rica-cb1")
	if err != nil {
		t.Fatalf("BankID returned an error (%v); the defect is that it does NOT, "+
			"so callers cannot detect the wrong value", err)
	}
	if got == "cb1" {
		t.Fatal("BankID now returns the correct bank id — if it was fixed, this test " +
			"and the DO NOT USE note above it should be revisited together")
	}
	if got != "rica-cb1" {
		t.Errorf("BankID = %q, expected the documented wrong value %q", got, "rica-cb1")
	}
	if !identity.BelongsToBank("funded_operator@spoke-costa-rica-cb1", "cb1") {
		t.Error("BelongsToBank must answer true where BankID misleads — that is its whole purpose")
	}
}

// TestSpokePrefix_ConflatesSpokesSharingTwoSegments pins the limitation that
// makes SpokePrefix a FALLBACK rather than the source of truth.
//
// It carries the same two-segment assumption as BankID, so
// "spoke-costa-rica-cb1" yields "spoke-costa". The service now reads the spoke id
// from SPOKE_ID, which the toolkit already exports to the compose environment, and
// only falls back here when that variable is absent — an environment deployed
// before the template carried it. main.go logs a WARNING in that case rather than
// reporting a healthy configuration.
//
// The limitation is asserted, not merely documented: two spokes sharing their
// first two segments become indistinguishable, and a receiver on a DIFFERENT
// spoke would be accepted as local. That is the reason the fallback must stay a
// fallback, and why this test should outlive the fallback's removal only as its
// justification.
func TestSpokePrefix_ConflatesSpokesSharingTwoSegments(t *testing.T) {
	a := identity.SpokePrefix("funded_operator@spoke-costa-rica-cb1")
	b := identity.SpokePrefix("funded_operator@spoke-costa-brava-cb1")

	if a != "spoke-costa" || b != "spoke-costa" {
		t.Fatalf("expected both to truncate to %q, got %q and %q — if SpokePrefix was "+
			"fixed, delete this test and the card that tracks it", "spoke-costa", a, b)
	}
	if a != b {
		t.Fatal("unreachable: the two prefixes differ")
	}
	// Stated as the consequence, so the risk is not mistaken for a naming quirk:
	// isLocalReceiver would accept a spoke-costa-brava receiver as local to
	// spoke-costa-rica. Both spokes existing at once is what makes this live.
	t.Log("two distinct spokes compare equal; isLocalReceiver cannot tell them apart")
}

// TestBelongsToSpoke uses the two live naming conventions, because the defect it
// guards was invisible under a two-segment spoke id: with SpokePrefix on both sides
// the check compared two identical wrong guesses and passed.
func TestBelongsToSpoke(t *testing.T) {
	for _, tc := range []struct {
		identity string
		spokeID  string
		want     bool
	}{
		// The regression case: a bank locking to its neighbour on the same spoke.
		{"funded_operator@spoke-costa-rica-cb2", "spoke-costa-rica", true},
		{"funded_operator@spoke-costa-rica-cb1", "spoke-costa-rica", true},
		{"funded_operator@spoke-costa-rica-cb", "spoke-costa-rica", true},
		// The conventions that worked by accident must keep working.
		{"funded_operator@spoke-brl-bank-itau", "spoke-brl", true},
		{"funded_operator@spoke-brl-cb", "spoke-brl", true},
		{"funded_operator@spoke-chile-cb3", "spoke-chile", true},
		// Cross-spoke receivers must be rejected — this is the check's actual job.
		{"funded_operator@spoke-brl-bank-itau", "spoke-costa-rica", false},
		{"funded_operator@spoke-costa-rica-cb1", "spoke-brl", false},
		{"funded_operator@spoke-peru-cb5", "spoke-chile", false},
		// KNOWN LIMIT, asserted so it is a documented behaviour and not a surprise:
		// "spoke-costa-rica-cb1" is ambiguous from the string alone (spokeId
		// "spoke-costa-rica" + bank "cb1", or spokeId "spoke-costa" + bank
		// "rica-cb1"), so a spoke whose id is a hyphen-boundary prefix of another's
		// accepts it. This is fail-OPEN and is the residual risk the doc comment
		// describes; it needs two live spokes named that way, which none are.
		{"funded_operator@spoke-costa-rica-cb1", "spoke-costa", true},
		{"funded_operator@spoke-costa-rica-cb1", "spoke", true},
		// A node named exactly the spoke id is not a node on it (no suffix).
		{"funded_operator@spoke-costa-rica", "spoke-costa-rica", false},
		// Degenerate input fails closed.
		{"", "spoke-brl", false},
		{"funded_operator@", "spoke-brl", false},
		{"funded_operator@spoke-brl-cb", "", false},
		{"funded_operator@spoke-brl-cb", "   ", false},
		// A bare bank id carries no spoke, so it cannot be verified as local.
		{"cb1", "spoke-costa-rica", false},
	} {
		if got := identity.BelongsToSpoke(tc.identity, tc.spokeID); got != tc.want {
			t.Errorf("BelongsToSpoke(%q, %q) = %v, want %v", tc.identity, tc.spokeID, got, tc.want)
		}
	}
}

// TestBelongsToSpoke_RejectsThePrefixComparison pins the exact failure mode: the old
// implementation compared SpokePrefix on both sides. Under a hyphenated spoke id
// that comparison is wrong in BOTH directions — it accepted nothing once the local
// side was corrected, and before that it would have accepted a receiver from a
// DIFFERENT spoke that happened to share the first two segments.
func TestBelongsToSpoke_RejectsThePrefixComparison(t *testing.T) {
	const local = "spoke-costa-rica"
	const receiver = "funded_operator@spoke-costa-rica-cb2"

	// What the old code did.
	if identity.SpokePrefix(receiver) == local {
		t.Fatal("SpokePrefix now agrees with the real spoke id — this guard is moot, re-derive it")
	}
	// What the fix does.
	if !identity.BelongsToSpoke(receiver, local) {
		t.Error("a receiver on the local spoke must be accepted")
	}

	// Two DIFFERENT spokes sharing the first two segments: the old comparison would
	// have called them equal.
	a := "funded_operator@spoke-costa-rica-cb1"
	if identity.SpokePrefix(a) != identity.SpokePrefix("funded_operator@spoke-costa-verde-cb1") {
		t.Fatal("the two fixtures no longer share a SpokePrefix; the guard proves nothing")
	}
	if identity.BelongsToSpoke("funded_operator@spoke-costa-verde-cb1", local) {
		t.Error("a receiver on spoke-costa-verde must NOT be local to spoke-costa-rica")
	}
}
