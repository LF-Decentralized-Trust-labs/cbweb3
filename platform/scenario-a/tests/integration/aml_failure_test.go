// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

package integration

import (
	"context"
	"testing"
	"time"

	compliancev1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	orchpb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Suite 1 — AML/CFT failure handling (D6 priority #1).
//
// These tests drive a payment/settlement through the orchestrator gated by a
// compliance decision. They assert that when compliance denies the transfer
// (limit exceeded), the settlement is BLOCKED and NO mint / lock / settle
// side-effect occurs — and that compliance is consulted at payment INITIATION,
// not only at onboarding. The gate is enforced by the caller (acting as the API
// gateway): consult compliance, branch, only then touch the orchestrator.

const (
	payerBankA = "bank-a" // resolves to central-bank-a
	amlCurrency = "USD"
)

// initiatePayment models what the API gateway does at payment initiation: it
// re-checks compliance for THIS transfer before asking the orchestrator to move
// any value. Returns the compliance response (caller decides whether to proceed).
func initiatePayment(t *testing.T, comp compliancev1.ComplianceServiceClient, payer, currency, amountHuman string) *compliancev1.CheckAndDeductTransferLimitResponse {
	t.Helper()
	resp, err := comp.CheckAndDeductTransferLimit(context.Background(), &compliancev1.CheckAndDeductTransferLimitRequest{
		PayerBankId: payer,
		Currency:    currency,
		AmountHuman: amountHuman,
	})
	if err != nil {
		t.Fatalf("compliance CheckAndDeductTransferLimit: %v", err)
	}
	return resp
}

// TestAML_DeniedTransfer_BlocksSettlement_NoSideEffect verifies that a transfer
// exceeding the configured limit is denied by compliance and that the
// orchestrator performs NO settlement side-effect (no lock, no mint, no transfer).
func TestAML_DeniedTransfer_BlocksSettlement_NoSideEffect(t *testing.T) {
	comp, fc := startCompliance(t)
	fc.SetLimit("central-bank-a", "1000") // daily cap of 1000

	orch := startOrchestrator(t, orchConfig{withHTLC: true})

	// At payment INITIATION the gateway re-checks compliance for an over-limit amount.
	decision := initiatePayment(t, comp, payerBankA, amlCurrency, "5000")
	if decision.Allowed {
		t.Fatal("expected compliance to DENY an over-limit transfer")
	}
	if decision.ErrorCode != "TRANSFER_LIMIT_EXCEEDED" {
		t.Errorf("expected TRANSFER_LIMIT_EXCEEDED, got %q", decision.ErrorCode)
	}

	// Because compliance denied, the gateway must NOT drive any orchestrator
	// settlement. We assert the invariant directly: no chain side-effects happened.
	mint, transfer, lock, unlock, transferLocked := orch.zeto.counters()
	if mint+transfer+lock+unlock+transferLocked != 0 {
		t.Errorf("denied transfer must produce no Zeto side-effects, got mint=%d transfer=%d lock=%d unlock=%d transferLocked=%d",
			mint, transfer, lock, unlock, transferLocked)
	}
	hl, hs, hr := orch.htlc.counters()
	if hl+hs+hr != 0 {
		t.Errorf("denied transfer must produce no on-chain HTLC side-effects, got lock=%d settle=%d refund=%d", hl, hs, hr)
	}
}

// TestAML_Denied_OrchestratorSettleNotReached confirms that when an HTLC was
// (hypothetically) locked but compliance denies at settle-time re-check, the
// caller blocks the settle and no token transfer occurs. This exercises the
// FailedPrecondition semantics the gateway maps a denial onto.
func TestAML_Denied_OrchestratorSettleNotReached(t *testing.T) {
	comp, fc := startCompliance(t)
	fc.SetLimit("central-bank-a", "1000")

	orch := startOrchestrator(t, orchConfig{withHTLC: true})
	ctx := context.Background()

	// Lock is allowed (within-limit consult succeeds) — a small initial leg.
	if d := initiatePayment(t, comp, payerBankA, amlCurrency, "100"); !d.Allowed {
		t.Fatalf("within-limit lock should be allowed, got %q", d.ErrorCode)
	}
	lockResp, err := orch.client.LockHTLC(ctx, &orchpb.LockHTLCRequest{
		AgreementId: "AML-LOCK",
		Receiver:    "bank-b",
		Amount:      "100",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// At settle-time the gateway re-checks compliance for a now-over-limit amount.
	settleDecision := initiatePayment(t, comp, payerBankA, amlCurrency, "5000")
	if settleDecision.Allowed {
		t.Fatal("expected settle-time compliance re-check to DENY")
	}

	// Gateway maps the denial to FailedPrecondition and does NOT call SettleHTLC.
	// Assert no token transfer occurred on the locked leg.
	_, _, _, _, transferLocked := orch.zeto.counters()
	if transferLocked != 0 {
		t.Errorf("blocked settle must not transfer locked tokens, got transferLocked=%d", transferLocked)
	}

	// The HTLC must remain LOCKED (un-settled) — proving no partial settlement.
	st, err := orch.client.GetHTLCStatus(ctx, &orchpb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if st.Lock.State != orchpb.HTLCState_HTLC_STATE_LOCKED {
		t.Errorf("expected HTLC to remain LOCKED after blocked settle, got %s", st.Lock.State)
	}

	// Sanity: the denial maps to FailedPrecondition at the gateway boundary.
	if code := status.Code(status.Error(codes.FailedPrecondition, settleDecision.ErrorCode)); code != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition mapping, got %s", code)
	}
}

// TestAML_RecheckedAtInitiation_NotJustOnboarding verifies compliance is
// consulted PER transfer at initiation: a first within-limit transfer is allowed
// and deducted; a second transfer that pushes cumulative volume past the limit is
// denied. Onboarding alone would not catch this — it is the per-initiation
// re-check that does.
func TestAML_RecheckedAtInitiation_NotJustOnboarding(t *testing.T) {
	comp, fc := startCompliance(t)
	fc.SetLimit("central-bank-a", "1000")

	// First transfer: 600 <= 1000 => allowed, accumulates to 600.
	first := initiatePayment(t, comp, payerBankA, amlCurrency, "600")
	if !first.Allowed {
		t.Fatalf("first within-limit transfer should be allowed, got %q", first.ErrorCode)
	}

	// Second transfer: 600 + 600 = 1200 > 1000 => denied at THIS initiation.
	second := initiatePayment(t, comp, payerBankA, amlCurrency, "600")
	if second.Allowed {
		t.Fatal("second transfer crossing the cumulative limit must be denied at initiation")
	}
	if second.ErrorCode != "TRANSFER_LIMIT_EXCEEDED" {
		t.Errorf("expected TRANSFER_LIMIT_EXCEEDED, got %q", second.ErrorCode)
	}
}

// TestAML_AllowedTransfer_Settles confirms the positive control: a within-limit
// transfer is allowed and a full settlement completes with the expected
// side-effects (proving the deny-path block above is meaningful, not vacuous).
func TestAML_AllowedTransfer_Settles(t *testing.T) {
	comp, fc := startCompliance(t)
	fc.SetLimit("central-bank-a", "1000")

	orch := startOrchestrator(t, orchConfig{withHTLC: true})
	ctx := context.Background()

	if d := initiatePayment(t, comp, payerBankA, amlCurrency, "500"); !d.Allowed {
		t.Fatalf("within-limit transfer should be allowed, got %q", d.ErrorCode)
	}

	lockResp, err := orch.client.LockHTLC(ctx, &orchpb.LockHTLCRequest{
		AgreementId: "AML-OK",
		Receiver:    "bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}
	if _, err := orch.client.SettleHTLC(ctx, &orchpb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	}); err != nil {
		t.Fatalf("SettleHTLC: %v", err)
	}

	st, err := orch.client.GetHTLCStatus(ctx, &orchpb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if st.Lock.State != orchpb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("allowed transfer should settle, got state %s", st.Lock.State)
	}
	if _, _, _, _, transferLocked := orch.zeto.counters(); transferLocked != 1 {
		t.Errorf("expected exactly one locked-token transfer on settle, got %d", transferLocked)
	}
}
