// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDBCounter guarantees a globally unique in-memory database name even when
// two tests share a sanitized name or run concurrently.
var testDBCounter atomic.Uint64

// newTestDB returns a fully isolated, freshly migrated SQLite database for a
// single test.
//
// Isolation is enforced on two axes:
//   - A unique shared-cache in-memory DSN per test (mode=memory&cache=shared
//     with a name derived from the test name + a monotonic counter). Shared
//     cache means every pooled connection sees the same database, so a query
//     never lands on a fresh, empty connection — the root cause of the
//     intermittent "no such table: relayer_queue_items" under -coverpkg /
//     -covermode=atomic. A per-test name prevents cross-test state leakage.
//   - The connection pool is pinned to a single connection. A shared-cache
//     in-memory database is destroyed when its last connection closes, so
//     pinning MaxOpenConns/MaxIdleConns to 1 keeps the migrated schema alive
//     for the whole test and removes any pool-ordering nondeterminism.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	safeName := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, t.Name())
	dsn := fmt.Sprintf("file:memdb_%s_%d?mode=memory&cache=shared", safeName, testDBCounter.Add(1))

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	// Keep the shared-cache in-memory database alive for the test's lifetime
	// and eliminate pool-ordering flakiness.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&podmain.BridgedAssetPosition{}, &podmain.RelayerQueueItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// fakeExecutor implements RelayerEventExecutor with scripted per-position outcomes.
type fakeExecutor struct {
	mu        sync.Mutex
	lockErr   map[string]error // positionID -> err to return on SubmitLockEvent
	burnErr   map[string]error
	lockCalls map[string]int
	burnCalls map[string]int
}

func newFakeExecutor() *fakeExecutor {
	return &fakeExecutor{
		lockErr:   map[string]error{},
		burnErr:   map[string]error{},
		lockCalls: map[string]int{},
		burnCalls: map[string]int{},
	}
}

func (f *fakeExecutor) SubmitLockEvent(_ context.Context, _, positionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lockCalls[positionID]++
	return f.lockErr[positionID]
}

func (f *fakeExecutor) SubmitBurnEvent(_ context.Context, _, positionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.burnCalls[positionID]++
	return f.burnErr[positionID]
}

var _ RelayerEventExecutor = (*fakeExecutor)(nil)

// fakeFXRepo implements ports.FXAgreementRepository for worker tests.
//
// `expired`, `listErr`, `updateErr` and `auditErr` are set before the worker
// starts and only read afterwards. `updated` and `auditEvents` are appended to
// from the worker goroutine while the test polls them, so those two — and only
// those two — are guarded by mu. Read them via numUpdated / snapshotUpdated in
// concurrent tests.
type fakeFXRepo struct {
	expired   []*podmain.FXAgreementRecord
	listErr   error
	updateErr map[string]error // tradeID -> err
	auditErr  error

	mu          sync.Mutex
	updated     []*podmain.FXAgreementRecord
	auditEvents []*podmain.FXAgreementEvent
}

// numUpdated returns the count of recorded updates under the lock.
func (r *fakeFXRepo) numUpdated() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.updated)
}

func (r *fakeFXRepo) CreateAgreement(context.Context, *podmain.FXAgreementRecord) error { return nil }
func (r *fakeFXRepo) GetAgreement(context.Context, string) (*podmain.FXAgreementRecord, error) {
	return nil, nil
}
func (r *fakeFXRepo) UpdateAgreement(_ context.Context, rec *podmain.FXAgreementRecord) error {
	if r.updateErr != nil {
		if err := r.updateErr[rec.TradeID]; err != nil {
			return err
		}
	}
	r.mu.Lock()
	r.updated = append(r.updated, rec)
	r.mu.Unlock()
	return nil
}
func (r *fakeFXRepo) ListAgreements(context.Context, ports.FXAgreementFilter) ([]*podmain.FXAgreementRecord, error) {
	return nil, nil
}
func (r *fakeFXRepo) CreateAuditEvent(_ context.Context, e *podmain.FXAgreementEvent) error {
	if r.auditErr != nil {
		return r.auditErr
	}
	r.mu.Lock()
	r.auditEvents = append(r.auditEvents, e)
	r.mu.Unlock()
	return nil
}
func (r *fakeFXRepo) ListAuditEvents(context.Context, string) ([]*podmain.FXAgreementEvent, error) {
	return nil, nil
}
func (r *fakeFXRepo) ListExpiredNonTerminal(_ context.Context, _ int64) ([]*podmain.FXAgreementRecord, error) {
	return r.expired, r.listErr
}

var _ ports.FXAgreementRepository = (*fakeFXRepo)(nil)
