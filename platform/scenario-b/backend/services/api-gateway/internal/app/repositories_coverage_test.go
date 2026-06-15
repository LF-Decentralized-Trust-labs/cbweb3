// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite" // pure-Go (no CGO) sqlite driver, test-only
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
)

func repoDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// ---------------------------------------------------------------------------
// SwapQuoteRepository
// ---------------------------------------------------------------------------

func TestSwapQuoteRepository(t *testing.T) {
	db := repoDB(t, &domain.SwapQuote{})
	r := newSwapQuoteRepository(db)
	ctx := context.Background()

	q := &domain.SwapQuote{QuoteID: "q-1", PoolPair: "P", AmountOut: "1", AmountIn: "1", CreatedAt: time.Now(), ValidUntil: time.Now().Add(time.Minute)}
	if err := r.Create(ctx, q); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := r.FindByID(ctx, "q-1")
	if err != nil || got.QuoteID != "q-1" {
		t.Fatalf("find: %v", err)
	}
	if _, err := r.FindByID(ctx, "missing"); err == nil {
		t.Fatal("expected not-found error")
	}

	// Insert an old quote and delete expired.
	old := &domain.SwapQuote{QuoteID: "q-old", PoolPair: "P", AmountOut: "1", AmountIn: "1", CreatedAt: time.Now().Add(-2 * time.Hour), ValidUntil: time.Now()}
	db.Create(old)
	n, err := r.DeleteExpired(ctx, time.Now().Add(-time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("delete expired: n=%d err=%v", n, err)
	}
}

// ---------------------------------------------------------------------------
// CrossCurrencySwapRepository
// ---------------------------------------------------------------------------

func TestCrossCurrencySwapRepository(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	ctx := context.Background()

	op := &domain.CrossCurrencySwapOperation{
		SwapID: "s-1", CorrelationID: "c-1", PayerBankID: "a", BeneficiaryBankID: "b",
		SourceCurrency: "BRL", TargetCurrency: "ARS", PoolPair: "P",
		AmountIn: "0", AmountOut: "100", MaxAmountIn: "1000", Status: domain.SwapStatusQuoting,
	}
	if err := r.Create(ctx, op); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := r.GetByID(ctx, "s-1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, err := r.FindByID(ctx, "s-1"); err != nil {
		t.Fatalf("find: %v", err)
	}
	if _, err := r.GetByID(ctx, "nope"); err == nil {
		t.Fatal("expected not-found")
	}

	corr, err := r.FindByCorrelationID(ctx, "c-1")
	if err != nil || len(corr) != 1 {
		t.Fatalf("find by correlation: %d %v", len(corr), err)
	}

	_ = r.UpdateStatus(ctx, "s-1", domain.SwapStatusSwapInProgress)
	_ = r.UpdateBridgeInPositionID(ctx, "s-1", "pos-in")
	_ = r.UpdateSwapResult(ctx, "s-1", "0xtx", "950")
	_ = r.UpdateBridgeOutPositionID(ctx, "s-1", "pos-out")
	_ = r.UpdateFailureReason(ctx, "s-1", "boom")

	got, _ := r.GetByID(ctx, "s-1")
	if got.Status != domain.SwapStatusFailed || got.AmountIn != "950" || got.SwapTxHash == nil || *got.SwapTxHash != "0xtx" {
		t.Fatalf("unexpected final state: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// PoolCommitRepository
// ---------------------------------------------------------------------------

func TestPoolCommitRepository(t *testing.T) {
	db := repoDB(t, &domain.PoolCommit{})
	r := NewPoolCommitRepository(db)
	ctx := context.Background()

	c := &domain.PoolCommit{
		CommitID: "cm-1", PoolPair: "P", ProviderID: "cb-a", Side: domain.CommitSideA,
		Amount: "100", Status: domain.CommitStatusPending,
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := r.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	if got, err := r.FindByID(ctx, "cm-1"); err != nil || got.CommitID != "cm-1" {
		t.Fatalf("find by id: %v", err)
	}
	if got, err := r.FindActiveByPairAndSide(ctx, "P", domain.CommitSideA); err != nil || got == nil {
		t.Fatalf("find active: %v", err)
	}
	if got, err := r.FindByProviderAndPair(ctx, "cb-a", "P"); err != nil || got == nil {
		t.Fatalf("find by provider: %v", err)
	}
	if list, err := r.ListByPair(ctx, "P", ""); err != nil || len(list) != 1 {
		t.Fatalf("list by pair: %d %v", len(list), err)
	}
	if list, err := r.ListByPair(ctx, "P", "PENDING"); err != nil || len(list) != 1 {
		t.Fatalf("list by pair+status: %d %v", len(list), err)
	}
	if err := r.UpdateStatus(ctx, "cm-1", domain.CommitStatusMatched); err != nil {
		t.Fatalf("update status: %v", err)
	}

	// Expired pending lookup.
	exp := &domain.PoolCommit{CommitID: "cm-exp", PoolPair: "Q", ProviderID: "cb-b", Side: domain.CommitSideB, Amount: "1", Status: domain.CommitStatusPending, CreatedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: time.Now().Add(-time.Hour)}
	db.Create(exp)
	expired, err := r.FindExpiredPending(ctx)
	if err != nil || len(expired) != 1 {
		t.Fatalf("find expired pending: %d %v", len(expired), err)
	}

	// On-chain commit id lookups.
	onChain := []byte{0xaa, 0xbb}
	withChain := &domain.PoolCommit{CommitID: "cm-chain", PoolPair: "R", ProviderID: "cb-c", Side: domain.CommitSideA, Amount: "1", Status: domain.CommitStatusPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), OnChainCommitID: &onChain}
	db.Create(withChain)
	hexID := "0xaabb"
	if err := r.UpdateStatusByOnChainCommitID(ctx, hexID, domain.CommitStatusExecuted); err != nil {
		t.Fatalf("update by on-chain id: %v", err)
	}
	if got, err := r.FindByOnChainCommitID(ctx, hexID); err != nil || got == nil {
		t.Fatalf("find by on-chain id: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SwapRollbackLogRepository
// ---------------------------------------------------------------------------

func TestSwapRollbackLogRepository(t *testing.T) {
	db := repoDB(t, &domain.SwapRollbackLog{})
	r := newSwapRollbackLogRepository(db)
	ctx := context.Background()

	log := &domain.SwapRollbackLog{RollbackID: "rb-1", SwapOperationID: "s-1", BridgeInPositionID: "pos", RollbackStatus: domain.RollbackStatusPending}
	if err := r.Create(ctx, log); err != nil {
		t.Fatalf("create: %v", err)
	}
	if list, err := r.FindBySwapID(ctx, "s-1"); err != nil || len(list) != 1 {
		t.Fatalf("find by swap id: %d %v", len(list), err)
	}
	if err := r.UpdateStatus(ctx, "rb-1", domain.RollbackStatusCompleted); err != nil {
		t.Fatalf("update status: %v", err)
	}
	if err := r.IncrementRetryCount(ctx, "rb-1"); err != nil {
		t.Fatalf("increment: %v", err)
	}
	if err := r.UpdateTxHashes(ctx, "rb-1", "0xburn", "0xunlock"); err != nil {
		t.Fatalf("update tx hashes: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TransferLimitRepository
// ---------------------------------------------------------------------------

func TestTransferLimitRepository(t *testing.T) {
	db := repoDB(t, &domain.TransferLimit{})
	r := newTransferLimitRepository(db)
	ctx := context.Background()

	lim, err := r.Create(ctx, "cb-a", "bank-a", "BRL", "1000")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if list, err := r.List(ctx, "cb-a"); err != nil || len(list) != 1 {
		t.Fatalf("list: %d %v", len(list), err)
	}
	if got, err := r.FindApplicableLimit(ctx, "cb-a", "bank-a", "BRL"); err != nil || got == nil {
		t.Fatalf("find applicable: %v", err)
	}
	// No matching limit.
	if got, err := r.FindApplicableLimit(ctx, "cb-z", "bank-z", "USD"); err != nil || got != nil {
		t.Fatalf("expected nil limit, got %v err %v", got, err)
	}
	if err := r.Delete(ctx, lim.LimitID, "cb-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := r.Delete(ctx, "missing", "cb-a"); err == nil {
		t.Fatal("expected not-found on delete")
	}
}

// ---------------------------------------------------------------------------
// RateLimitCounterRepository
// ---------------------------------------------------------------------------

func TestRateLimitCounterRepository(t *testing.T) {
	db := repoDB(t, &domain.SwapRateLimitCounter{})
	r := newRateLimitCounterRepository(db)
	ctx := context.Background()
	win := time.Now().Truncate(time.Minute)

	// First increment creates the record.
	if err := r.IncrementCounter(ctx, "bank-a", domain.RateLimitWindowMinute, win); err != nil {
		t.Fatalf("increment 1: %v", err)
	}
	// Second increment updates it.
	if err := r.IncrementCounter(ctx, "bank-a", domain.RateLimitWindowMinute, win); err != nil {
		t.Fatalf("increment 2: %v", err)
	}
	count, exceeded, err := r.CheckLimit(ctx, "bank-a", domain.RateLimitWindowMinute, win)
	if err != nil || count != 2 || exceeded {
		t.Fatalf("check limit: count=%d exceeded=%v err=%v", count, exceeded, err)
	}
	// Unknown bank → 0.
	count, _, _ = r.CheckLimit(ctx, "bank-z", domain.RateLimitWindowMinute, win)
	if count != 0 {
		t.Fatalf("expected 0 for unknown bank, got %d", count)
	}

	db.Create(&domain.SwapRateLimitCounter{ID: "old", BankID: "x", WindowType: domain.RateLimitWindowMinute, WindowStart: time.Now().Add(-3 * time.Hour), SwapCount: 1})
	n, err := r.DeleteExpired(ctx, time.Now().Add(-2*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("delete expired: n=%d err=%v", n, err)
	}
}

// ---------------------------------------------------------------------------
// LPPositionRepository
// ---------------------------------------------------------------------------

func TestLPPositionRepository(t *testing.T) {
	db := repoDB(t, &domain.LiquidityPosition{})
	r := newLPPositionRepository(db)
	ctx := context.Background()

	pos := &domain.LiquidityPosition{LPID: "lp-1", ProviderBankID: "cb-a", PoolPair: "P", TokenAContributed: "1", TokenBContributed: "1", LPShares: "1", Status: domain.LPStatusActive, FeeClaimAccumulated: "0"}
	if err := r.Create(ctx, pos); err != nil {
		t.Fatalf("create: %v", err)
	}
	if list, err := r.FindByPoolPair(ctx, "P"); err != nil || len(list) != 1 {
		t.Fatalf("find by pool pair: %d %v", len(list), err)
	}
	if list, err := r.FindByProviderAndPoolPair(ctx, "cb-a", "P"); err != nil || len(list) != 1 {
		t.Fatalf("find by provider+pair: %d %v", len(list), err)
	}
}

// ---------------------------------------------------------------------------
// LPFeeEventRepository
// ---------------------------------------------------------------------------

func TestLPFeeEventRepository(t *testing.T) {
	db := repoDB(t, &domain.LPFeeEvent{})
	r := NewLPFeeEventRepository(db)
	ctx := context.Background()

	ev := &domain.LPFeeEvent{EventID: "ev-1", PoolPair: "P", SwapOrderID: "ord-1", FeeAmountA: "10", FeeAmountB: "0", Distribution: "{}"}
	if err := r.Create(ctx, ev); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got, err := r.FindBySwapOrderID(ctx, "ord-1"); err != nil || got == nil {
		t.Fatalf("find by order id: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PairRepository
// ---------------------------------------------------------------------------

func TestPairRepository(t *testing.T) {
	db := repoDB(t, &domain.PairProposal{})
	r := NewPairRepository(db)
	ctx := context.Background()

	p := &domain.PairProposal{PairID: "BRL-USD", ProposerCB: "cb-a", TokenAAddress: "0xa", TokenBAddress: "0xb", AMMAddress: "0xamm", Status: domain.PairStatusProposed, ProposedAt: time.Now()}
	if err := r.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got, err := r.FindByPairID(ctx, "BRL-USD"); err != nil || got == nil {
		t.Fatalf("find by id: %v", err)
	}
	if got, err := r.FindByTokenPair(ctx, "0xa", "0xb"); err != nil || got == nil {
		t.Fatalf("find by token pair: %v", err)
	}
	if err := r.Activate(ctx, "BRL-USD", "cb-b", time.Now()); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if list, err := r.ListActive(ctx); err != nil || len(list) != 1 {
		t.Fatalf("list active: %d %v", len(list), err)
	}
	if list, err := r.ListAll(ctx); err != nil || len(list) != 1 {
		t.Fatalf("list all: %d %v", len(list), err)
	}
}

// ---------------------------------------------------------------------------
// PoolStatusEnricher
// ---------------------------------------------------------------------------

func TestPoolStatusEnricher(t *testing.T) {
	db := repoDB(t, &domain.LiquidityPosition{}, &domain.PoolCommit{})
	e := newPoolStatusEnricher(db)
	ctx := context.Background()

	db.Create(&domain.LiquidityPosition{LPID: "lp-1", PoolPair: "P", Status: domain.LPStatusActive, TokenAContributed: "1", TokenBContributed: "1", LPShares: "1", FeeClaimAccumulated: "0"})
	db.Create(&domain.LiquidityPosition{LPID: "lp-2", PoolPair: "P", Status: domain.LPStatusWithdrawn, TokenAContributed: "1", TokenBContributed: "1", LPShares: "1", FeeClaimAccumulated: "0"})

	count, err := e.CountActiveLPs(ctx, "P")
	if err != nil || count != 1 {
		t.Fatalf("count active LPs: count=%d err=%v", count, err)
	}

	db.Create(&domain.PoolCommit{CommitID: "cm-1", PoolPair: "P", ProviderID: "cb-a", Side: domain.CommitSideA, Amount: "100", Status: domain.CommitStatusPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	pending, err := e.ListPendingCommits(ctx, "P")
	if err != nil || len(pending) != 1 {
		t.Fatalf("list pending commits: %d %v", len(pending), err)
	}
	if pending[0].CommitID != "cm-1" {
		t.Fatalf("unexpected pending commit: %+v", pending[0])
	}
}

// ---------------------------------------------------------------------------
// identityCBChecker (off-chain fallback)
// ---------------------------------------------------------------------------

type fakeUserManager struct {
	users []interfaces.UserSummary
	err   error
}

func (f *fakeUserManager) ListUsers(_ context.Context, _, _ string) ([]interfaces.UserSummary, int, error) {
	return f.users, len(f.users), f.err
}
func (f *fakeUserManager) GetUser(_ context.Context, _ string) (interfaces.UserDetail, error) {
	return interfaces.UserDetail{}, nil
}

func TestIdentityCBChecker(t *testing.T) {
	ctx := context.Background()
	um := &fakeUserManager{users: []interfaces.UserSummary{{WalletAddress: "0xABC"}}}
	c := newIdentityCBChecker(um)

	// Empty address → false, no lookup.
	if ok, err := c.IsCentralBankAddress(ctx, ""); err != nil || ok {
		t.Fatalf("empty address: ok=%v err=%v", ok, err)
	}
	// Matching (case-insensitive).
	if ok, err := c.IsCentralBankAddress(ctx, "0xabc"); err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	// Non-matching.
	if ok, err := c.IsCentralBankAddress(ctx, "0xdef"); err != nil || ok {
		t.Fatalf("expected no match, ok=%v err=%v", ok, err)
	}
	// List error propagates.
	cErr := newIdentityCBChecker(&fakeUserManager{err: errInlineApp("boom")})
	if _, err := cErr.IsCentralBankAddress(ctx, "0xabc"); err == nil {
		t.Fatal("expected list error")
	}
}

type errInlineApp string

func (e errInlineApp) Error() string { return string(e) }
