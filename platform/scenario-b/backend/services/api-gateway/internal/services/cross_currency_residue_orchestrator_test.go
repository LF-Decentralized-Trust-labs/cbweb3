// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// --- stubs for Step 4 (residue return) ---

type recordingResidueRelay struct {
	called   bool
	gotReq   CrossCurrencyResidueReturnRequest
	failWith error
}

func (s *recordingResidueRelay) NotifyResidueReturn(_ context.Context, req CrossCurrencyResidueReturnRequest) (string, error) {
	s.called = true
	s.gotReq = req
	if s.failWith != nil {
		return "", s.failWith
	}
	return "cb-residue-pos-1", nil
}

// recordingBurnUnlock captures both burn legs so a test can tell a settlement apart from a
// residue return.
type recordingBurnUnlock struct {
	residueCalls int
	residue      struct {
		amount      string
		spoke       string
		burnFrom    string
		beneficiary string
		swapTxHash  string
		parentID    string
	}
	residueErr error
}

func (s *recordingBurnUnlock) BurnAndEnqueue(context.Context, string, string) (*BridgePositionResult, error) {
	return &BridgePositionResult{}, nil
}

func (s *recordingBurnUnlock) EnqueueBurnAfterSwap(context.Context, string, string, string, string, string, string, ...string) (*BridgePositionResult, error) {
	return &BridgePositionResult{PositionID: "settlement-pos"}, nil
}

func (s *recordingBurnUnlock) EnqueueResidueReturn(
	_ context.Context,
	_, spokeNetwork, _, _, amount, _ string,
	burnFrom, beneficiary, swapTxHash, parentPositionID string,
) (*BridgePositionResult, error) {
	s.residueCalls++
	s.residue.amount = amount
	s.residue.spoke = spokeNetwork
	s.residue.burnFrom = burnFrom
	s.residue.beneficiary = beneficiary
	s.residue.swapTxHash = swapTxHash
	s.residue.parentID = parentPositionID
	if s.residueErr != nil {
		return nil, s.residueErr
	}
	return &BridgePositionResult{PositionID: "local-residue-pos"}, nil
}

type recordingLimitChecker struct {
	deducted []string
	restored []string
}

func (c *recordingLimitChecker) CheckAndDeduct(_ context.Context, _, _, amountHuman string) error {
	c.deducted = append(c.deducted, amountHuman)
	return nil
}

func (c *recordingLimitChecker) Restore(_ context.Context, _, _, amountHuman string) {
	c.restored = append(c.restored, amountHuman)
}

type stubWalletResolver struct {
	addr string
	err  error
}

func (s stubWalletResolver) ResolveWalletAddress(context.Context, string) (string, error) {
	return s.addr, s.err
}

// alwaysReleasedPoller satisfies the Step 3 wait without the intermediate ACTIVE tick, so
// tests on the sovereign path (where bridge-in does not poll) do not sleep.
type alwaysReleasedPoller struct{}

func (alwaysReleasedPoller) GetBridgeState(context.Context, string) (domain.BridgeState, error) {
	return domain.BridgeStateReleased, nil
}

// residueReq bridges 1000 and (with fixedAmountInSwap{amountIn:"800"}) consumes 800,
// leaving 200 to return.
func residueReq() CrossCurrencySwapRequest {
	return CrossCurrencySwapRequest{
		SwapID:         "swap-res-1",
		CorrelationID:  "corr-res-1",
		SourceCurrency: "BRL",
		TargetCurrency: "ARS",
		PoolPair:       "W-BRL-ARS",
		AmountOut:      "100",
		MaxAmountIn:    "1000",
		PayerBankID:    "bank-a",
	}
}

// On the sovereign path the return is delegated to the issuing CB, and the request carries
// the swap tx and the bridge-in position — never an amount, which the CB derives itself.
func TestResidueReturn_DelegatesToCBWhenRelayConfigured(t *testing.T) {
	repo := &statefulSwapRepo{}
	relay := &recordingResidueRelay{}
	burn := &recordingBurnUnlock{}

	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil,
		&stubLockMint{}, burn, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, alwaysReleasedPoller{},
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithResidueReturnRelay(relay).
		WithHubSignerAddress("0xSWAPSIGNER")

	res, err := orch.Execute(context.Background(), residueReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != domain.SwapStatusCompleted {
		t.Fatalf("expected COMPLETED, got %s", res.Status)
	}
	if !relay.called {
		t.Fatal("expected the residue return to be delegated to the CB relay")
	}
	if burn.residueCalls != 0 {
		t.Fatalf("expected no local residue enqueue when a relay is configured, got %d", burn.residueCalls)
	}
	if res.ResidueAmount != "200" {
		t.Fatalf("expected residue 200 (1000 bridged − 800 consumed), got %q", res.ResidueAmount)
	}
	if res.ResidueStatus != domain.ResidueReturnEnqueued {
		t.Fatalf("expected RETURN_ENQUEUED, got %s", res.ResidueStatus)
	}
	if res.ResiduePositionID != "cb-residue-pos-1" {
		t.Fatalf("expected the CB-side position id, got %q", res.ResiduePositionID)
	}
	if relay.gotReq.BridgeInPositionID != "cb-position-123" {
		t.Fatalf("expected the bridge-in position to be referenced, got %q", relay.gotReq.BridgeInPositionID)
	}
	if relay.gotReq.SwapTxHash != "0xdeadbeef" {
		t.Fatalf("expected the swap tx hash to be referenced, got %q", relay.gotReq.SwapTxHash)
	}
	if relay.gotReq.SpokeIn != "spoke-brl" {
		t.Fatalf("expected spoke-brl (derived from source currency), got %q", relay.gotReq.SpokeIn)
	}
	if relay.gotReq.PayerBankID != "bank-a" {
		t.Fatalf("expected payer bank-a, got %q", relay.gotReq.PayerBankID)
	}
	// The persisted record must agree with the returned result.
	if repo.op.ResidueAmount != "200" || repo.op.ResidueStatus != domain.ResidueReturnEnqueued {
		t.Fatalf("persisted residue mismatch: amount=%q status=%s", repo.op.ResidueAmount, repo.op.ResidueStatus)
	}
}

// On the local path (this gateway is the issuing CB) the return is enqueued directly, burned
// from the Hub swap signer and delivered to the payer's own spoke wallet.
func TestResidueReturn_LocalPathEnqueuesBurnToPayerWallet(t *testing.T) {
	repo := &statefulSwapRepo{}
	burn := &recordingBurnUnlock{}

	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil,
		&stubLockMint{}, burn, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, &stubActiveThenReleasedPoller{},
	).WithHubSignerAddress("0xCBSIGNER").
		WithPayerWalletResolver(stubWalletResolver{addr: "0xPAYERWALLET"})

	res, err := orch.Execute(context.Background(), residueReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if burn.residueCalls != 1 {
		t.Fatalf("expected exactly one residue enqueue, got %d", burn.residueCalls)
	}
	if burn.residue.amount != "200" {
		t.Fatalf("expected residue 200, got %q", burn.residue.amount)
	}
	if burn.residue.burnFrom != "0xCBSIGNER" {
		t.Fatalf("expected burn from the hub signer, got %q", burn.residue.burnFrom)
	}
	if burn.residue.beneficiary != "0xPAYERWALLET" {
		t.Fatalf("expected delivery to the payer wallet, got %q", burn.residue.beneficiary)
	}
	if burn.residue.spoke != "spoke-brl" {
		t.Fatalf("expected the source spoke, got %q", burn.residue.spoke)
	}
	if burn.residue.swapTxHash != "0xdeadbeef" {
		t.Fatalf("expected the swap tx hash for idempotency, got %q", burn.residue.swapTxHash)
	}
	if burn.residue.parentID == "" {
		t.Fatal("expected the residue leg to link to its bridge-in parent position")
	}
	if res.ResiduePositionID != "local-residue-pos" {
		t.Fatalf("expected the local position id, got %q", res.ResiduePositionID)
	}
}

// A swap that consumed the whole cap leaves nothing to return.
func TestResidueReturn_SkippedWhenFullCapConsumed(t *testing.T) {
	repo := &statefulSwapRepo{}
	burn := &recordingBurnUnlock{}
	relay := &recordingResidueRelay{}

	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil,
		&stubLockMint{}, burn, fixedAmountInSwap{amountIn: "1000"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, &stubActiveThenReleasedPoller{},
	).WithResidueReturnRelay(relay).WithHubSignerAddress("0xCBSIGNER")

	res, err := orch.Execute(context.Background(), residueReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if relay.called || burn.residueCalls != 0 {
		t.Fatal("expected no residue return when the swap consumed the full cap")
	}
	if res.ResidueAmount != "0" {
		t.Fatalf("expected residue 0, got %q", res.ResidueAmount)
	}
	if res.ResidueStatus != domain.ResidueNone {
		t.Fatalf("expected NONE, got %s", res.ResidueStatus)
	}
}

// The payment is already final when Step 4 runs. A failed return must be recorded for
// reconciliation, never reopen the swap nor surface as an error to the payer.
func TestResidueReturn_FailureDoesNotFailTheSwap(t *testing.T) {
	repo := &statefulSwapRepo{}
	relay := &recordingResidueRelay{failWith: errors.New("CB unreachable")}

	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil,
		&stubLockMint{}, &recordingBurnUnlock{}, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, alwaysReleasedPoller{},
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithResidueReturnRelay(relay).
		WithHubSignerAddress("0xSWAPSIGNER")

	res, err := orch.Execute(context.Background(), residueReq())
	if err != nil {
		t.Fatalf("a failed residue return must not fail the swap, got error: %v", err)
	}
	if res.Status != domain.SwapStatusCompleted {
		t.Fatalf("expected the swap to stay COMPLETED, got %s", res.Status)
	}
	if res.ResidueStatus != domain.ResidueReturnFailed {
		t.Fatalf("expected RETURN_FAILED, got %s", res.ResidueStatus)
	}
	if res.ResiduePositionID != "" {
		t.Fatalf("expected no position id on failure, got %q", res.ResiduePositionID)
	}
	if repo.op.Status != domain.SwapStatusCompleted {
		t.Fatalf("persisted status must remain COMPLETED, got %s", repo.op.Status)
	}
	if repo.op.ResidueAmount != "200" {
		t.Fatalf("the stranded amount must be recorded for reconciliation, got %q", repo.op.ResidueAmount)
	}
}

// An unresolvable payer wallet on the local path is reported, not silently enqueued into a
// delivery path that cannot run.
func TestResidueReturn_LocalPathWalletResolutionFailureIsRecorded(t *testing.T) {
	repo := &statefulSwapRepo{}
	burn := &recordingBurnUnlock{}

	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil,
		&stubLockMint{}, burn, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, &stubActiveThenReleasedPoller{},
	).WithHubSignerAddress("0xCBSIGNER").
		WithPayerWalletResolver(stubWalletResolver{err: errors.New("bank not registered")})

	res, err := orch.Execute(context.Background(), residueReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if burn.residueCalls != 0 {
		t.Fatal("expected no enqueue when the payer wallet cannot be resolved")
	}
	if res.ResidueStatus != domain.ResidueReturnFailed {
		t.Fatalf("expected RETURN_FAILED, got %s", res.ResidueStatus)
	}
}

// The daily quota reserves the worst case before Step 1 because the real cost is unknown.
// Once known, the unused part must go back — otherwise every swap silently burns the
// slippage buffer out of the bank's limit.
func TestResidueReturn_RestoresUnusedDailyQuota(t *testing.T) {
	limits := &recordingLimitChecker{}
	orch := NewCrossCurrencySwapOrchestrator(
		&statefulSwapRepo{}, nil,
		&stubLockMint{}, &recordingBurnUnlock{}, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, &stubActiveThenReleasedPoller{},
	).WithHubSignerAddress("0xCBSIGNER").
		WithTransferLimitChecker(limits).
		WithPayerWalletResolver(stubWalletResolver{addr: "0xPAYERWALLET"})

	if _, err := orch.Execute(context.Background(), residueReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(limits.deducted) != 1 || limits.deducted[0] != "1000" {
		t.Fatalf("expected the worst case to be reserved once, got %v", limits.deducted)
	}
	if len(limits.restored) != 1 {
		t.Fatalf("expected exactly one restore (no double-restore from the failure defer), got %v", limits.restored)
	}
	if limits.restored[0] != "200" {
		t.Fatalf("expected the unused 200 to be restored, got %q", limits.restored[0])
	}
}

// failingCactiRelay makes Step 3 fail after a successful swap — the partial-success path.
type failingCactiRelay struct{ err error }

func (f failingCactiRelay) NotifyBridgeOut(context.Context, CactiCrossCurrencyBridgeOutRequest) (string, error) {
	return "", f.err
}

// The unspent input was never owed to anyone, so a bridge-out failure must not strand it on
// top of an undelivered payment. Step 4 runs on the Step 3 failure path too.
func TestResidueReturn_RunsWhenBridgeOutFails(t *testing.T) {
	repo := &statefulSwapRepo{}
	relay := &recordingResidueRelay{}

	orch := NewCrossCurrencySwapOrchestrator(
		repo, nil,
		&stubLockMint{}, &recordingBurnUnlock{}, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, alwaysReleasedPoller{},
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithCactiRelay(failingCactiRelay{err: errors.New("cacti unreachable")}).
		WithResidueReturnRelay(relay).
		WithHubSignerAddress("0xSWAPSIGNER")

	_, err := orch.Execute(context.Background(), residueReq())
	if err == nil {
		t.Fatal("expected the bridge-out failure to surface to the caller")
	}
	if !relay.called {
		t.Fatal("expected the residue to be returned even though bridge-out failed")
	}
	if repo.op.ResidueAmount != "200" {
		t.Fatalf("expected the residue to be recorded, got %q", repo.op.ResidueAmount)
	}
	if repo.op.ResidueStatus != domain.ResidueReturnEnqueued {
		t.Fatalf("expected RETURN_ENQUEUED, got %s", repo.op.ResidueStatus)
	}
	// The delivery leg genuinely failed — that verdict must stand.
	if repo.op.Status != domain.SwapStatusFailed {
		t.Fatalf("expected the swap to stay FAILED, got %s", repo.op.Status)
	}
}

// The residue is returned exactly once, never twice, on any single Execute.
func TestResidueReturn_RunsAtMostOncePerSwap(t *testing.T) {
	relay := &recordingResidueRelay{}
	countingRelay := &countingResidueRelay{inner: relay}

	orch := NewCrossCurrencySwapOrchestrator(
		&statefulSwapRepo{}, nil,
		&stubLockMint{}, &recordingBurnUnlock{}, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, alwaysReleasedPoller{},
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithResidueReturnRelay(countingRelay).
		WithHubSignerAddress("0xSWAPSIGNER")

	if _, err := orch.Execute(context.Background(), residueReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if countingRelay.calls != 1 {
		t.Fatalf("expected exactly one residue return, got %d", countingRelay.calls)
	}
}

type countingResidueRelay struct {
	inner *recordingResidueRelay
	calls int
}

func (c *countingResidueRelay) NotifyResidueReturn(ctx context.Context, req CrossCurrencyResidueReturnRequest) (string, error) {
	c.calls++
	return c.inner.NotifyResidueReturn(ctx, req)
}

// A swap that releases the unused buffer and *then* fails in Step 3 must not have that buffer
// restored twice: the failure path owes only the part that was still reserved.
func TestResidueReturn_NoDoubleQuotaRestoreWhenBridgeOutFails(t *testing.T) {
	limits := &recordingLimitChecker{}
	orch := NewCrossCurrencySwapOrchestrator(
		&statefulSwapRepo{}, nil,
		&stubLockMint{}, &recordingBurnUnlock{}, fixedAmountInSwap{amountIn: "800"},
		stubPoolActive{}, stubCBOK{},
		nil, nil, alwaysReleasedPoller{},
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithCactiRelay(failingCactiRelay{err: errors.New("cacti unreachable")}).
		WithResidueReturnRelay(&recordingResidueRelay{}).
		WithTransferLimitChecker(limits).
		WithHubSignerAddress("0xSWAPSIGNER")

	if _, err := orch.Execute(context.Background(), residueReq()); err == nil {
		t.Fatal("expected the bridge-out failure to surface")
	}

	// Reserved 1000; released 200 as unused buffer; the failure owes the remaining 800.
	if len(limits.deducted) != 1 || limits.deducted[0] != "1000" {
		t.Fatalf("expected a single reservation of 1000, got %v", limits.deducted)
	}
	total := new(big.Int)
	for _, r := range limits.restored {
		v, ok := new(big.Int).SetString(r, 10)
		if !ok {
			t.Fatalf("unparseable restore amount %q", r)
		}
		total.Add(total, v)
	}
	if total.String() != "1000" {
		t.Fatalf("total restored must equal the reservation (1000), got %s from %v", total, limits.restored)
	}
}

// On a failed swap the whole reservation goes back through the existing failure path, and the
// residue restore must not add to it.
func TestResidueReturn_FailedSwapRestoresFullReservationOnce(t *testing.T) {
	limits := &recordingLimitChecker{}
	orch := NewCrossCurrencySwapOrchestrator(
		&statefulSwapRepo{}, nil,
		&stubLockMint{}, &recordingBurnUnlock{}, stubSwapService{}, // swap stub always errors
		stubPoolActive{}, stubCBOK{},
		nil, nil, &stubActiveThenReleasedPoller{},
	).WithHubSignerAddress("0xCBSIGNER").
		WithTransferLimitChecker(limits)

	if _, err := orch.Execute(context.Background(), residueReq()); err == nil {
		t.Fatal("expected the swap to fail")
	}

	if len(limits.restored) != 1 || limits.restored[0] != "1000" {
		t.Fatalf("expected a single full-reservation restore on failure, got %v", limits.restored)
	}
}
