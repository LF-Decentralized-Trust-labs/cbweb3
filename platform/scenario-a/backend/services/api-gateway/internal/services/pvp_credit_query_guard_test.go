// SPDX-License-Identifier: Apache-2.0

package services_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// This file is a STOPGAP. The credit query is the load-bearing part of the fix, and
// nothing here executes it: api-gateway has no database test harness, and adding one
// means a new dependency in a service that vendors, which is a team decision rather
// than a side effect of this change. Until that harness exists, the query shape is
// pinned by reading the source — the same technique the toolkit uses for compose
// templates. It is weaker than a behavioural test and should be replaced by one.
//
// What it defends: scoping a bank's credits by the DERIVED receiver_bank_id column,
// which is what made a Costa Rica bank blind to value it had received. The column is
// a label; the stored receiver identity is the fact.
const serviceSrc = "pvp_ledger_service.go"

// scopedOnDerivedColumn matches a WHERE clause that filters on receiver_bank_id.
// Deliberately shape-based rather than a substring test: the column name still
// appears legitimately in the struct comment and in the input field, and a guard that
// only looked for the name would pass on a real regression while failing on a comment.
var scopedOnDerivedColumn = regexp.MustCompile(`Where\([^)]*receiver_bank_id`)

func serviceSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(serviceSrc)
	if err != nil {
		t.Fatalf("read %s: %v", serviceSrc, err)
	}
	return string(b)
}

func TestCreditQuery_DoesNotScopeOnTheDerivedColumn(t *testing.T) {
	if loc := scopedOnDerivedColumn.FindString(serviceSource(t)); loc != "" {
		t.Errorf("the credit query scopes on the derived bank-id column: %q\n"+
			"That column is a best-effort label written by a positional split; it disagrees "+
			"with the bank's configured code whenever the spoke id contains a hyphen "+
			"(spoke-costa-rica-cb1 stored \"rica-cb1\" while the bank asks for \"cb1\"), and the "+
			"query returned zero rows. Scope on the stored receiver identity instead.", loc)
	}
}

func TestCreditQuery_ScopesOnTheStoredIdentity(t *testing.T) {
	src := serviceSource(t)
	if !strings.Contains(src, "CreditScopePatterns(bankID)") {
		t.Error("the credit query no longer builds its scope from CreditScopePatterns — " +
			"the SQL narrowing and the Go predicate must stay derived from one place")
	}
	if !regexp.MustCompile(`Where\("receiver LIKE \? OR receiver LIKE \?"`).MatchString(src) {
		t.Error("the credit query no longer narrows on the receiver identity")
	}
}

// TestGuard_CatchesTheRegression is the guard's own test: it must fire on the exact
// line the old code had, and stay quiet for the mentions of the column that are fine.
func TestGuard_CatchesTheRegression(t *testing.T) {
	regression := `Where("receiver_bank_id = ?", bankID).`
	if !scopedOnDerivedColumn.MatchString(regression) {
		t.Errorf("the guard does not catch the original query: %q", regression)
	}
	for _, benign := range []string{
		"// ReceiverBankID is a best-effort label; see receiver_bank_id in the domain model",
		"`gorm:\"column:receiver_bank_id;index\"`",
		`Where("receiver LIKE ? OR receiver LIKE ?", pat[0], pat[1])`,
	} {
		if scopedOnDerivedColumn.MatchString(benign) {
			t.Errorf("the guard false-positives on a benign line: %q", benign)
		}
	}
}
