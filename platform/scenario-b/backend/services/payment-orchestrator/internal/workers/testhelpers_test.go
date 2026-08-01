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

// testDBSeq keeps the per-test database name unique even when the same test name repeats (subtests,
// -count>1).
var testDBSeq atomic.Int64

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// An in-memory SQLite database belongs to its CONNECTION. With cache=private the pool can open a
	// second connection, which is a DIFFERENT, empty database — so AutoMigrate runs on one and the
	// worker's query lands on another, failing with "no such table: relayer_queue_items". That made
	// TestRelayerWorker_RunProcessesAndStops flaky (2 failures in 3 runs): the worker queries from its
	// own goroutine, which is exactly when a second connection is opened.
	//
	// cache=shared fixes that, but a shared cache is keyed by NAME — with a fixed name every test in
	// the package would share one database and collide on unique constraints (which is what happened
	// on the first attempt at this fix). So the name is unique per test, giving an isolated database
	// that all of that pool's connections can see.
	//
	// The pool is pinned to one connection as well: a shared-cache in-memory database is destroyed
	// when its LAST connection closes, so pool churn could otherwise drop it mid-test.
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()), testDBSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
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
type fakeFXRepo struct {
	expired     []*podmain.FXAgreementRecord
	listErr     error
	updateErr   map[string]error // tradeID -> err
	updated     []*podmain.FXAgreementRecord
	auditEvents []*podmain.FXAgreementEvent
	auditErr    error
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
	r.updated = append(r.updated, rec)
	return nil
}
func (r *fakeFXRepo) ListAgreements(context.Context, ports.FXAgreementFilter) ([]*podmain.FXAgreementRecord, error) {
	return nil, nil
}
func (r *fakeFXRepo) CreateAuditEvent(_ context.Context, e *podmain.FXAgreementEvent) error {
	if r.auditErr != nil {
		return r.auditErr
	}
	r.auditEvents = append(r.auditEvents, e)
	return nil
}
func (r *fakeFXRepo) ListAuditEvents(context.Context, string) ([]*podmain.FXAgreementEvent, error) {
	return nil, nil
}
func (r *fakeFXRepo) ListExpiredNonTerminal(_ context.Context, _ int64) ([]*podmain.FXAgreementRecord, error) {
	return r.expired, r.listErr
}

var _ ports.FXAgreementRepository = (*fakeFXRepo)(nil)
