package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// --- minimal stubs for the orchestrator dependencies ---

type stubSwapRepo struct{}

func (stubSwapRepo) Create(context.Context, *domain.CrossCurrencySwapOperation) error { return nil }
func (stubSwapRepo) GetByID(context.Context, string) (*domain.CrossCurrencySwapOperation, error) {
	return nil, nil
}
func (stubSwapRepo) UpdateStatus(context.Context, string, domain.SwapOperationStatus) error {
	return nil
}
func (stubSwapRepo) UpdateBridgeInPositionID(context.Context, string, string) error { return nil }
func (stubSwapRepo) UpdateSwapResult(context.Context, string, string, string) error { return nil }
func (stubSwapRepo) UpdateBridgeOutPositionID(context.Context, string, string) error {
	return nil
}
func (stubSwapRepo) UpdateFailureReason(context.Context, string, string) error { return nil }

type stubLockMint struct{ called bool }

func (s *stubLockMint) LockAndEnqueue(_ context.Context, _, _, _, _, _, _ string, _ ...string) (*BridgePositionResult, error) {
	s.called = true
	return &BridgePositionResult{PositionID: "local-should-not-happen"}, nil
}

type stubBurnUnlock struct{}

func (stubBurnUnlock) BurnAndEnqueue(context.Context, string, string) (*BridgePositionResult, error) {
	return &BridgePositionResult{}, nil
}
func (stubBurnUnlock) EnqueueBurnAfterSwap(context.Context, string, string, string, string, string, string, ...string) (*BridgePositionResult, error) {
	return &BridgePositionResult{}, nil
}

type stubSwapService struct{}

// Execute fails on purpose so Execute returns right after Step 1/2 — the test only asserts
// which bridge-in path was taken, not the full swap.
func (stubSwapService) Execute(context.Context, SwapRequest) (*SwapResult, error) {
	return nil, errors.New("swap stub: stop after bridge-in")
}

type stubPoolActive struct{}

func (stubPoolActive) IsActive(context.Context, string) (bool, error) { return true, nil }

type stubCBOK struct{}

func (stubCBOK) IsHalted(context.Context, string) (bool, error) { return false, nil }

type stubBridgeInRelay struct {
	called bool
	gotReq CrossCurrencyBridgeInRequest
}

func (s *stubBridgeInRelay) NotifyBridgeIn(_ context.Context, req CrossCurrencyBridgeInRequest) (string, error) {
	s.called = true
	s.gotReq = req
	return "cb-position-123", nil
}

// TestExecute_BridgeIn_DelegatesToCBWhenRelayConfigured asserts the sovereignty fix:
// when a bridge-in relay is wired, Step 1 delegates the W-<source> mint to the issuing CB
// and never calls the local lock-mint service (which would mint with a non-CB signer).
func TestExecute_BridgeIn_DelegatesToCBWhenRelayConfigured(t *testing.T) {
	local := &stubLockMint{}
	relay := &stubBridgeInRelay{}

	orch := NewCrossCurrencySwapOrchestrator(
		stubSwapRepo{},
		nil, // quoteRepo (optional)
		local,
		stubBurnUnlock{},
		stubSwapService{},
		stubPoolActive{},
		stubCBOK{},
		nil, // rollbackCoordinator (optional)
		nil, // bridgeAssets (optional)
		nil, // bridgePoller (not used on the delegated path)
	).WithBridgeInRelay(relay).WithHubSignerAddress("0xSWAPSIGNER")

	_, _ = orch.Execute(context.Background(), CrossCurrencySwapRequest{
		SwapID:         "swap-1",
		CorrelationID:  "corr-1",
		SourceCurrency: "BRL",
		TargetCurrency: "ARS",
		PoolPair:       "W-BRL-ARS",
		AmountOut:      "100",
		MaxAmountIn:    "1000",
		PayerBankID:    "bank-a",
	})

	if !relay.called {
		t.Fatalf("expected bridge-in to be delegated to the CB relay, but relay was not called")
	}
	if local.called {
		t.Fatalf("expected local lock-mint NOT to be called when a bridge-in relay is configured")
	}
	if relay.gotReq.SwapSenderAddress != "0xSWAPSIGNER" {
		t.Fatalf("expected bridge-in to mint to the gateway swap signer 0xSWAPSIGNER, got %q", relay.gotReq.SwapSenderAddress)
	}
	if relay.gotReq.Amount != "1000" {
		t.Fatalf("expected bridge-in amount to be MaxAmountIn (1000), got %q", relay.gotReq.Amount)
	}
}

// TestExecute_BridgeIn_LocalPathWhenNoRelay asserts the CB self-service path still works:
// with no relay, Step 1 uses the local lock-mint and polls the local bridge state.
func TestExecute_BridgeIn_LocalPathWhenNoRelay(t *testing.T) {
	local := &stubLockMint{}

	orch := NewCrossCurrencySwapOrchestrator(
		stubSwapRepo{},
		nil,
		local,
		stubBurnUnlock{},
		stubSwapService{},
		stubPoolActive{},
		stubCBOK{},
		nil,
		nil,
		stubActivePoller{},
	).WithHubSignerAddress("0xCBSIGNER")

	_, _ = orch.Execute(context.Background(), CrossCurrencySwapRequest{
		SwapID:        "swap-2",
		CorrelationID: "corr-2",
		PoolPair:      "W-BRL-ARS",
		AmountOut:     "100",
		MaxAmountIn:   "1000",
		PayerBankID:   "central-bank-a",
	})

	if !local.called {
		t.Fatalf("expected local lock-mint to be called when no bridge-in relay is configured")
	}
}

type stubActivePoller struct{}

func (stubActivePoller) GetBridgeState(context.Context, string) (domain.BridgeState, error) {
	return domain.BridgeStateActive, nil
}

// stubActiveThenReleasedPoller satisfies both bridge waits in the local CB self-service path:
// the bridge-in step polls until ACTIVE, the bridge-out step polls until RELEASED. It reports
// ACTIVE on the first call and RELEASED thereafter, so a test that runs the full happy path
// does not block on the 120s waitForBridgeUnlocked timeout (a fixed-ACTIVE poller would, since
// bridge-out never observes RELEASED).
type stubActiveThenReleasedPoller struct{ calls int }

func (p *stubActiveThenReleasedPoller) GetBridgeState(context.Context, string) (domain.BridgeState, error) {
	p.calls++
	if p.calls == 1 {
		return domain.BridgeStateActive, nil
	}
	return domain.BridgeStateReleased, nil
}

// fixedAmountInSwap returns a swap whose realized amount_in is whatever the test sets,
// modelling the post-fix client that decodes the true amount from LogSwap rather than
// echoing MaxAmountIn.
type fixedAmountInSwap struct{ amountIn string }

func (s fixedAmountInSwap) Execute(context.Context, SwapRequest) (*SwapResult, error) {
	return &SwapResult{TxHash: "0xdeadbeef", AmountIn: s.amountIn, OrderID: "ord-1"}, nil
}

// TestExecute_SlippageCheckFiresOnRealizedAmountIn locks in the Task 2 fix: the orchestrator's
// post-trade slippage guard must reject a swap whose realized amount_in exceeds MaxAmountIn.
// This path was dead before the fix because the AMM client echoed MaxAmountIn as amount_in,
// making actual == max by construction (the check could never trip).
func TestExecute_SlippageCheckFiresOnRealizedAmountIn(t *testing.T) {
	orch := NewCrossCurrencySwapOrchestrator(
		stubSwapRepo{},
		nil,
		&stubLockMint{},
		stubBurnUnlock{},
		fixedAmountInSwap{amountIn: "1500"}, // realized cost above the 1000 cap
		stubPoolActive{},
		stubCBOK{},
		nil, // rollbackCoordinator optional; slippage still returns an error without it
		nil,
		stubActivePoller{},
	).WithHubSignerAddress("0xCBSIGNER")

	_, err := orch.Execute(context.Background(), CrossCurrencySwapRequest{
		SwapID:        "swap-slip",
		CorrelationID: "corr-slip",
		PoolPair:      "W-BRL-ARS",
		AmountOut:     "100",
		MaxAmountIn:   "1000",
		PayerBankID:   "central-bank-a",
	})

	if err == nil {
		t.Fatal("expected slippage error when realized amount_in (1500) exceeds max (1000), got nil")
	}
	if !strings.Contains(err.Error(), "slippage") {
		t.Fatalf("expected a slippage error, got: %v", err)
	}
}

// TestExecute_SlippageCheckPassesWithinCap is the companion: a realized amount_in at or under
// the cap must NOT trip the guard (it then proceeds to bridge-out).
func TestExecute_SlippageCheckPassesWithinCap(t *testing.T) {
	orch := NewCrossCurrencySwapOrchestrator(
		stubSwapRepo{},
		nil,
		&stubLockMint{},
		stubBurnUnlock{},
		fixedAmountInSwap{amountIn: "800"}, // realized cost under the 1000 cap
		stubPoolActive{},
		stubCBOK{},
		nil,
		nil,
		&stubActiveThenReleasedPoller{}, // ACTIVE for bridge-in, then RELEASED for bridge-out
	).WithHubSignerAddress("0xCBSIGNER")

	_, err := orch.Execute(context.Background(), CrossCurrencySwapRequest{
		SwapID:        "swap-ok",
		CorrelationID: "corr-ok",
		PoolPair:      "W-BRL-ARS",
		AmountOut:     "100",
		MaxAmountIn:   "1000",
		PayerBankID:   "central-bank-a",
	})

	if err != nil && strings.Contains(err.Error(), "slippage") {
		t.Fatalf("did not expect a slippage error for amount_in within cap, got: %v", err)
	}
}
