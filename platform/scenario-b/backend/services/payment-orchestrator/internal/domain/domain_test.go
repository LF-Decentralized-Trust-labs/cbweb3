// SPDX-License-Identifier: Apache-2.0

package domain

import "testing"

func TestFXState_IsTerminal(t *testing.T) {
	terminal := []FXState{FXStateRejected, FXStateCancelled, FXStateSettled}
	nonTerminal := []FXState{FXStateInvalid, FXStateProposed, FXStateAccepted}

	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestSwapExecError_Error(t *testing.T) {
	tests := []SwapErrorCode{
		ErrCodeSlippageLimitExceeded,
		ErrCodeInsufficientPoolLiquidity,
		ErrCodeZKValidationFailed,
		ErrCodeCircuitBreakerHalted,
	}
	for _, code := range tests {
		err := &SwapExecError{Code: code}
		if err.Error() != string(code) {
			t.Errorf("expected %q, got %q", code, err.Error())
		}
	}
}

// Guards the canonical string values that cross the on-chain / API boundary;
// changing any of these silently is a breaking change.
func TestCanonicalStateConstants(t *testing.T) {
	cases := []struct{ got, want string }{
		{string(SwapStatePending), "PENDING"},
		{string(SwapStateSubmitted), "SUBMITTED"},
		{string(SwapStateConfirming), "CONFIRMING"},
		{string(SwapStateCompleted), "COMPLETED"},
		{string(SwapStateFailed), "FAILED"},
		{string(BridgeStateLocking), "LOCKING"},
		{string(BridgeStateActive), "ACTIVE"},
		{string(BridgeStateBurning), "BURNING"},
		{string(BridgeStateBurned), "BURNED"},
		{string(BridgeStateReleased), "RELEASED"},
		{string(RelayerStatePending), "PENDING"},
		{string(RelayerStateInFlight), "IN_FLIGHT"},
		{string(RelayerStateCompleted), "COMPLETED"},
		{string(RelayerStateFailed), "FAILED"},
		{string(RelayerStateEscalated), "ESCALATED"},
		{RelayerEventTypeLockMint, "LOCK_MINT"},
		{RelayerEventTypeBurnUnlock, "BURN_UNLOCK"},
		{string(DepositStatusPending), "PENDING"},
		{string(EscrowStatusApproved), "APPROVED"},
		{string(RedeemStatusRejected), "REJECTED"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("constant mismatch: got %q want %q", c.got, c.want)
		}
	}
	// Alias must equal the canonical PENDING constant.
	if RelayerItemStatePending != RelayerStatePending {
		t.Errorf("RelayerItemStatePending alias diverged from RelayerStatePending")
	}
}

func TestBridgeStateReconciliationRequired(t *testing.T) {
	if BridgeStateReconciliationRequired != "RECONCILIATION_REQUIRED" {
		t.Errorf("unexpected value %q", BridgeStateReconciliationRequired)
	}
}
