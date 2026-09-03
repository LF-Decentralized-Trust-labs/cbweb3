// SPDX-License-Identifier: Apache-2.0

package repository_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
)

// testLogger returns a discarded slog.Logger for tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestDB returns an in-memory SQLite *gorm.DB. The pure-Go (modernc) driver
// is used so the suite stays hermetic and CGO-free.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Each test gets a private connection pool so shared-cache memory DBs do not
	// collide across tests.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// --- Escrow GORM repository ---

func TestGormEscrowRepository_DepositLifecycle(t *testing.T) {
	repo, err := repository.NewGormEscrowRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()

	rec := domain.DepositRecord{
		ID: "d1", RequesterID: "bank-a", RequesterBesuAddress: "0xabc",
		RequesterPaladinIdentity: "op@spoke-a-bank-a", Amount: "100",
		Status: domain.DepositStatusPending, CreatedAt: time.Now().UTC(),
	}
	if err := repo.CreateDeposit(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, found, err := repo.GetDeposit(ctx, "d1")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got.Amount != "100" || got.Status != domain.DepositStatusPending {
		t.Errorf("unexpected record: %+v", got)
	}

	// Not-found path.
	_, found, err = repo.GetDeposit(ctx, "missing")
	if err != nil || found {
		t.Errorf("expected not found, got found=%v err=%v", found, err)
	}

	rec.Status = domain.DepositStatusApproved
	rec.MintTxHash = "0xmint"
	if err := repo.UpdateDeposit(ctx, rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _, _ = repo.GetDeposit(ctx, "d1")
	if got.Status != domain.DepositStatusApproved || got.MintTxHash != "0xmint" {
		t.Errorf("update not persisted: %+v", got)
	}

	// Second deposit for a different requester to exercise the filter.
	_ = repo.CreateDeposit(ctx, domain.DepositRecord{
		ID: "d2", RequesterID: "bank-b", RequesterBesuAddress: "0xdef",
		RequesterPaladinIdentity: "op@spoke-a-bank-b", Amount: "5",
		Status: domain.DepositStatusPending, CreatedAt: time.Now().UTC(),
	})
	all, err := repo.ListDeposits(ctx, "")
	if err != nil || len(all) != 2 {
		t.Errorf("list all: len=%d err=%v", len(all), err)
	}
	filtered, _ := repo.ListDeposits(ctx, "bank-a")
	if len(filtered) != 1 || filtered[0].ID != "d1" {
		t.Errorf("filtered list = %+v", filtered)
	}
}

func TestGormEscrowRepository_EscrowLifecycle(t *testing.T) {
	repo, err := repository.NewGormEscrowRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()

	rec := domain.EscrowRecord{
		ID: "e1", RequesterID: "bank-a", RequesterBesuAddress: "0xabc",
		RequesterPaladinIdentity: "op@spoke-a-bank-a", Amount: "200",
		Status: domain.EscrowStatusPending, CreatedAt: time.Now().UTC(),
	}
	if err := repo.CreateEscrow(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, found, _ := repo.GetEscrow(ctx, "e1")
	if !found {
		t.Fatal("escrow not found after create")
	}
	_, found, _ = repo.GetEscrow(ctx, "nope")
	if found {
		t.Fatal("expected not-found")
	}

	rec.Status = domain.EscrowStatusApproved
	rec.BurnTxHash = "0xburn"
	rec.MintTxHash = "0xmint"
	if err := repo.UpdateEscrow(ctx, rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _, _ := repo.GetEscrow(ctx, "e1")
	if got.BurnTxHash != "0xburn" || got.Status != domain.EscrowStatusApproved {
		t.Errorf("update not persisted: %+v", got)
	}

	list, _ := repo.ListEscrows(ctx, "bank-a")
	if len(list) != 1 {
		t.Errorf("list = %d", len(list))
	}
	listAll, _ := repo.ListEscrows(ctx, "")
	if len(listAll) != 1 {
		t.Errorf("list all = %d", len(listAll))
	}
}

func TestGormEscrowRepository_RedeemLifecycle(t *testing.T) {
	repo, err := repository.NewGormEscrowRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()

	rec := domain.RedeemRecord{
		ID: "r1", RequesterID: "bank-a", RequesterBesuAddress: "0xabc",
		RequesterPaladinIdentity: "op@spoke-a-bank-a", Amount: "300",
		Status: domain.RedeemStatusPending, ZetoTransferTxHash: "0xzt",
		CreatedAt: time.Now().UTC(),
	}
	if err := repo.CreateRedeem(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, found, _ := repo.GetRedeem(ctx, "r1")
	if !found {
		t.Fatal("redeem not found after create")
	}
	_, found, _ = repo.GetRedeem(ctx, "nope")
	if found {
		t.Fatal("expected not-found")
	}

	rec.Status = domain.RedeemStatusApproved
	rec.FiatMintTxHash = "0xfm"
	if err := repo.UpdateRedeem(ctx, rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _, _ := repo.GetRedeem(ctx, "r1")
	if got.FiatMintTxHash != "0xfm" || got.Status != domain.RedeemStatusApproved {
		t.Errorf("update not persisted: %+v", got)
	}

	list, _ := repo.ListRedeems(ctx, "bank-a")
	if len(list) != 1 {
		t.Errorf("list = %d", len(list))
	}
	listAll, _ := repo.ListRedeems(ctx, "")
	if len(listAll) != 1 {
		t.Errorf("list all = %d", len(listAll))
	}
}

// --- HTLC GORM repository ---

func TestGormHTLCRepository_Lifecycle(t *testing.T) {
	repo, err := repository.NewGormHTLCRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()

	rec := &domain.HTLCRecord{
		ContractID: "c1", AgreementID: "FX1", Sender: "op@spoke-a-bank-a",
		Receiver: "op@spoke-a-bank-b", Amount: "100", HashLock: "hash1",
		TimeLock: 9999, Secret: "sec", ZetoLockRef: "ref",
		State: domain.HTLCStateLocked, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := repo.CreateHTLC(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetHTLC(ctx, "c1")
	if err != nil || got == nil || got.State != domain.HTLCStateLocked {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	// Not-found returns (nil, nil) per the postgres impl contract.
	missing, err := repo.GetHTLC(ctx, "missing")
	if err != nil || missing != nil {
		t.Errorf("expected (nil,nil), got %+v err=%v", missing, err)
	}

	byHash, err := repo.GetHTLCByHashLock(ctx, "hash1")
	if err != nil || byHash == nil || byHash.ContractID != "c1" {
		t.Errorf("by hashlock: %+v err=%v", byHash, err)
	}
	missHash, err := repo.GetHTLCByHashLock(ctx, "nope")
	if err != nil || missHash != nil {
		t.Errorf("expected (nil,nil) for missing hashlock, got %+v err=%v", missHash, err)
	}

	rec.State = domain.HTLCStateSettled
	rec.CounterpartyLocked = true
	if err := repo.UpdateHTLC(ctx, rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.GetHTLC(ctx, "c1")
	if got.State != domain.HTLCStateSettled || !got.CounterpartyLocked {
		t.Errorf("update not persisted: %+v", got)
	}

	// A non-terminal record should be returned by ListNonTerminal; settled should not.
	nt, err := repo.ListNonTerminal(ctx)
	if err != nil {
		t.Fatalf("list non-terminal: %v", err)
	}
	for _, r := range nt {
		if r.ContractID == "c1" {
			t.Errorf("settled record should not be non-terminal")
		}
	}

	// Add a locked record to exercise ListHTLCs filters and ListNonTerminal.
	_ = repo.CreateHTLC(ctx, &domain.HTLCRecord{
		ContractID: "c2", AgreementID: "FX2", Sender: "op@spoke-a-bank-a",
		Receiver: "op@spoke-a-bank-c", Amount: "50", HashLock: "hash2",
		TimeLock: 9999, State: domain.HTLCStateLocked,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})

	nt, _ = repo.ListNonTerminal(ctx)
	if len(nt) != 1 || nt[0].ContractID != "c2" {
		t.Errorf("non-terminal = %+v", nt)
	}

	byAgreement, _ := repo.ListHTLCs(ctx, ports.HTLCFilter{AgreementID: "FX2"})
	if len(byAgreement) != 1 || byAgreement[0].ContractID != "c2" {
		t.Errorf("filter by agreement = %+v", byAgreement)
	}
	bySender, _ := repo.ListHTLCs(ctx, ports.HTLCFilter{Sender: "op@spoke-a-bank-a"})
	if len(bySender) != 2 {
		t.Errorf("filter by sender = %d", len(bySender))
	}
	byReceiver, _ := repo.ListHTLCs(ctx, ports.HTLCFilter{Receiver: "op@spoke-a-bank-c"})
	if len(byReceiver) != 1 {
		t.Errorf("filter by receiver = %d", len(byReceiver))
	}
	byState, _ := repo.ListHTLCs(ctx, ports.HTLCFilter{State: string(domain.HTLCStateLocked)})
	if len(byState) != 1 {
		t.Errorf("filter by state = %d", len(byState))
	}
}

// --- FX agreement GORM repository ---

func TestGormFXAgreementRepository_Lifecycle(t *testing.T) {
	repo, err := repository.NewGormFXAgreementRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()

	now := time.Now().UTC()
	rec := &domain.FXAgreementRecord{
		TradeID: "T1", Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD",
		CounterCurrency: "BRL", Rate: "1.2", ExpiryDate: uint64(now.Add(time.Hour).Unix()),
		State: domain.FXStateProposed, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateAgreement(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetAgreement(ctx, "T1")
	if err != nil || got == nil || got.State != domain.FXStateProposed {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	missing, err := repo.GetAgreement(ctx, "nope")
	if err != nil || missing != nil {
		t.Errorf("expected (nil,nil), got %+v err=%v", missing, err)
	}

	rec.State = domain.FXStateAccepted
	if err := repo.UpdateAgreement(ctx, rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.GetAgreement(ctx, "T1")
	if got.State != domain.FXStateAccepted {
		t.Errorf("update not persisted: %s", got.State)
	}

	// Filters.
	byParty, _ := repo.ListAgreements(ctx, ports.FXAgreementFilter{Counterparty: "bank-b"})
	if len(byParty) != 1 {
		t.Errorf("filter by counterparty = %d", len(byParty))
	}
	byState, _ := repo.ListAgreements(ctx, ports.FXAgreementFilter{State: domain.FXStateAccepted})
	if len(byState) != 1 {
		t.Errorf("filter by state = %d", len(byState))
	}
	none, _ := repo.ListAgreements(ctx, ports.FXAgreementFilter{State: domain.FXStateRejected})
	if len(none) != 0 {
		t.Errorf("expected no rejected agreements, got %d", len(none))
	}

	// Audit events: append-only.
	if err := repo.CreateAuditEvent(ctx, &domain.FXAgreementEvent{
		TradeID: "T1", FromState: domain.FXStateProposed, ToState: domain.FXStateAccepted,
		Actor: "tester", OccurredAt: now, Source: domain.EventSourceLocalAPI,
	}); err != nil {
		t.Fatalf("create event: %v", err)
	}
	events, err := repo.ListAuditEvents(ctx, "T1")
	if err != nil || len(events) != 1 {
		t.Fatalf("list events: len=%d err=%v", len(events), err)
	}
	if events[0].ToState != domain.FXStateAccepted {
		t.Errorf("event ToState = %s", events[0].ToState)
	}
}

// TestGormFXAgreementRepository_CreateAgreement_DuplicateIsNoop covers the race between the
// synchronous propose handler and the FXIndexer: both can call CreateAgreement for the same
// trade_id. The second call must silently no-op (ON CONFLICT DO NOTHING), not return an error.
func TestGormFXAgreementRepository_CreateAgreement_DuplicateIsNoop(t *testing.T) {
	repo, err := repository.NewGormFXAgreementRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()

	now := time.Now().UTC()
	rec := &domain.FXAgreementRecord{
		TradeID: "T-DUP", Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD",
		CounterCurrency: "BRL", Rate: "1.2", ExpiryDate: uint64(now.Add(time.Hour).Unix()),
		State: domain.FXStateProposed, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateAgreement(ctx, rec); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// A second writer (e.g. the FXIndexer) racing to project the same on-chain agreement.
	dup := *rec
	if err := repo.CreateAgreement(ctx, &dup); err != nil {
		t.Fatalf("duplicate create must no-op, got error: %v", err)
	}

	got, err := repo.GetAgreement(ctx, "T-DUP")
	if err != nil || got == nil || got.State != domain.FXStateProposed {
		t.Fatalf("get after duplicate: %+v err=%v", got, err)
	}
}

func TestGormFXAgreementRepository_SpokeKeyedColumns(t *testing.T) {
	repo, err := repository.NewGormFXAgreementRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()

	rec := &domain.FXAgreementRecord{
		TradeID: "T-SPOKE-GORM", Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD",
		CounterCurrency: "BRL", Rate: "1.2",
		ExpiryDate:     uint64(now.Add(time.Hour).Unix()),
		SourceSpokeId:  "spoke-brl",
		DestSpokeId:    "spoke-usd",
		SourceReceiver: "recv@spoke-brl-bank-a",
		DestReceiver:   "recv@spoke-usd-bank-b",
		State:          domain.FXStateProposed,
		CreatedAt:      now, UpdatedAt: now,
	}
	if err := repo.CreateAgreement(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetAgreement(ctx, "T-SPOKE-GORM")
	if err != nil || got == nil {
		t.Fatalf("get: err=%v got=%v", err, got)
	}
	if got.SourceSpokeId != "spoke-brl" {
		t.Errorf("SourceSpokeId = %q, want %q", got.SourceSpokeId, "spoke-brl")
	}
	if got.DestSpokeId != "spoke-usd" {
		t.Errorf("DestSpokeId = %q, want %q", got.DestSpokeId, "spoke-usd")
	}
	if got.SourceReceiver != "recv@spoke-brl-bank-a" {
		t.Errorf("SourceReceiver = %q, want %q", got.SourceReceiver, "recv@spoke-brl-bank-a")
	}
	if got.DestReceiver != "recv@spoke-usd-bank-b" {
		t.Errorf("DestReceiver = %q, want %q", got.DestReceiver, "recv@spoke-usd-bank-b")
	}
}

func TestGormFXAgreementRepository_SpokeKeyedMigration_Idempotent(t *testing.T) {
	db := newTestDB(t)

	// Create the fx_agreements table with the LEGACY schema (spoke_a_receiver /
	// spoke_b_receiver columns present; source_* / dest_* columns absent).
	createLegacy := `
		CREATE TABLE fx_agreements (
			trade_id TEXT PRIMARY KEY,
			originator TEXT NOT NULL,
			counterparty_b TEXT NOT NULL,
			settlement_agent TEXT,
			custodian TEXT,
			beneficiary TEXT,
			origin_amount TEXT NOT NULL,
			counter_amount TEXT NOT NULL,
			origin_currency TEXT NOT NULL,
			counter_currency TEXT NOT NULL,
			rate TEXT NOT NULL,
			spoke_a_receiver TEXT,
			spoke_b_receiver TEXT,
			expiry_date INTEGER NOT NULL,
			state TEXT NOT NULL,
			on_chain_tx_hash TEXT,
			group_id TEXT,
			contract_address TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`
	if err := db.Exec(createLegacy).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}

	// Seed a legacy row.
	seed := `
		INSERT INTO fx_agreements (trade_id, originator, counterparty_b, origin_amount, counter_amount,
			origin_currency, counter_currency, rate, spoke_a_receiver, spoke_b_receiver,
			expiry_date, state, created_at, updated_at)
		VALUES ('T-LEGACY-1', 'bank-a', 'bank-b', '100', '120', 'USD', 'BRL', '1.2',
			'recv@spoke-a', 'recv@spoke-b', 9999999999, 'PROPOSED',
			datetime('now'), datetime('now'))
	`
	if err := db.Exec(seed).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	// AutoMigrate adds the new columns (source_spoke_id, etc.) from the GORM model.
	if err := db.AutoMigrate(&repository.FXAgreementModel{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// Run migration the first time.
	if err := repository.RunSpokeKeyedMigration(db, testLogger()); err != nil {
		t.Fatalf("first migration: %v", err)
	}

	// Verify the new columns have been backfilled.
	var row struct {
		SourceSpokeId  string
		DestSpokeId    string
		SourceReceiver string
		DestReceiver   string
	}
	if err := db.Raw(`SELECT source_spoke_id, dest_spoke_id, source_receiver, dest_receiver FROM fx_agreements WHERE trade_id = 'T-LEGACY-1'`).Scan(&row).Error; err != nil {
		t.Fatalf("query backfilled row: %v", err)
	}
	if row.SourceSpokeId != "spoke-a" {
		t.Errorf("source_spoke_id = %q, want %q", row.SourceSpokeId, "spoke-a")
	}
	if row.DestSpokeId != "spoke-b" {
		t.Errorf("dest_spoke_id = %q, want %q", row.DestSpokeId, "spoke-b")
	}
	if row.SourceReceiver != "recv@spoke-a" {
		t.Errorf("source_receiver = %q, want %q", row.SourceReceiver, "recv@spoke-a")
	}
	if row.DestReceiver != "recv@spoke-b" {
		t.Errorf("dest_receiver = %q, want %q", row.DestReceiver, "recv@spoke-b")
	}

	// Verify legacy columns are gone.
	var colCount int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('fx_agreements') WHERE name = 'spoke_a_receiver'").Scan(&colCount).Error; err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if colCount != 0 {
		t.Errorf("spoke_a_receiver column should be dropped")
	}
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('fx_agreements') WHERE name = 'spoke_b_receiver'").Scan(&colCount).Error; err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if colCount != 0 {
		t.Errorf("spoke_b_receiver column should be dropped")
	}

	// Second run must be a no-op (old columns already gone — UPDATE fails,
	// RunSpokeKeyedMigration returns nil).
	if err := repository.RunSpokeKeyedMigration(db, testLogger()); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	// Row count unchanged.
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM fx_agreements").Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

// TestRunSpokeKeyedMigration_FreshSchema verifies that RunSpokeKeyedMigration
// is a no-op when the table has never had legacy columns (fresh schema via
// AutoMigrate only — no spoke_a_receiver / spoke_b_receiver columns).
func TestRunSpokeKeyedMigration_FreshSchema(t *testing.T) {
	db := newTestDB(t)

	// AutoMigrate adds source_spoke_id, dest_spoke_id, etc. but NOT the
	// legacy spoke_a_receiver / spoke_b_receiver columns.
	if err := db.AutoMigrate(&repository.FXAgreementModel{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// Migration should be a no-op — no legacy columns present.
	if err := repository.RunSpokeKeyedMigration(db, testLogger()); err != nil {
		t.Fatalf("RunSpokeKeyedMigration on fresh schema: %v", err)
	}

	// Verify no legacy columns exist.
	for _, col := range []string{"spoke_a_receiver", "spoke_b_receiver"} {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('fx_agreements') WHERE name = '%s'", col)
		if err := db.Raw(query).Scan(&count).Error; err != nil {
			t.Fatalf("pragma for %s: %v", col, err)
		}
		if count != 0 {
			t.Errorf("column %s should not exist on fresh schema", col)
		}
	}

	// Verify new columns exist.
	for _, col := range []string{"source_spoke_id", "dest_spoke_id", "source_receiver", "dest_receiver"} {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('fx_agreements') WHERE name = '%s'", col)
		if err := db.Raw(query).Scan(&count).Error; err != nil {
			t.Fatalf("pragma for %s: %v", col, err)
		}
		if count != 1 {
			t.Errorf("column %s should exist on fresh schema, got count=%d", col, count)
		}
	}
}

// TestRunSpokeKeyedMigration_PartialState simulates an interrupted migration
// (spoke_a_receiver already dropped, spoke_b_receiver still present, data
// already backfilled) and verifies that RunSpokeKeyedMigration completes
// the remaining work without data loss.
func TestRunSpokeKeyedMigration_PartialState(t *testing.T) {
	db := newTestDB(t)

	// 1. Create legacy table with both positional columns.
	createLegacy := `
		CREATE TABLE fx_agreements (
			trade_id TEXT PRIMARY KEY,
			originator TEXT NOT NULL,
			counterparty_b TEXT NOT NULL,
			settlement_agent TEXT,
			custodian TEXT,
			beneficiary TEXT,
			origin_amount TEXT NOT NULL,
			counter_amount TEXT NOT NULL,
			origin_currency TEXT NOT NULL,
			counter_currency TEXT NOT NULL,
			rate TEXT NOT NULL,
			spoke_a_receiver TEXT,
			spoke_b_receiver TEXT,
			expiry_date INTEGER NOT NULL,
			state TEXT NOT NULL,
			on_chain_tx_hash TEXT,
			group_id TEXT,
			contract_address TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`
	if err := db.Exec(createLegacy).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}

	// 2. Seed a legacy row.
	seed := `
		INSERT INTO fx_agreements (trade_id, originator, counterparty_b, origin_amount, counter_amount,
			origin_currency, counter_currency, rate, spoke_a_receiver, spoke_b_receiver,
			expiry_date, state, created_at, updated_at)
		VALUES ('T-PARTIAL-1', 'bank-a', 'bank-b', '100', '120', 'USD', 'BRL', '1.2',
			'recv@a', 'recv@b', 9999999999, 'PROPOSED',
			datetime('now'), datetime('now'))
	`
	if err := db.Exec(seed).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	// 3. AutoMigrate adds the new spoke-keyed columns.
	if err := db.AutoMigrate(&repository.FXAgreementModel{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// 4. Simulate backfill that would have run before the first DROP.
	backfill := `
		UPDATE fx_agreements SET
			source_spoke_id = 'spoke-a',
			dest_spoke_id = 'spoke-b',
			source_receiver = COALESCE(spoke_a_receiver, ''),
			dest_receiver = COALESCE(spoke_b_receiver, '')
		WHERE source_spoke_id IS NULL OR source_spoke_id = ''
	`
	if err := db.Exec(backfill).Error; err != nil {
		t.Fatalf("simulate backfill: %v", err)
	}

	// 5. Simulate first DROP (spoke_a_receiver) completed — only spoke_b_receiver remains.
	if err := db.Exec("ALTER TABLE fx_agreements DROP COLUMN spoke_a_receiver").Error; err != nil {
		t.Fatalf("simulate first DROP: %v", err)
	}

	// 6. Call RunSpokeKeyedMigration — should complete the remaining DROP.
	if err := repository.RunSpokeKeyedMigration(db, testLogger()); err != nil {
		t.Fatalf("RunSpokeKeyedMigration on partial state: %v", err)
	}

	// 7. Assertions: no error (checked above), spoke_b_receiver absent.
	var colCount int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('fx_agreements') WHERE name = 'spoke_b_receiver'").Scan(&colCount).Error; err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if colCount != 0 {
		t.Errorf("spoke_b_receiver column should be dropped")
	}

	// 8. Verify data intact.
	var row struct {
		SourceReceiver string
		DestReceiver   string
		SourceSpokeId  string
		DestSpokeId    string
	}
	if err := db.Raw(`SELECT source_spoke_id, dest_spoke_id, source_receiver, dest_receiver FROM fx_agreements WHERE trade_id = 'T-PARTIAL-1'`).Scan(&row).Error; err != nil {
		t.Fatalf("query row: %v", err)
	}
	if row.SourceReceiver != "recv@a" {
		t.Errorf("source_receiver = %q, want %q", row.SourceReceiver, "recv@a")
	}
	if row.DestReceiver != "recv@b" {
		t.Errorf("dest_receiver = %q, want %q", row.DestReceiver, "recv@b")
	}
	if row.SourceSpokeId != "spoke-a" {
		t.Errorf("source_spoke_id = %q, want %q", row.SourceSpokeId, "spoke-a")
	}
	if row.DestSpokeId != "spoke-b" {
		t.Errorf("dest_spoke_id = %q, want %q", row.DestSpokeId, "spoke-b")
	}

	// 9. Row count = 1 (no data duplication or loss).
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM fx_agreements").Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

func TestGormFXAgreementRepository_ListExpiredNonTerminal(t *testing.T) {
	repo, err := repository.NewGormFXAgreementRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()

	// Expired + non-terminal → should be returned.
	_ = repo.CreateAgreement(ctx, &domain.FXAgreementRecord{
		TradeID: "expired-proposed", Originator: "a", CounterpartyB: "b",
		OriginAmount: "1", CounterAmount: "1", OriginCurrency: "USD", CounterCurrency: "BRL",
		Rate: "1", ExpiryDate: uint64(now.Add(-time.Hour).Unix()),
		State: domain.FXStateProposed, CreatedAt: now, UpdatedAt: now,
	})
	// Expired but terminal → must be excluded.
	_ = repo.CreateAgreement(ctx, &domain.FXAgreementRecord{
		TradeID: "expired-settled", Originator: "a", CounterpartyB: "b",
		OriginAmount: "1", CounterAmount: "1", OriginCurrency: "USD", CounterCurrency: "BRL",
		Rate: "1", ExpiryDate: uint64(now.Add(-time.Hour).Unix()),
		State: domain.FXStateSettled, CreatedAt: now, UpdatedAt: now,
	})
	// Not yet expired → must be excluded.
	_ = repo.CreateAgreement(ctx, &domain.FXAgreementRecord{
		TradeID: "future-accepted", Originator: "a", CounterpartyB: "b",
		OriginAmount: "1", CounterAmount: "1", OriginCurrency: "USD", CounterCurrency: "BRL",
		Rate: "1", ExpiryDate: uint64(now.Add(time.Hour).Unix()),
		State: domain.FXStateAccepted, CreatedAt: now, UpdatedAt: now,
	})

	expired, err := repo.ListExpiredNonTerminal(ctx, now.Unix())
	if err != nil {
		t.Fatalf("list expired: %v", err)
	}
	if len(expired) != 1 || expired[0].TradeID != "expired-proposed" {
		t.Errorf("expired = %+v", expired)
	}
}
