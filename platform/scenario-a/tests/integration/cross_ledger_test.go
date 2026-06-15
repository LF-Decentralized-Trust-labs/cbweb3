// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	orchpb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Suite 2 — hub-and-spoke cross-ledger consistency / atomicity (D6 priority #2).
//
// The orchestrator carries the dual-layer HTLC atomicity logic. With the cacti
// relay faked (captureRelay), these tests drive the two-leg flow:
//
//   HAPPY: lock leg A (initiator, holds secret) -> relay confirms counterparty
//          lock (CounterpartyLocked) -> SettleHTLC via secret reveal succeeds;
//          both layers (on-chain HTLC + Zeto) settle exactly once.
//
//   FAILURE: counterparty never locks -> initiator's settle stays blocked ->
//          timelock expires -> RefundHTLC fully unwinds the lock. No partial
//          settlement: tokens are unlocked, never transferred; no value created.

// TestCrossLedger_HappyPath_BothLegsSettle verifies the atomic two-leg settle.
func TestCrossLedger_HappyPath_BothLegsSettle(t *testing.T) {
	orch := startOrchestrator(t, orchConfig{
		crossSpokeMode: true,
		withHTLC:       true,
		fxRepo:         newMemFXRepo(),
	})
	ctx := context.Background()

	// Leg A: the initiator locks and receives the secret.
	lockResp, err := orch.client.LockHTLC(ctx, &orchpb.LockHTLCRequest{
		AgreementId: "XL-HAPPY",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Before the counterparty locks, settle MUST be blocked (atomicity guard).
	_, err = orch.client.SettleHTLC(ctx, &orchpb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("settle before counterparty lock must be FailedPrecondition, got %v", err)
	}
	if _, _, _, _, transferLocked := orch.zeto.counters(); transferLocked != 0 {
		t.Fatalf("no tokens may move before counterparty lock, got transferLocked=%d", transferLocked)
	}

	// Relay observes the counterparty's matching leg (same hashLock, different
	// contractId) and flips CounterpartyLocked, unblocking settlement.
	if err := orch.relay.lockHandler()(ports.InteroperabilityProof{
		ContractID: "remote-leg-cid",
		HashLock:   lockResp.HashLock,
	}); err != nil {
		t.Fatalf("relay lock handler: %v", err)
	}

	st0, _ := orch.client.GetHTLCStatus(ctx, &orchpb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if !st0.Lock.CounterpartyLocked {
		t.Fatal("expected CounterpartyLocked true after relay confirmation")
	}

	// Now the secret reveal settles both layers.
	settleResp, err := orch.client.SettleHTLC(ctx, &orchpb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC after counterparty lock: %v", err)
	}
	if settleResp.HtlcTxHash == "" || settleResp.ZetoTxHash == "" {
		t.Error("expected both on-chain HTLC and Zeto tx hashes on settle")
	}

	st, err := orch.client.GetHTLCStatus(ctx, &orchpb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if st.Lock.State != orchpb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("expected SETTLED, got %s", st.Lock.State)
	}

	// Both layers settled exactly once; nothing was refunded.
	_, htlcSettle, htlcRefund := orch.htlc.counters()
	if htlcSettle != 1 {
		t.Errorf("expected on-chain HTLC settle once, got %d", htlcSettle)
	}
	if htlcRefund != 0 {
		t.Errorf("expected no on-chain refund on happy path, got %d", htlcRefund)
	}
	if _, _, _, unlock, transferLocked := orch.zeto.counters(); transferLocked != 1 || unlock != 0 {
		t.Errorf("expected exactly one Zeto locked-transfer and no unlock, got transferLocked=%d unlock=%d", transferLocked, unlock)
	}
}

// TestCrossLedger_RelaySettleEvent_SettlesResponderLeg drives the responder-side
// settlement: a relay LogHTLCClaimed event carrying the revealed secret settles
// the local leg, demonstrating the secret bridges across ledgers.
func TestCrossLedger_RelaySettleEvent_SettlesResponderLeg(t *testing.T) {
	orch := startOrchestrator(t, orchConfig{
		crossSpokeMode: true,
		fxRepo:         newMemFXRepo(),
	})
	ctx := context.Background()

	lockResp, err := orch.client.LockHTLC(ctx, &orchpb.LockHTLCRequest{
		AgreementId: "XL-RELAY-SETTLE",
		Receiver:    "bank-b",
		Amount:      "250",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Counterparty lock observed first (unblocks the settle guard).
	if err := orch.relay.lockHandler()(ports.InteroperabilityProof{
		ContractID: "remote-cid",
		HashLock:   lockResp.HashLock,
	}); err != nil {
		t.Fatalf("relay lock handler: %v", err)
	}

	// Relay delivers the revealed secret from the counterparty ledger.
	payload, _ := json.Marshal(map[string]string{"secret": lockResp.Secret})
	if err := orch.relay.settleHandler()(ports.InteroperabilityProof{
		ContractID:   "foreign-settle-cid",
		ProofPayload: payload,
	}); err != nil {
		t.Fatalf("relay settle handler: %v", err)
	}

	st, _ := orch.client.GetHTLCStatus(ctx, &orchpb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if st.Lock.State != orchpb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("expected SETTLED after relay settle event, got %s", st.Lock.State)
	}
}

// TestCrossLedger_FailurePath_Timeout_FullRefund_NoPartialSettlement verifies
// that when the counterparty never locks its leg, the initiator's settle stays
// blocked and a post-timeout refund fully unwinds the lock — no partial
// settlement, no value created.
func TestCrossLedger_FailurePath_Timeout_FullRefund_NoPartialSettlement(t *testing.T) {
	orch := startOrchestrator(t, orchConfig{
		crossSpokeMode: true,
		withHTLC:       true,
		fxRepo:         newMemFXRepo(),
	})
	ctx := context.Background()

	// Lock with an already-expired timelock to model the counterparty abandoning
	// the swap (relay never confirms a counterparty lock).
	lockResp, err := orch.client.LockHTLC(ctx, &orchpb.LockHTLCRequest{
		AgreementId: "XL-TIMEOUT",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) - 10, // already expired
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Settle is impossible: counterparty never locked, so the guard blocks it.
	_, err = orch.client.SettleHTLC(ctx, &orchpb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("settle without counterparty lock must be FailedPrecondition, got %v", err)
	}

	// After timeout, refund fully unwinds the lock.
	refundResp, err := orch.client.RefundHTLC(ctx, &orchpb.RefundHTLCRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("RefundHTLC after timeout: %v", err)
	}
	if refundResp.ZetoTxHash == "" {
		t.Error("expected a Zeto unlock tx hash on refund")
	}

	st, err := orch.client.GetHTLCStatus(ctx, &orchpb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if st.Lock.State != orchpb.HTLCState_HTLC_STATE_REFUNDED {
		t.Errorf("expected REFUNDED after timeout refund, got %s", st.Lock.State)
	}

	// Atomicity invariant: tokens were unlocked (refunded) and NEVER transferred.
	// No partial settlement — the locked value returns whole to the sender.
	_, _, lock, unlock, transferLocked := orch.zeto.counters()
	if lock != 1 {
		t.Errorf("expected exactly one Zeto lock, got %d", lock)
	}
	if transferLocked != 0 {
		t.Errorf("refund path must NOT transfer locked tokens (no partial settlement), got %d", transferLocked)
	}
	if unlock != 1 {
		t.Errorf("expected exactly one Zeto unlock (full refund), got %d", unlock)
	}

	// On-chain layer refunded, never settled.
	_, htlcSettle, htlcRefund := orch.htlc.counters()
	if htlcSettle != 0 {
		t.Errorf("on-chain HTLC must not settle on the failure path, got %d", htlcSettle)
	}
	if htlcRefund != 1 {
		t.Errorf("expected on-chain HTLC refund once, got %d", htlcRefund)
	}
}

// TestCrossLedger_RefundBeforeTimeout_Rejected guards the refund precondition:
// a refund attempted before the timelock expires must be rejected, preventing a
// griefing unwind while the counterparty leg could still complete.
func TestCrossLedger_RefundBeforeTimeout_Rejected(t *testing.T) {
	orch := startOrchestrator(t, orchConfig{
		crossSpokeMode: true,
		withHTLC:       true,
		fxRepo:         newMemFXRepo(),
	})
	ctx := context.Background()

	lockResp, err := orch.client.LockHTLC(ctx, &orchpb.LockHTLCRequest{
		AgreementId: "XL-EARLY-REFUND",
		Receiver:    "bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600, // not expired
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	_, err = orch.client.RefundHTLC(ctx, &orchpb.RefundHTLCRequest{ContractId: lockResp.ContractId})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("refund before timeout must be FailedPrecondition, got %v", err)
	}

	// No unwind happened: state still LOCKED, no unlock side-effect.
	st, _ := orch.client.GetHTLCStatus(ctx, &orchpb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if st.Lock.State != orchpb.HTLCState_HTLC_STATE_LOCKED {
		t.Errorf("expected LOCKED after rejected early refund, got %s", st.Lock.State)
	}
	if _, _, _, unlock, _ := orch.zeto.counters(); unlock != 0 {
		t.Errorf("rejected refund must not unlock tokens, got %d", unlock)
	}
}
