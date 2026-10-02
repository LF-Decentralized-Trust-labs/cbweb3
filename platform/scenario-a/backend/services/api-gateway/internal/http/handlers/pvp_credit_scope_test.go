// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// pvpScopeCases is the shared case table for credit scoping: the identities of both
// live naming conventions, with the bank code each one really belongs to.
//
// The middle column is what the OLD code stored in receiver_bank_id (a positional
// split, SplitN(node,"-",3)[2]) and is kept here as the record of the defect: for a
// two-word country it disagrees with the bank's configured code, so the query
// `receiver_bank_id = 'cb1'` found nothing and a Costa Rica bank saw no credit for
// value it had received.
var pvpScopeCases = []struct {
	identity  string
	bankCode  string // BANK_CODE, from the manifest's spec.bankId — what the read asks for
	oldStored string // what the positional split wrote; "" when it errored outright
}{
	{"funded_operator@spoke-brl-bank-itau", "bank-itau", "bank-itau"},
	{"funded_operator@spoke-cop-bank-bancolombia", "bank-bancolombia", "bank-bancolombia"},
	{"funded_operator@spoke-cop-bank-davivienda", "bank-davivienda", "bank-davivienda"},
	{"funded_operator@spoke-chile-cb3", "cb3", "cb3"},
	{"funded_operator@spoke-peru-cb5", "cb5", "cb5"},
	{"funded_operator@spoke-costa-rica-cb1", "cb1", "rica-cb1"}, // the defect
	{"funded_operator@spoke-costa-rica-cb2", "cb2", "rica-cb2"}, // the defect
	{"funded_operator@spoke-brl-cb", "cb", "cb"},
}

// likeSuffix emulates the SQL patterns CreditScopePatterns produces. Both are of the
// form "%<literal>", i.e. a plain suffix test, so this is an exact stand-in.
func likeSuffix(value, pattern string) bool {
	return strings.HasSuffix(value, strings.TrimPrefix(pattern, "%"))
}

func scopeMatches(identity, bankID string) bool {
	pat := services.CreditScopePatterns(bankID)
	return likeSuffix(identity, pat[0]) || likeSuffix(identity, pat[1])
}

// TestCreditScopePatterns_AreASupersetOfThePredicate is the drift guard between the
// SQL narrowing and the Go authority. The query may over-fetch; it must never miss a
// row the predicate would accept, because a missed row is a credit the receiving bank
// never sees.
func TestCreditScopePatterns_AreASupersetOfThePredicate(t *testing.T) {
	for _, tc := range pvpScopeCases {
		if !identityBelongsToBank(tc.identity, tc.bankCode) {
			t.Fatalf("table is wrong: %q should belong to %q", tc.identity, tc.bankCode)
		}
		if !scopeMatches(tc.identity, tc.bankCode) {
			t.Errorf("SQL narrowing misses %q for bank %q — the predicate accepts it, so the row would be lost",
				tc.identity, tc.bankCode)
		}
	}
}

// TestCreditScope_RejectsTheOldStoredLabel demonstrates the defect directly: querying
// with the bank's real code against what the old writer stored returns nothing for a
// two-word country. It is the reason scoping moved off that column.
func TestCreditScope_RejectsTheOldStoredLabel(t *testing.T) {
	var diverged int
	for _, tc := range pvpScopeCases {
		if tc.oldStored == tc.bankCode {
			continue
		}
		diverged++
		// The old read was `receiver_bank_id = bankCode`, an exact match against the
		// stored label — and the label differs here, so it returned zero rows. The new
		// read tests the identity instead, and must find the credit.
		if !identityBelongsToBank(tc.identity, tc.bankCode) || !scopeMatches(tc.identity, tc.bankCode) {
			t.Errorf("%q: the identity-based scope must find the credit for %q (old label was %q)",
				tc.identity, tc.bankCode, tc.oldStored)
		}
	}
	if diverged == 0 {
		t.Fatal("the table no longer contains a two-word-country case; the guard proves nothing")
	}
}

// TestCreditScope_DoesNotLeakAcrossBanks is the other direction: over-fetching is
// tolerated only because the Go predicate rejects it. A bank must never be served
// another bank's credit.
func TestCreditScope_DoesNotLeakAcrossBanks(t *testing.T) {
	for _, tc := range pvpScopeCases {
		for _, other := range pvpScopeCases {
			if other.bankCode == tc.bankCode {
				continue
			}
			if identityBelongsToBank(tc.identity, other.bankCode) {
				t.Errorf("%q must not belong to %q", tc.identity, other.bankCode)
			}
		}
	}
	// Substring spoofing: a prefix of a real bank code must not match.
	for _, bad := range []string{"cb", "cb1", "bank", "rica-cb1"} {
		if identityBelongsToBank("funded_operator@spoke-costa-rica-cb11", bad) {
			t.Errorf("cb11's identity must not belong to %q", bad)
		}
	}
}
