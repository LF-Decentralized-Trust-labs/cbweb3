// SPDX-License-Identifier: Apache-2.0

package repository_test

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
)

func TestGormRelayWatermarkStore_GetSetUpsert(t *testing.T) {
	db := newTestDB(t)
	store, err := repository.NewGormRelayWatermarkStoreFromDB(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	// Absent watermark → found=false, no error.
	if _, found, err := store.GetWatermark(ctx, "settle"); err != nil || found {
		t.Fatalf("expected not-found, got found=%v err=%v", found, err)
	}

	// First write.
	if err := store.SetWatermark(ctx, "settle", 1_000); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, found, err := store.GetWatermark(ctx, "settle")
	if err != nil || !found || v != 1_000 {
		t.Fatalf("get after set: v=%d found=%v err=%v", v, found, err)
	}

	// Upsert (same kind) must overwrite, not insert a duplicate.
	if err := store.SetWatermark(ctx, "settle", 5_000); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	v, _, _ = store.GetWatermark(ctx, "settle")
	if v != 5_000 {
		t.Fatalf("value after upsert = %d, want 5000", v)
	}

	// Distinct kinds are tracked independently.
	if err := store.SetWatermark(ctx, "lock", 42); err != nil {
		t.Fatalf("set lock: %v", err)
	}
	if v, _, _ := store.GetWatermark(ctx, "lock"); v != 42 {
		t.Fatalf("lock value = %d, want 42", v)
	}
	if v, _, _ := store.GetWatermark(ctx, "settle"); v != 5_000 {
		t.Fatalf("settle value = %d, want 5000 (should be independent of lock)", v)
	}
}

// TestGormRelayWatermarkStore_SurvivesRestart proves the durability contract that
// backs the R2-H-11 fix: a watermark written by one store instance is visible to a
// fresh instance opened against the same database, exactly as after a process restart.
func TestGormRelayWatermarkStore_SurvivesRestart(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	first, err := repository.NewGormRelayWatermarkStoreFromDB(db)
	if err != nil {
		t.Fatalf("first store: %v", err)
	}
	if err := first.SetWatermark(ctx, "settle", 7_777); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Simulate a restart: a brand-new store over the same DB (same AutoMigrate).
	second, err := repository.NewGormRelayWatermarkStoreFromDB(db)
	if err != nil {
		t.Fatalf("second store: %v", err)
	}
	v, found, err := second.GetWatermark(ctx, "settle")
	if err != nil || !found {
		t.Fatalf("watermark not visible after restart: found=%v err=%v", found, err)
	}
	if v != 7_777 {
		t.Fatalf("watermark after restart = %d, want 7777", v)
	}
}
