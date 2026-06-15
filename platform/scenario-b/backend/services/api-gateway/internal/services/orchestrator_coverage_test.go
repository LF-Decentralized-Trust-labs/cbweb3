// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// statefulSwapRepo records the latest status/fields so the deferred transfer-limit
// restore logic (which re-reads the op via GetByID) and GetStatus can be exercised.
type statefulSwapRepo struct {
	op        *domain.CrossCurrencySwapOperation
	createErr error
	getErr    error
}

func (r *statefulSwapRepo) Create(_ context.Context, op *domain.CrossCurrencySwapOperation) error {
	if r.createErr != nil {
		return r.createErr
	}
	cp := *op
	r.op = &cp
	return nil
}
func (r *statefulSwapRepo) GetByID(_ context.Context, _ string) (*domain.CrossCurrencySwapOperation, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.op, nil
}
func (r *statefulSwapRepo) UpdateStatus(_ context.Context, _ string, s domain.SwapOperationStatus) error {
	if r.op != nil {
		r.op.Status = s
	}
	return nil
}
func (r *statefulSwapRepo) UpdateBridgeInPositionID(_ context.Context, _, id string) error {
	if r.op != nil {
		r.op.BridgeInPositionID = &id
	}
	return nil
}
func (r *statefulSwapRepo) UpdateSwapResult(_ context.Context, _, tx, amt string) error {
	if r.op != nil {
		r.op.SwapTxHash = &tx
		r.op.AmountIn = amt
	}
	return nil
}
func (r *statefulSwapRepo) UpdateBridgeOutPositionID(_ context.Context, _, id string) error {
	if r.op != nil {
		r.op.BridgeOutPositionID = &id
	}
	return nil
}
func (r *statefulSwapRepo) UpdateFailureReason(_ context.Context, _, reason string) error {
	if r.op != nil {
		r.op.Status = domain.SwapStatusFailed
		r.op.FailureReason = &reason
	}
	return nil
}

func happyReq() CrossCurrencySwapRequest {
	return CrossCurrencySwapRequest{
		SwapID:         "swap-1",
		CorrelationID:  "corr-1",
		SourceCurrency: "BRL",
		TargetCurrency: "ARS",
		PoolPair:       "W-BRL-ARS",
		AmountOut:      "100",
		MaxAmountIn:    "1000",
		PayerBankID:    "central-bank-a",
	}
}

func newOrch(repo CrossCurrencySwapRepository, swapSvc SwapServiceIface, poller BridgePositionPoller) *CrossCurrencySwapOrchestrator {
	return NewCrossCurrencySwapOrchestrator(
		repo, nil,
		&stubLockMint{}, stubBurnUnlock{}, swapSvc,
		stubPoolActive{}, stubCBOK{},
		nil, nil, poller,
	).WithHubSignerAddress("0xCBSIGNER")
}

func TestOrchestrator_CreateError(t *testing.T) {
	orch := newOrch(&statefulSwapRepo{createErr: errors.New("db down")}, stubSwapService{}, stubActivePoller{})
	if _, err := orch.Execute(context.Background(), happyReq()); err == nil {
		t.Fatal("expected create error")
	}
}

func TestOrchestrator_HappyPath_LocalBridge(t *testing.T) {
	repo := &statefulSwapRepo{}
	orch := newOrch(repo, fixedAmountInSwap{amountIn: "800"}, &stubActiveThenReleasedPoller{})
	res, err := orch.Execute(context.Background(), happyReq())
	if err != nil {
		t.Fatalf("unexpected error on happy path: %v", err)
	}
	if res.Status != domain.SwapStatusCompleted {
		t.Fatalf("expected COMPLETED, got %s", res.Status)
	}
	if res.SwapTxHash != "0xdeadbeef" || res.AmountIn != "800" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestOrchestrator_PoolNotActive(t *testing.T) {
	repo := &statefulSwapRepo{}
	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil, &stubLockMint{}, stubBurnUnlock{}, stubSwapService{},
		poolInactive{}, stubCBOK{}, nil, nil, stubActivePoller{},
	)
	_, err := orch.Execute(context.Background(), happyReq())
	if err == nil || !strings.Contains(err.Error(), "not ACTIVE") {
		t.Fatalf("expected pool not active error, got %v", err)
	}
}

func TestOrchestrator_PoolStatusError(t *testing.T) {
	repo := &statefulSwapRepo{}
	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil, &stubLockMint{}, stubBurnUnlock{}, stubSwapService{},
		poolErr{}, stubCBOK{}, nil, nil, stubActivePoller{},
	)
	if _, err := orch.Execute(context.Background(), happyReq()); err == nil {
		t.Fatal("expected pool status error")
	}
}

func TestOrchestrator_BreakerHalted(t *testing.T) {
	repo := &statefulSwapRepo{}
	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil, &stubLockMint{}, stubBurnUnlock{}, stubSwapService{},
		stubPoolActive{}, cbHalted{}, nil, nil, stubActivePoller{},
	)
	_, err := orch.Execute(context.Background(), happyReq())
	if err == nil || !strings.Contains(err.Error(), "HALTED") {
		t.Fatalf("expected breaker halted error, got %v", err)
	}
}

func TestOrchestrator_BreakerError(t *testing.T) {
	repo := &statefulSwapRepo{}
	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil, &stubLockMint{}, stubBurnUnlock{}, stubSwapService{},
		stubPoolActive{}, cbErr{}, nil, nil, stubActivePoller{},
	)
	if _, err := orch.Execute(context.Background(), happyReq()); err == nil {
		t.Fatal("expected breaker error")
	}
}

func TestOrchestrator_QuoteExpired(t *testing.T) {
	repo := &statefulSwapRepo{}
	expired := &domain.SwapQuote{ValidUntil: time.Now().Add(-time.Minute)}
	orch := NewCrossCurrencySwapOrchestrator(
		repo, &quoteRepoStub{quote: expired}, &stubLockMint{}, stubBurnUnlock{}, stubSwapService{},
		stubPoolActive{}, stubCBOK{}, nil, nil, stubActivePoller{},
	)
	qid := "quote-1"
	req := happyReq()
	req.QuoteID = &qid
	_, err := orch.Execute(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected quote expired error, got %v", err)
	}
}

func TestOrchestrator_QuoteNotFound(t *testing.T) {
	repo := &statefulSwapRepo{}
	orch := NewCrossCurrencySwapOrchestrator(
		repo, &quoteRepoStub{err: errors.New("missing")}, &stubLockMint{}, stubBurnUnlock{}, stubSwapService{},
		stubPoolActive{}, stubCBOK{}, nil, nil, stubActivePoller{},
	)
	qid := "quote-x"
	req := happyReq()
	req.QuoteID = &qid
	if _, err := orch.Execute(context.Background(), req); err == nil {
		t.Fatal("expected quote not found error")
	}
}

func TestOrchestrator_TransferLimitRejected_AndRestoreOnFailure(t *testing.T) {
	repo := &statefulSwapRepo{}
	limiter := &recordingLimiter{checkErr: errors.New("daily limit hit")}
	orch := newOrch(repo, stubSwapService{}, stubActivePoller{}).WithTransferLimitChecker(limiter)
	if _, err := orch.Execute(context.Background(), happyReq()); err == nil {
		t.Fatal("expected transfer limit error")
	}
}

func TestOrchestrator_TransferLimit_RestoreOnSwapFailure(t *testing.T) {
	repo := &statefulSwapRepo{}
	limiter := &recordingLimiter{}
	// swap fails after the limit deduct → deferred restore must fire (status FAILED).
	orch := newOrch(repo, stubSwapService{}, stubActivePoller{}).WithTransferLimitChecker(limiter)
	if _, err := orch.Execute(context.Background(), happyReq()); err == nil {
		t.Fatal("expected swap failure")
	}
	if !limiter.restored {
		t.Fatal("expected transfer quota to be restored after swap failure")
	}
}

func TestOrchestrator_SwapFailsTriggersRollback(t *testing.T) {
	repo := &statefulSwapRepo{}
	// Real coordinator whose bridge reverso succeeds on the first attempt → no backoff sleep.
	rb := NewSwapRollbackCoordinator(reversoOK{}, rollbackRepoStub{})
	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil, &stubLockMint{}, stubBurnUnlock{}, stubSwapService{},
		stubPoolActive{}, stubCBOK{}, rb, nil, stubActivePoller{},
	).WithHubSignerAddress("0x")
	if _, err := orch.Execute(context.Background(), happyReq()); err == nil {
		t.Fatal("expected swap failure")
	}
}

func TestOrchestrator_GetStatus(t *testing.T) {
	tx := "0xtx"
	binID := "pos-in"
	boutID := "pos-out"
	reason := "some reason"
	completed := time.Now()
	repo := &statefulSwapRepo{op: &domain.CrossCurrencySwapOperation{
		SwapID:              "swap-9",
		CorrelationID:       "corr-9",
		Status:              domain.SwapStatusCompleted,
		AmountIn:            "800",
		AmountOut:           "100",
		EffectiveRate:       1.0,
		BridgeInPositionID:  &binID,
		SwapTxHash:          &tx,
		BridgeOutPositionID: &boutID,
		FailureReason:       &reason,
		CompletedAt:         &completed,
	}}
	orch := newOrch(repo, stubSwapService{}, stubActivePoller{})
	res, err := orch.GetStatus(context.Background(), "swap-9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SwapTxHash != tx || res.BridgeInPositionID != binID || res.BridgeOutPositionID != boutID || res.FailureReason != reason {
		t.Fatalf("unexpected status result: %+v", res)
	}
}

func TestOrchestrator_GetStatus_NotFound(t *testing.T) {
	repo := &statefulSwapRepo{getErr: errors.New("missing")}
	orch := newOrch(repo, stubSwapService{}, stubActivePoller{})
	if _, err := orch.GetStatus(context.Background(), "nope"); err == nil {
		t.Fatal("expected not found error")
	}
}

// --- helper stub types ---

type poolInactive struct{}

func (poolInactive) IsActive(context.Context, string) (bool, error) { return false, nil }

type poolErr struct{}

func (poolErr) IsActive(context.Context, string) (bool, error) { return false, errors.New("rpc") }

type cbHalted struct{}

func (cbHalted) IsHalted(context.Context, string) (bool, error) { return true, nil }

type cbErr struct{}

func (cbErr) IsHalted(context.Context, string) (bool, error) { return false, errors.New("rpc") }

type quoteRepoStub struct {
	quote *domain.SwapQuote
	err   error
}

func (q *quoteRepoStub) Create(context.Context, *domain.SwapQuote) error { return nil }
func (q *quoteRepoStub) FindByID(context.Context, string) (*domain.SwapQuote, error) {
	return q.quote, q.err
}
func (q *quoteRepoStub) DeleteExpired(context.Context, time.Time) (int64, error) { return 0, nil }

type recordingLimiter struct {
	checkErr error
	restored bool
}

func (r *recordingLimiter) CheckAndDeduct(context.Context, string, string, string) error {
	return r.checkErr
}
func (r *recordingLimiter) Restore(context.Context, string, string, string) {
	r.restored = true
}

type reversoOK struct{}

func (reversoOK) BurnAndEnqueue(context.Context, string, string) (*BridgePositionResult, error) {
	return &BridgePositionResult{}, nil
}

type rollbackRepoStub struct{}

func (rollbackRepoStub) Create(context.Context, *domain.SwapRollbackLog) error    { return nil }
func (rollbackRepoStub) UpdateStatus(context.Context, string, domain.RollbackStatus) error {
	return nil
}
func (rollbackRepoStub) IncrementRetryCount(context.Context, string) error           { return nil }
func (rollbackRepoStub) UpdateTxHashes(context.Context, string, string, string) error { return nil }
