// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"
)

// --- stubs specific to the Step 2 delegation ---

type stubHubSwapRelay struct {
	called   bool
	gotReq   CrossCurrencyHubSwapRequest
	result   *SwapResult
	failWith error
}

func (s *stubHubSwapRelay) ExecuteHubSwap(_ context.Context, req CrossCurrencyHubSwapRequest) (*SwapResult, error) {
	s.called = true
	s.gotReq = req
	if s.failWith != nil {
		return nil, s.failWith
	}
	return s.result, nil
}

// recordingSwapService reports whether the local (in-process) swap path ran. On a delegating
// gateway it must never run: executing locally requires a Hub signing key, which is exactly
// what the delegation removes.
type recordingSwapService struct{ called bool }

func (s *recordingSwapService) Execute(context.Context, SwapRequest) (*SwapResult, error) {
	s.called = true
	return nil, errors.New("local swap stub: must not be reached on the delegated path")
}

// stubCactiRelay captures the bridge-out request so the test can assert which Hub address
// the beneficiary CB is told to burn from.
type stubCactiRelay struct {
	called bool
	gotReq CactiCrossCurrencyBridgeOutRequest
}

func (s *stubCactiRelay) NotifyBridgeOut(_ context.Context, req CactiCrossCurrencyBridgeOutRequest) (string, error) {
	s.called = true
	s.gotReq = req
	return "echo-1", nil
}

func hubSwapOrchestrator(swapSvc SwapServiceIface) *CrossCurrencySwapOrchestrator {
	return NewCrossCurrencySwapOrchestrator(
		stubSwapRepo{},
		nil, // quoteRepo (optional)
		&stubLockMint{},
		stubBurnUnlock{},
		swapSvc,
		stubPoolActive{},
		stubCBOK{},
		nil, // rollbackCoordinator (optional)
		nil, // bridgeAssets (optional)
		nil, // bridgePoller (not used on the delegated path)
	).WithBridgeInRelay(&stubBridgeInRelay{})
}

func hubSwapRequest() CrossCurrencySwapRequest {
	return CrossCurrencySwapRequest{
		SwapID:            "swap-1",
		CorrelationID:     "corr-1",
		SourceCurrency:    "BRL",
		TargetCurrency:    "COP",
		PoolPair:          "W-BRL-W-COP",
		AmountOut:         "500",
		MaxAmountIn:       "1000",
		PayerBankID:       "bank-a",
		BeneficiaryBankID: "bank-b",
	}
}

// TestExecute_HubSwap_DelegatesToCBWhenRelayConfigured is the core sovereignty assertion for
// Step 2: with a hub-swap relay wired, the trade is executed by the issuing CB and the local
// AMM path — which would require this gateway to hold the CB's key — is never touched.
func TestExecute_HubSwap_DelegatesToCBWhenRelayConfigured(t *testing.T) {
	local := &recordingSwapService{}
	relay := &stubHubSwapRelay{result: &SwapResult{
		TxHash: "0xswap", AmountIn: "800", HubSenderAddress: "0xCBHUB",
	}}

	orch := hubSwapOrchestrator(local).WithHubSwapRelay(relay)

	_, _ = orch.Execute(context.Background(), hubSwapRequest())

	if !relay.called {
		t.Fatalf("expected Step 2 to be delegated to the CB, but the relay was not called")
	}
	if local.called {
		t.Fatalf("expected the local AMM swap NOT to run when a hub-swap relay is configured")
	}
	if relay.gotReq.BridgeInPositionID != "cb-position-123" {
		t.Fatalf("expected the funding bridge-in position to be forwarded, got %q", relay.gotReq.BridgeInPositionID)
	}
	if relay.gotReq.MaxAmountIn != "1000" || relay.gotReq.AmountOut != "500" {
		t.Fatalf("expected amounts to be forwarded unchanged, got out=%q max_in=%q", relay.gotReq.AmountOut, relay.gotReq.MaxAmountIn)
	}
	if relay.gotReq.TargetCurrency != "COP" || relay.gotReq.PoolPair != "W-BRL-W-COP" {
		t.Fatalf("expected corridor to be forwarded so the CB resolves direction, got pair=%q target=%q",
			relay.gotReq.PoolPair, relay.gotReq.TargetCurrency)
	}
	if relay.gotReq.PayerBankID != "bank-a" {
		t.Fatalf("expected the payer bank to be named for the CB's compliance re-check, got %q", relay.gotReq.PayerBankID)
	}
}

// TestExecute_HubSwap_BridgeOutBurnsFromTheExecutingCBAddress guards the address that broke
// when Step 2 moved to the CB: the swap output lands on the CB's Hub address, so the
// beneficiary CB must be told to burn from there — not from this gateway's own signer, which
// on a delegating bank holds nothing.
func TestExecute_HubSwap_BridgeOutBurnsFromTheExecutingCBAddress(t *testing.T) {
	cacti := &stubCactiRelay{}
	relay := &stubHubSwapRelay{result: &SwapResult{
		TxHash: "0xswap", AmountIn: "800", HubSenderAddress: "0xCBHUB",
	}}

	orch := hubSwapOrchestrator(&recordingSwapService{}).
		WithHubSwapRelay(relay).
		WithCactiRelay(cacti).
		// A stale local signer must lose to the address reported by the executing CB.
		WithHubSignerAddress("0xSTALEBANKSIGNER")

	_, err := orch.Execute(context.Background(), hubSwapRequest())
	if err != nil {
		t.Fatalf("expected the delegated swap to settle, got %v", err)
	}
	if !cacti.called {
		t.Fatalf("expected Step 3 to be dispatched to the beneficiary CB")
	}
	if cacti.gotReq.SwapSenderAddress != "0xCBHUB" {
		t.Fatalf("expected bridge-out to burn from the executing CB address 0xCBHUB, got %q", cacti.gotReq.SwapSenderAddress)
	}
	if cacti.gotReq.SwapTxHash != "0xswap" {
		t.Fatalf("expected the on-chain swap tx hash to be forwarded, got %q", cacti.gotReq.SwapTxHash)
	}
}

// TestExecute_HubSwap_LocalPathKeepsItsOwnSignerAddress pins the unchanged behaviour for a CB
// running the swap itself: with no relay, the gateway's own signer holds the output.
func TestExecute_HubSwap_LocalPathKeepsItsOwnSignerAddress(t *testing.T) {
	cacti := &stubCactiRelay{}
	local := &fixedSwapService{result: &SwapResult{TxHash: "0xlocal", AmountIn: "800"}}

	orch := hubSwapOrchestrator(local).
		WithCactiRelay(cacti).
		WithHubSignerAddress("0xOWNSIGNER")

	_, err := orch.Execute(context.Background(), hubSwapRequest())
	if err != nil {
		t.Fatalf("expected the local swap to settle, got %v", err)
	}
	if !local.called {
		t.Fatalf("expected the local AMM swap to run when no hub-swap relay is configured")
	}
	if cacti.gotReq.SwapSenderAddress != "0xOWNSIGNER" {
		t.Fatalf("expected bridge-out to burn from this gateway's own signer, got %q", cacti.gotReq.SwapSenderAddress)
	}
}

// TestExecute_HubSwap_DelegationFailureFailsTheSwap asserts a failed delegation is a failed
// swap: nothing was traded, so the flow must not proceed to bridge-out.
func TestExecute_HubSwap_DelegationFailureFailsTheSwap(t *testing.T) {
	cacti := &stubCactiRelay{}
	relay := &stubHubSwapRelay{failWith: errors.New("central bank hub-swap returned HTTP 422")}

	orch := hubSwapOrchestrator(&recordingSwapService{}).
		WithHubSwapRelay(relay).
		WithCactiRelay(cacti)

	_, err := orch.Execute(context.Background(), hubSwapRequest())
	if err == nil {
		t.Fatalf("expected a failed delegation to fail the swap")
	}
	if cacti.called {
		t.Fatalf("expected no bridge-out after a failed swap")
	}
}

// fixedSwapService succeeds with a canned result so the local path can reach Step 3.
type fixedSwapService struct {
	called bool
	result *SwapResult
}

func (s *fixedSwapService) Execute(context.Context, SwapRequest) (*SwapResult, error) {
	s.called = true
	return s.result, nil
}

// stubConsumptionRecorder captures what a locally executed trade recorded against its funding
// position — the figure the issuing CB's Hub reconciliation reads as `consumed`.
type stubConsumptionRecorder struct {
	claimed    *HubSwapRecord
	finalized  [3]string // positionID, amountIn, txHash
	alreadyOwn *HubSwapRecord
	claimErr   error
}

func (s *stubConsumptionRecorder) Claim(_ context.Context, rec *HubSwapRecord) (*HubSwapRecord, bool, error) {
	if s.claimErr != nil {
		return nil, false, s.claimErr
	}
	if s.alreadyOwn != nil {
		return s.alreadyOwn, false, nil
	}
	s.claimed = rec
	return nil, true, nil
}

func (s *stubConsumptionRecorder) Finalize(_ context.Context, positionID, amountIn, txHash string) (*HubSwapRecord, error) {
	s.finalized = [3]string{positionID, amountIn, txHash}
	return &HubSwapRecord{BridgeInPositionID: positionID, AmountIn: amountIn, SwapTxHash: txHash}, nil
}

// A CB that runs Step 2 itself must record what the trade cost, in the same place the delegated path
// does. Without it the Hub reconciliation reads the position as never swapped, claims its whole mint
// is still on the Hub, and reports a negative figure once the residue comes back.
func TestExecute_HubSwap_LocalPathRecordsWhatTheTradeCost(t *testing.T) {
	recorder := &stubConsumptionRecorder{}
	local := &fixedSwapService{result: &SwapResult{TxHash: "0xlocal", AmountIn: "800"}}

	orch := hubSwapOrchestrator(local).
		WithCactiRelay(&stubCactiRelay{}).
		WithHubSignerAddress("0xOWNSIGNER").
		WithHubSwapConsumptionRecorder(recorder)

	if _, err := orch.Execute(context.Background(), hubSwapRequest()); err != nil {
		t.Fatalf("expected the local swap to settle, got %v", err)
	}
	if recorder.claimed == nil {
		t.Fatal("the local trade must be recorded against its funding position")
	}
	if recorder.claimed.BridgeInPositionID != "cb-position-123" {
		t.Fatalf("recorded against %q; want the funding bridge-in position", recorder.claimed.BridgeInPositionID)
	}
	if recorder.finalized != [3]string{"cb-position-123", "800", "0xlocal"} {
		t.Fatalf("finalized with %v; want the realized cost and tx hash", recorder.finalized)
	}
}

// Bookkeeping must never fail a completed payment: the trade is on-chain by then, and the missing
// record is exactly what the reconciliation reports as unattributed.
func TestExecute_HubSwap_LocalPathSettlesEvenIfRecordingFails(t *testing.T) {
	recorder := &stubConsumptionRecorder{claimErr: errors.New("db down")}
	local := &fixedSwapService{result: &SwapResult{TxHash: "0xlocal", AmountIn: "800"}}

	orch := hubSwapOrchestrator(local).
		WithCactiRelay(&stubCactiRelay{}).
		WithHubSignerAddress("0xOWNSIGNER").
		WithHubSwapConsumptionRecorder(recorder)

	if _, err := orch.Execute(context.Background(), hubSwapRequest()); err != nil {
		t.Fatalf("a failed bookkeeping write must not fail the payment, got %v", err)
	}
}
