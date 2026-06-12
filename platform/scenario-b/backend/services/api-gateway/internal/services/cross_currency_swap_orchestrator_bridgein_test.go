// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// --- minimal stubs for the orchestrator dependencies ---

type stubSwapRepo struct{}

func (stubSwapRepo) Create(context.Context, *domain.CrossCurrencySwapOperation) error    { return nil }
func (stubSwapRepo) GetByID(context.Context, string) (*domain.CrossCurrencySwapOperation, error) {
	return nil, nil
}
func (stubSwapRepo) UpdateStatus(context.Context, string, domain.SwapOperationStatus) error { return nil }
func (stubSwapRepo) UpdateBridgeInPositionID(context.Context, string, string) error         { return nil }
func (stubSwapRepo) UpdateAmountIn(context.Context, string, string) error                   { return nil }
func (stubSwapRepo) UpdateSwapTxHash(context.Context, string, string) error                 { return nil }
func (stubSwapRepo) UpdateBridgeOutPositionID(context.Context, string, string) error        { return nil }
func (stubSwapRepo) UpdateFailureReason(context.Context, string, string) error              { return nil }

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
