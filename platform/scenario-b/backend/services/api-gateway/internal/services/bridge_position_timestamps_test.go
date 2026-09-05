// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// The second half of "the beneficiary cannot see the payment it received".
//
// Reaching the bank's portal was not enough: the dashboard drops any activity row without a
// timestamp (`items.filter(i => i.ts)`), and the position DTO carried none. So the listing
// answered 200 with the payment in it and the screen still read "No activity yet" — the same
// symptom, one layer further along, and invisible from the API alone.

func TestPositionResult_CarriesTheTimestampsTheActivityFeedNeeds(t *testing.T) {
	db := newBridgeDB(t)
	pos := seedPosition(t, "bank-macro", "pos-1", "RELEASED")
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := NewBridgePositionReader(db).ListPositionsForOwner(context.Background(), "bank-macro", "")
	if err != nil {
		t.Fatalf("ListPositionsForOwner: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d positions, want 1", len(got))
	}

	if got[0].CreatedAt.IsZero() {
		t.Error("created_at is zero — the portal has nothing to order or date the payment by")
	}
	if got[0].UpdatedAt.IsZero() {
		t.Error("updated_at is zero — the activity feed reads this field and drops rows without it, " +
			"so the beneficiary's payment stays invisible even though the listing returns it")
	}
}

// TestPositionResult_UpdatedAtTracksTheLastChange keeps the field meaningful: a delivery moves
// through states, and an operator dating a payment by when it was created rather than when it
// settled is being told the wrong thing.
func TestPositionResult_UpdatedAtTracksTheLastChange(t *testing.T) {
	db := newBridgeDB(t)
	pos := seedPosition(t, "bank-macro", "pos-1", "LOCKING")
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := db.Model(&domain.BridgedAssetPosition{}).
		Where("position_id = ?", "pos-1").
		Update("bridge_state", "RELEASED").Error; err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := NewBridgePositionReader(db).ListPositionsForOwner(context.Background(), "bank-macro", "")
	if err != nil {
		t.Fatalf("ListPositionsForOwner: %v", err)
	}
	if !got[0].UpdatedAt.After(got[0].CreatedAt) {
		t.Errorf("updated_at (%v) did not move past created_at (%v) after a state change",
			got[0].UpdatedAt, got[0].CreatedAt)
	}
}
