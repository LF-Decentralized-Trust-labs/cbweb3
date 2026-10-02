// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// A beneficiary bank cannot see the payment it received: the delivery position is created on
// its CENTRAL BANK's gateway, and the bank's portal asks its own gateway, which has no rows.
// Exposing the CB's listing to a bank is what closes that — but the existing ListPositions
// returns every bank's rows, so it cannot be exposed as it stands. This is the scoped read
// that can be.

func seedPosition(t *testing.T, owner, id, state string) *domain.BridgedAssetPosition {
	t.Helper()
	return &domain.BridgedAssetPosition{
		PositionID:     id,
		OwnerBankID:    owner,
		SpokeNetwork:   "spoke-ars",
		NativeAsset:    "0xnative",
		MirroredAsset:  "0xmirrored",
		MirroredAmount: "50000000000000000000",
		BridgeState:    domain.BridgeState(state),
	}
}

func TestListPositionsForOwner_ReturnsOnlyThatBanksRows(t *testing.T) {
	db := newBridgeDB(t)
	for _, p := range []*domain.BridgedAssetPosition{
		seedPosition(t, "bank-macro", "pos-macro-1", "RELEASED"),
		seedPosition(t, "bank-galicia", "pos-galicia-1", "RELEASED"),
		seedPosition(t, "bank-macro", "pos-macro-2", "LOCKING"),
	} {
		if err := db.Create(p).Error; err != nil {
			t.Fatalf("seed %s: %v", p.PositionID, err)
		}
	}

	got, err := NewBridgePositionReader(db).ListPositionsForOwner(context.Background(), "bank-macro", "")
	if err != nil {
		t.Fatalf("ListPositionsForOwner: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d positions, want 2 — the listing must return this bank's rows and no others", len(got))
	}
	for _, p := range got {
		if p.OwnerBankID != "bank-macro" {
			t.Errorf("returned a position owned by %q; a bank must never see another's", p.OwnerBankID)
		}
	}
}

func TestListPositionsForOwner_StillHonoursTheStateFilter(t *testing.T) {
	db := newBridgeDB(t)
	for _, p := range []*domain.BridgedAssetPosition{
		seedPosition(t, "bank-macro", "pos-1", "RELEASED"),
		seedPosition(t, "bank-macro", "pos-2", "LOCKING"),
	} {
		if err := db.Create(p).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := NewBridgePositionReader(db).ListPositionsForOwner(context.Background(), "bank-macro", "RELEASED")
	if err != nil {
		t.Fatalf("ListPositionsForOwner: %v", err)
	}
	if len(got) != 1 || got[0].PositionID != "pos-1" {
		t.Fatalf("state filter did not apply: got %+v", got)
	}
}

// TestListPositionsForOwner_RefusesAnEmptyOwner is the load-bearing one. An empty owner must
// not degrade into "every row": this method exists precisely to be reachable by a commercial
// bank, so a silent fall-through would hand one bank the whole book.
func TestListPositionsForOwner_RefusesAnEmptyOwner(t *testing.T) {
	db := newBridgeDB(t)
	if err := db.Create(seedPosition(t, "bank-macro", "pos-1", "RELEASED")).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := NewBridgePositionReader(db).ListPositionsForOwner(context.Background(), "  ", "")
	if err == nil {
		t.Fatalf("an empty owner returned %d rows instead of an error", len(got))
	}
	if len(got) != 0 {
		t.Errorf("an empty owner must return nothing, got %d rows", len(got))
	}
}
