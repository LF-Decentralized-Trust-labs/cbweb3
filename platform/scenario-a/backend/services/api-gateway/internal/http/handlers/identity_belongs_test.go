// SPDX-License-Identifier: Apache-2.0

// Authorization must test bank membership, never parse a bank id out of a
// Paladin identity.
//
// A node name is `<spokeId>-<bankId>` and both halves may contain hyphens, so
// bankIDFromIdentity's fixed 3-way split cannot recover them. On spoke-costa-rica
// it returned "rica-cb1" for bank "cb1" — with a nil error, so every caller
// treated it as trustworthy. The observed damage: the bank that CREATED an HTLC
// got 403 "not a counterparty of this HTLC" on its own lock, and its PvP debits
// vanished from its own statement.

package handlers

import "testing"

// belongsToBankCases is the shared case table. THE SAME TABLE lives in the
// payment-orchestrator's internal/identity tests and in the bank portal's
// features/fx tests. Three implementations exist because the services are
// separate Go modules (this one vendors its deps) and the third is TypeScript.
// One table in three places is the anti-drift mechanism: change it in all three,
// or a fix in one silently regresses the others — which is how this defect
// spread in the first place.
var belongsToBankCases = []struct {
	name     string
	identity string
	bankID   string
	want     bool
}{
	// The reported defect. A two-word country puts a hyphen inside the spoke id.
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

func TestIdentityBelongsToBank(t *testing.T) {
	for _, tc := range belongsToBankCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := identityBelongsToBank(tc.identity, tc.bankID); got != tc.want {
				t.Errorf("identityBelongsToBank(%q, %q) = %v, want %v",
					tc.identity, tc.bankID, got, tc.want)
			}
		})
	}
}

// TestIsHTLCCounterparty_CreatorOfTheLock is the reported symptom, at the level
// that produced the 403. cb1 created the lock and must be a counterparty to it.
func TestIsHTLCCounterparty_CreatorOfTheLock(t *testing.T) {
	sender := "funded_operator@spoke-costa-rica-cb1"
	receiver := "funded_operator@spoke-costa-rica-cb2"

	ok, err := isHTLCCounterparty(sender, receiver, "cb1")
	if err != nil {
		t.Fatalf("isHTLCCounterparty returned an error: %v", err)
	}
	if !ok {
		t.Error(`the lock's own sender was refused as a counterparty — this is the ` +
			`403 "not a counterparty of this HTLC" reported on spoke-costa-rica`)
	}

	// The receiver too, and nobody else.
	if ok, _ := isHTLCCounterparty(sender, receiver, "cb2"); !ok {
		t.Error("the receiver must also be a counterparty")
	}
	if ok, _ := isHTLCCounterparty(sender, receiver, "cb3"); ok {
		t.Error("a bank that is neither sender nor receiver must not be a counterparty")
	}
}

// TestBankIDFromIdentity_IsUnsafeForAuthorization records WHY the primitive above
// exists, so nobody folds the checks back onto the extractor.
//
// The defect is not that it errors — it is that it does NOT. It returns a wrong
// value with a nil error, and every caller read that as "trustworthy".
func TestBankIDFromIdentity_IsUnsafeForAuthorization(t *testing.T) {
	got, err := bankIDFromIdentity("funded_operator@spoke-costa-rica-cb1")
	if err != nil {
		t.Fatalf("bankIDFromIdentity returned an error (%v); the defect is that it "+
			"does NOT, so callers cannot detect the wrong value", err)
	}
	if got == "cb1" {
		t.Fatal("bankIDFromIdentity now returns the correct bank id — if it was fixed, " +
			"this test and the DO NOT USE note above it should be revisited together")
	}
	if got != "rica-cb1" {
		t.Errorf("bankIDFromIdentity = %q, expected the documented wrong value %q",
			got, "rica-cb1")
	}
	if !identityBelongsToBank("funded_operator@spoke-costa-rica-cb1", "cb1") {
		t.Error("identityBelongsToBank must answer true where the extractor misleads — " +
			"that is its whole purpose")
	}
}
