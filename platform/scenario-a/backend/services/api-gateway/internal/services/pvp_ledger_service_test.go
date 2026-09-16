// SPDX-License-Identifier: Apache-2.0

package services_test

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// TestPvPLedgerService_Constructor confirms the constructor returns a non-nil service.
func TestPvPLedgerService_Constructor(t *testing.T) {
	if services.NewPvPLedgerService(nil) == nil {
		t.Fatal("NewPvPLedgerService must return non-nil")
	}
}

// TestPvPLedgerService_NoDatabase asserts the nil-db guards: with no database
// configured, RecordLeg and ListCreditsForBank must fail fast rather than panic.
func TestPvPLedgerService_NoDatabase(t *testing.T) {
	svc := services.NewPvPLedgerService(nil)
	ctx := context.Background()

	if err := svc.RecordLeg(ctx, services.SettledLegInput{
		ContractID: "htlc-1",
		Receiver:   "op@spoke-a-bank-b",
	}); err == nil {
		t.Error("RecordLeg with nil db must return an error")
	}

	belongs := func(string) bool { return true }
	if _, err := svc.ListCreditsForBank(ctx, "bank-b", belongs); err == nil {
		t.Error("ListCreditsForBank with nil db must return an error")
	}
}

// TestPvPLedgerService_ListCreditsFailsClosed asserts the scope guards. An unscoped
// read of the Central Bank's ledger would hand one bank every other bank's
// settlements, so a missing bank id or a missing predicate must be an error, never an
// empty scope that matches everything.
func TestPvPLedgerService_ListCreditsFailsClosed(t *testing.T) {
	svc := services.NewPvPLedgerService(nil)
	ctx := context.Background()
	belongs := func(string) bool { return true }

	for _, tc := range []struct {
		name    string
		bankID  string
		belongs func(string) bool
	}{
		{"empty bank id", "", belongs},
		{"blank bank id", "   ", belongs},
		{"nil predicate", "cb1", nil},
	} {
		if _, err := svc.ListCreditsForBank(ctx, tc.bankID, tc.belongs); err == nil {
			t.Errorf("%s: must return an error rather than an unscoped read", tc.name)
		}
	}
}

// TestCreditScopePatterns_Shape pins the two patterns the query is built from. They
// must be suffix matches, because the identity is "<key>@<node>" and the bank id sits
// at its end; a prefix or a bare "%bankID%" would match another bank's node.
func TestCreditScopePatterns_Shape(t *testing.T) {
	got := services.CreditScopePatterns(" cb1 ")
	want := [2]string{"%@cb1", "%-cb1"}
	if got != want {
		t.Errorf("CreditScopePatterns = %q, want %q (and the bank id must be trimmed)", got, want)
	}
}
