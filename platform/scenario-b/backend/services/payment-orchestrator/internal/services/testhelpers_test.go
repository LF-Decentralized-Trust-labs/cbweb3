// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB returns an in-memory SQLite gorm DB with the bridging schema migrated.
// Each call gets an isolated database (shared-cache disabled), so tests are hermetic.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&podmain.BridgedAssetPosition{}, &podmain.RelayerQueueItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// --- fakes implementing the service collaborator interfaces ---

// fakeLockCaller implements SpokeBridgeLockCaller.
type fakeLockCaller struct {
	txHash string
	err    error
	calls  int
	gotAmt string
}

func (f *fakeLockCaller) LockAsset(_ context.Context, _, _, _, amount string) (*podmain.LockResult, error) {
	f.calls++
	f.gotAmt = amount
	if f.err != nil {
		return nil, f.err
	}
	return &podmain.LockResult{TxHash: f.txHash}, nil
}

// fakeLockRelayer implements RelayerSubmitter.
type fakeLockRelayer struct {
	err   error
	calls int
	keys  []string
}

func (f *fakeLockRelayer) SubmitLockEvent(_ context.Context, idempotencyKey, _ string) error {
	f.calls++
	f.keys = append(f.keys, idempotencyKey)
	return f.err
}

// fakeUnlockCaller implements SpokeBridgeUnlockCaller.
type fakeUnlockCaller struct {
	txHash string
	err    error
	calls  int
}

func (f *fakeUnlockCaller) UnlockAsset(_ context.Context, _, _, _, _ string) (*podmain.UnlockResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &podmain.UnlockResult{TxHash: f.txHash}, nil
}

// fakeBurnRelayer implements BurnRelayerSubmitter.
type fakeBurnRelayer struct {
	err   error
	calls int
	keys  []string
}

func (f *fakeBurnRelayer) SubmitBurnEvent(_ context.Context, idempotencyKey, _ string) error {
	f.calls++
	f.keys = append(f.keys, idempotencyKey)
	return f.err
}

func countPositions(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&podmain.BridgedAssetPosition{}).Count(&n).Error; err != nil {
		t.Fatalf("count positions: %v", err)
	}
	return n
}

func countQueueItems(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&podmain.RelayerQueueItem{}).Count(&n).Error; err != nil {
		t.Fatalf("count queue items: %v", err)
	}
	return n
}
