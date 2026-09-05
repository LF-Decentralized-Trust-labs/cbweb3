// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// MarkDeliveredAfterRetry is the only write that moves a swap OFF its terminal verdict, so both
// halves of its predicate matter and neither is visible from the service layer, which sees a
// fake: the promotion has to happen, and it has to happen only to a row the synchronous call
// gave up on.

func deliveredOp(swapID string, status domain.SwapOperationStatus) *domain.CrossCurrencySwapOperation {
	reason := "cacti bridge-out relay failed (partial success)"
	return &domain.CrossCurrencySwapOperation{
		SwapID:            swapID,
		CorrelationID:     "corr-" + swapID,
		PayerBankID:       "bank-itau",
		BeneficiaryBankID: "bank-macro",
		SourceCurrency:    "BRL",
		TargetCurrency:    "ARS",
		PoolPair:          "W-BRL-W-ARS",
		AmountIn:          "1000",
		AmountOut:         "900",
		MaxAmountIn:       "3000",
		Status:            status,
		FailureReason:     &reason,
		BridgeOutStatus:   domain.BridgeOutDeliveryFailed,
		BridgeOutAttempts: 1,
	}
}

func TestMarkDeliveredAfterRetry_PromotesTheVerdictAndTheReasonTogether(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	ctx := context.Background()

	if err := db.Create(deliveredOp("s1", domain.SwapStatusFailed)).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	const reason = "delivery recovered on attempt 2 of 5 — the beneficiary has been paid"
	if err := r.MarkDeliveredAfterRetry(ctx, "s1", reason); err != nil {
		t.Fatalf("MarkDeliveredAfterRetry: %v", err)
	}

	var got domain.CrossCurrencySwapOperation
	if err := db.First(&got, "swap_id = ?", "s1").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != domain.SwapStatusDeliveredAfterRetry {
		t.Errorf("status = %q, want %q — the column an operator reads first still says the "+
			"beneficiary was never paid", got.Status, domain.SwapStatusDeliveredAfterRetry)
	}
	if got.FailureReason == nil || *got.FailureReason != reason {
		t.Errorf("failure_reason = %v, want the recovery text", got.FailureReason)
	}
}

// TestMarkDeliveredAfterRetry_LeavesANonFailedRowAlone pins the guard. A COMPLETED row reaching
// this write would mean the delivery succeeded twice; rewriting its verdict to
// DELIVERED_AFTER_RETRY would downgrade a clean settlement into one that reads as a recovery,
// and would overwrite a reason that is not there to overwrite.
func TestMarkDeliveredAfterRetry_LeavesANonFailedRowAlone(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	ctx := context.Background()

	op := deliveredOp("s2", domain.SwapStatusCompleted)
	op.FailureReason = nil
	if err := db.Create(op).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := r.MarkDeliveredAfterRetry(ctx, "s2", "recovered"); err != nil {
		t.Fatalf("MarkDeliveredAfterRetry must not error on a row it declines to touch: %v", err)
	}

	var got domain.CrossCurrencySwapOperation
	if err := db.First(&got, "swap_id = ?", "s2").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != domain.SwapStatusCompleted {
		t.Errorf("status = %q, want it left at %q", got.Status, domain.SwapStatusCompleted)
	}
	if got.FailureReason != nil && *got.FailureReason != "" {
		t.Errorf("failure_reason = %q, want it left empty on a settled swap", *got.FailureReason)
	}
}
