// SPDX-License-Identifier: Apache-2.0

// Package services provides tests for the on-chain transaction-hash visibility of the
// Scenario B circuit breaker (043-breaker-txhash-mock-docs / FR-001 to FR-005 / FR-009).
package services

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// txHashCaller is a fake AMMCircuitBreakerCaller that returns a distinct transaction hash
// per write action, so a test can prove the service carries the right hash for each one
// rather than reusing a single value.
type txHashCaller struct {
	pauseTx     string
	proposeID   string
	proposeTx   string
	signTx      string
	pauseErr    error
	proposeErr  error
	signErr     error
	executeErr  error
	paused      bool
	resumeID    string
	resumeSigs  int
	resumeQuoru int
	// chainTx is what the AMM's events report as the pair's latest breaker action, by any
	// institution. It is deliberately separate from the per-action hashes above so a test
	// can distinguish "the status read the chain" from "the status read this gateway's own
	// signature row" — the two agree only by accident in a single-institution fixture.
	chainTx    string
	chainTxErr error
}

func (f *txHashCaller) PauseCircuitBreaker(_ context.Context, _ string, _ []byte) (string, error) {
	if f.pauseErr == nil {
		f.paused = true
	}
	return f.pauseTx, f.pauseErr
}

func (f *txHashCaller) ProposeResume(_ context.Context, _ string, _ []byte) (string, string, error) {
	return f.proposeID, f.proposeTx, f.proposeErr
}

func (f *txHashCaller) SignResume(_ context.Context, _, _ string, _ []byte) (string, error) {
	if f.signErr == nil {
		f.paused = false
	}
	return f.signTx, f.signErr
}

func (f *txHashCaller) ExecuteResume(_ context.Context, _, _ string) error { return f.executeErr }

func (f *txHashCaller) IsPaused(_ context.Context, _ string) (bool, error) { return f.paused, nil }

func (f *txHashCaller) ActiveResumeProposal(_ context.Context, _ string) (string, int, int, error) {
	return f.resumeID, f.resumeSigs, f.resumeQuoru, nil
}

func (f *txHashCaller) LatestBreakerTxHash(_ context.Context, _ string) (string, error) {
	return f.chainTx, f.chainTxErr
}

// newCBTestDB seeds a risk-control row so the state Updates() have a target.
func newCBTestDB(t *testing.T, pair string) *gorm.DB {
	t.Helper()
	db := newTestDB(t, &domain.ScenarioBRiskControlState{}, &domain.CircuitBreakerSignature{})
	db.Create(&domain.ScenarioBRiskControlState{PoolPair: pair, CircuitBreakerState: domain.CircuitBreakerLive})
	return db
}

// latestSignature returns the most recent CircuitBreakerSignature row for a pair.
func latestSignature(t *testing.T, db *gorm.DB, pair string) domain.CircuitBreakerSignature {
	t.Helper()
	var sig domain.CircuitBreakerSignature
	if err := db.Where("control_id = ?", pair).Order("signed_at DESC").First(&sig).Error; err != nil {
		t.Fatalf("no signature row for %s: %v", pair, err)
	}
	return sig
}

// T006 / FR-001 / FR-002: Pause returns its on-chain hash and persists it.
func TestCircuitBreakerService_Pause_ReturnsAndPersistsTxHash(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	db := newCBTestDB(t, pair)
	svc := NewCircuitBreakerService(db, &txHashCaller{pauseTx: "0xpause111"})

	txHash, err := svc.Pause(context.Background(), pair, "cb-bra", "INCIDENT", []byte{0x01})
	if err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	if txHash != "0xpause111" {
		t.Fatalf("expected pause tx hash 0xpause111, got %q", txHash)
	}
	if got := latestSignature(t, db, pair).OnChainTxRef; got != "0xpause111" {
		t.Fatalf("expected persisted OnChainTxRef 0xpause111, got %q", got)
	}
}

// T007 / FR-003 / D-3: ProposeResume returns the proposal ID *and* a distinct tx hash,
// persists the hash, and keeps RequestID carrying the proposal ID.
func TestCircuitBreakerService_ProposeResume_ReturnsBothIDAndTxHash(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	db := newCBTestDB(t, pair)
	svc := NewCircuitBreakerService(db, &txHashCaller{
		proposeID: "0xproposal222",
		proposeTx: "0xtx333",
	})

	requestID, txHash, err := svc.ProposeResume(context.Background(), pair, "cb-bra", []byte{0x01})
	if err != nil {
		t.Fatalf("propose resume failed: %v", err)
	}
	if requestID != "0xproposal222" {
		t.Fatalf("expected request id 0xproposal222, got %q", requestID)
	}
	if txHash != "0xtx333" {
		t.Fatalf("expected tx hash 0xtx333, got %q", txHash)
	}
	if requestID == txHash {
		t.Fatal("request id and tx hash must be distinct identifiers (rule D-3)")
	}

	sig := latestSignature(t, db, pair)
	if sig.OnChainTxRef != "0xtx333" {
		t.Fatalf("expected persisted OnChainTxRef 0xtx333, got %q", sig.OnChainTxRef)
	}
	if sig.RequestID != "0xproposal222" {
		t.Fatalf("RequestID must keep the proposal id needed for signing, got %q", sig.RequestID)
	}
}

// T008 / FR-001 / FR-002: SignResume returns its on-chain hash and persists it.
func TestCircuitBreakerService_SignResume_ReturnsAndPersistsTxHash(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	db := newCBTestDB(t, pair)
	svc := NewCircuitBreakerService(db, &txHashCaller{signTx: "0xsign444"})

	txHash, err := svc.SignResume(context.Background(), pair, "0xproposal222", "cb-arg", []byte{0x02})
	if err != nil {
		t.Fatalf("sign resume failed: %v", err)
	}
	if txHash != "0xsign444" {
		t.Fatalf("expected sign tx hash 0xsign444, got %q", txHash)
	}

	sig := latestSignature(t, db, pair)
	if sig.OnChainTxRef != "0xsign444" {
		t.Fatalf("expected persisted OnChainTxRef 0xsign444, got %q", sig.OnChainTxRef)
	}
	if sig.RequestID != "0xproposal222" {
		t.Fatalf("RequestID must remain the proposal id, got %q", sig.RequestID)
	}
}

// T009 / FR-005 / D-2: GetStatus reports the pair's most recent action hash ordered by
// SignedAt, including when that action was taken by a different institution.
func TestCircuitBreakerService_GetStatus_ReportsLatestActionHashAnyInstitution(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	db := newCBTestDB(t, pair)
	caller := &txHashCaller{
		pauseTx:     "0xpause111",
		proposeID:   "0xproposal222",
		proposeTx:   "0xtx333",
		resumeID:    "0xproposal222",
		resumeSigs:  1,
		resumeQuoru: 2,
	}
	svc := NewCircuitBreakerService(db, caller)
	ctx := context.Background()

	if _, err := svc.Pause(ctx, pair, "cb-bra", "INCIDENT", []byte{0x01}); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	status, err := svc.GetStatus(ctx, pair)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.TxHash != "0xpause111" {
		t.Fatalf("expected status to carry the pause hash, got %q", status.TxHash)
	}

	// A *different* central bank proposes the resume; status must move to that hash.
	if _, _, err := svc.ProposeResume(ctx, pair, "cb-arg", []byte{0x02}); err != nil {
		t.Fatalf("propose resume failed: %v", err)
	}
	status, err = svc.GetStatus(ctx, pair)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.TxHash != "0xtx333" {
		t.Fatalf("expected status to carry the latest action hash 0xtx333 regardless of signer, got %q", status.TxHash)
	}
}

// T010 / FR-009 / D-1: in no-chain mode every action still succeeds and no hash is produced.
func TestCircuitBreakerService_NoChain_ActionsSucceedWithoutTxHash(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	db := newCBTestDB(t, pair)
	// A caller wired to a chain that yields no hash: actions succeed, hashes are empty.
	svc := NewCircuitBreakerService(db, &txHashCaller{})
	ctx := context.Background()

	txHash, err := svc.Pause(ctx, pair, "cb-bra", "INCIDENT", nil)
	if err != nil {
		t.Fatalf("pause must still succeed with no hash: %v", err)
	}
	if txHash != "" {
		t.Fatalf("expected no pause hash, got %q", txHash)
	}

	_, proposeTx, err := svc.ProposeResume(ctx, pair, "cb-bra", nil)
	if err != nil {
		t.Fatalf("propose resume must still succeed with no hash: %v", err)
	}
	if proposeTx != "" {
		t.Fatalf("expected no propose hash, got %q", proposeTx)
	}

	signTx, err := svc.SignResume(ctx, pair, "req-1", "cb-arg", nil)
	if err != nil {
		t.Fatalf("sign resume must still succeed with no hash: %v", err)
	}
	if signTx != "" {
		t.Fatalf("expected no sign hash, got %q", signTx)
	}

	status, err := svc.GetStatus(ctx, pair)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.TxHash != "" {
		t.Fatalf("expected absent status hash in no-chain mode, got %q", status.TxHash)
	}
}

// FR-006 / rule D-2: the reported hash is the pair's latest action on the LEDGER, not the
// latest action this gateway happens to have performed.
//
// This is the topology the deployment actually has and the one the test above cannot reach:
// each Central Bank runs its own gateway and its own database, and records only the actions
// it performed itself. Here this gateway's table holds a pause it made, while the chain has
// since seen a newer action by the counterparty CB. Reading the local table would answer with
// the stale local pause, and the two Central Banks would cite different transactions for the
// same pair — under a portal that tells the operator the value comes from the ledger.
func TestCircuitBreakerService_GetStatus_PrefersLedgerOverThisGatewaysOwnRow(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	db := newCBTestDB(t, pair)
	caller := &txHashCaller{pauseTx: "0xmine-local"}
	svc := NewCircuitBreakerService(db, caller)
	ctx := context.Background()

	if _, err := svc.Pause(ctx, pair, "cb-bra", "INCIDENT", []byte{0x01}); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	// The counterparty CB acts on its own gateway: nothing lands in this database, but the
	// AMM's events move on.
	caller.chainTx = "0xtheirs-on-chain"

	status, err := svc.GetStatus(ctx, pair)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.TxHash != "0xtheirs-on-chain" {
		t.Fatalf("expected the ledger's latest action 0xtheirs-on-chain, got %q", status.TxHash)
	}
}

// A gateway with no AMM wired still reports what it can: its own signature row is the only
// record of a breaker action there, so the fallback must not be dropped along with the
// switch to a ledger-sourced reference.
func TestCircuitBreakerService_GetStatus_FallsBackToLocalRowWithoutChainReference(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	db := newCBTestDB(t, pair)
	// chainTx empty and a lookup error: both mean "the ledger gave nothing".
	caller := &txHashCaller{pauseTx: "0xmine-local", chainTxErr: errors.New("no amm wired")}
	svc := NewCircuitBreakerService(db, caller)
	ctx := context.Background()

	if _, err := svc.Pause(ctx, pair, "cb-bra", "INCIDENT", []byte{0x01}); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	status, err := svc.GetStatus(ctx, pair)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.TxHash != "0xmine-local" {
		t.Fatalf("expected the local row 0xmine-local as fallback, got %q", status.TxHash)
	}
}

// FR-002: an on-chain failure still surfaces as an error, and no hash is invented.
func TestCircuitBreakerService_TxHash_OnChainErrorsSurface(t *testing.T) {
	const pair = "W-BRL-W-ARS"
	ctx := context.Background()
	chainErr := errors.New("chain unreachable")

	db := newCBTestDB(t, pair)
	if _, err := NewCircuitBreakerService(db, &txHashCaller{pauseErr: chainErr}).Pause(ctx, pair, "b", "r", nil); err == nil {
		t.Fatal("expected pause on-chain error")
	}
	if _, _, err := NewCircuitBreakerService(db, &txHashCaller{proposeErr: chainErr}).ProposeResume(ctx, pair, "b", nil); err == nil {
		t.Fatal("expected propose on-chain error")
	}
	if _, err := NewCircuitBreakerService(db, &txHashCaller{signErr: chainErr}).SignResume(ctx, pair, "r", "b", nil); err == nil {
		t.Fatal("expected sign on-chain error")
	}
}
