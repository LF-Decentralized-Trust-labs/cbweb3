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
		ContractID:     "htlc-1",
		ReceiverBankID: "bank-b",
	}); err == nil {
		t.Error("RecordLeg with nil db must return an error")
	}

	if _, err := svc.ListCreditsForBank(ctx, "bank-b"); err == nil {
		t.Error("ListCreditsForBank with nil db must return an error")
	}
}
