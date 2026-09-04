// SPDX-License-Identifier: Apache-2.0

// Package domain defines CrossCurrencySwapOperation model for tracking end-to-end
// cross-currency swap flow (bridge-in → swap Hub → bridge-out).
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package domain

import "time"

// SwapOperationStatus is the lifecycle state of a cross-currency swap operation.
type SwapOperationStatus string

const (
	SwapStatusQuoting           SwapOperationStatus = "QUOTING"
	SwapStatusBridgeInProgress  SwapOperationStatus = "BRIDGE_IN_PROGRESS"
	SwapStatusSwapInProgress    SwapOperationStatus = "SWAP_IN_PROGRESS"
	SwapStatusBridgeOutProgress SwapOperationStatus = "BRIDGE_OUT_PROGRESS"
	SwapStatusCompleted         SwapOperationStatus = "COMPLETED"
	SwapStatusFailed            SwapOperationStatus = "FAILED"
)

// ResidueReturnStatus records what happened to the unspent slippage buffer
// (MaxAmountIn − realized amount_in) after the swap settled. It is deliberately
// independent of the swap status: the payment is already final when the residue is
// handled, so a failed return must never reopen a COMPLETED swap.
type ResidueReturnStatus string

const (
	// ResidueNone means there was nothing to return — the swap consumed the full cap.
	ResidueNone ResidueReturnStatus = "NONE"
	// ResidueReturnEnqueued means the return leg was accepted and is being driven by the
	// relayer; its terminal state lives on the bridge position, not here.
	ResidueReturnEnqueued ResidueReturnStatus = "RETURN_ENQUEUED"
	// ResidueReturnFailed means the return could not be enqueued. The value is not lost — it
	// sits on the issuing CB's Hub address — but the payer stays over-debited until it is
	// returned. Retryable: nothing about the request is caller-supplied, the amount is derived
	// from the bridge-in position plus the on-chain LogSwap, and the endpoint is idempotent on
	// (swap_tx_hash, RESIDUE). This is the state the retry worker picks up.
	ResidueReturnFailed ResidueReturnStatus = "RETURN_FAILED"
	// ResidueReturnEscalated means the retries were exhausted. Terminal for automation: the
	// value is still on the CB's Hub address and now needs a human. Mirrors the relayer's
	// RELAYER_EXHAUSTED escalation rather than retrying forever.
	ResidueReturnEscalated ResidueReturnStatus = "RETURN_ESCALATED"
)

// BridgeOutDeliveryStatus records what happened to the DELIVERY leg — the notification
// that makes CB-B burn the wrapped output on the Hub and release the native token to the
// beneficiary on its spoke.
//
// It is separate from SwapOperationStatus for the same reason ResidueReturnStatus is: the
// swap itself has a verdict of its own, and the delivery's fate must not be conflated with
// it. Where they differ is which way the asymmetry runs. A failed residue leaves the payer
// over-debited on a settled payment; a failed delivery leaves value that has already left
// two balance sheets sitting on the Hub with no beneficiary — the partial settlement the
// constitution forbids.
type BridgeOutDeliveryStatus string

const (
	// BridgeOutDeliveryNotified means the relay accepted the notification. Its terminal state
	// then lives on CB-B's bridge position, not here — this gateway is not the authority on
	// what CB-B did next.
	BridgeOutDeliveryNotified BridgeOutDeliveryStatus = "DELIVERY_NOTIFIED"
	// BridgeOutDeliveryFailed means the notification could not be delivered or was rejected.
	// The value is not lost: the wrapped output sits on the Hub address that received the
	// swap, which the burn would have drawn from. This is the state the retry worker picks up.
	//
	// Retryable for the same two reasons the residue return is, and both were built
	// deliberately for a different purpose (R2-CR-6): nothing that authorizes the burn comes
	// from the request — CB-B re-derives the amount, the burn-from address and the recipient
	// from the on-chain swap receipt — and the endpoint is idempotent on swap_tx_hash. A
	// rejected notification never consumed the hash, so the first successful retry creates the
	// position and any later repeat is answered with the existing one.
	BridgeOutDeliveryFailed BridgeOutDeliveryStatus = "DELIVERY_FAILED"
	// BridgeOutDeliveryEscalated means the retries were exhausted, or a governance pause
	// outlived the deferral bound. Terminal for automation: the beneficiary still has nothing,
	// the payer is still debited, and a human has to decide between a late delivery and a
	// refund. Mirrors ResidueReturnEscalated rather than retrying forever.
	BridgeOutDeliveryEscalated BridgeOutDeliveryStatus = "DELIVERY_ESCALATED"
)

// CrossCurrencySwapOperation tracks end-to-end cross-currency swap, linking
// 3 sub-operations (bridge-in, swap Hub, bridge-out) via correlation_id.
type CrossCurrencySwapOperation struct {
	SwapID              string              `gorm:"primaryKey;column:swap_id;type:varchar(64)"`
	CorrelationID       string              `gorm:"column:correlation_id;not null;uniqueIndex"`
	PayerBankID         string              `gorm:"column:payer_bank_id;not null;index"`
	BeneficiaryBankID   string              `gorm:"column:beneficiary_bank_id;not null"`
	SourceCurrency      string              `gorm:"column:source_currency;not null"`
	TargetCurrency      string              `gorm:"column:target_currency;not null"`
	PoolPair            string              `gorm:"column:pool_pair;not null"`
	AmountIn            string              `gorm:"column:amount_in;not null"`
	AmountOut           string              `gorm:"column:amount_out;not null"`
	MaxAmountIn         string              `gorm:"column:max_amount_in;not null"`
	EffectiveRate       float64             `gorm:"column:effective_rate"`
	Status              SwapOperationStatus `gorm:"column:status;not null;default:'QUOTING'"`
	BridgeInPositionID  *string             `gorm:"column:bridge_in_position_id"`
	SwapTxHash          *string             `gorm:"column:swap_tx_hash"`
	BridgeOutPositionID *string             `gorm:"column:bridge_out_position_id"`
	QuoteID             *string             `gorm:"column:quote_id"`
	FailureReason       *string             `gorm:"column:failure_reason"`
	// ResidueAmount is MaxAmountIn − AmountIn: the slippage buffer the bridge-in had to
	// move before the real cost was known, and which the swap did not consume.
	ResidueAmount string `gorm:"column:residue_amount;default:''"`
	// ResiduePositionID is the bridge position returning ResidueAmount to the payer.
	ResiduePositionID *string `gorm:"column:residue_position_id"`
	// ResidueStatus tracks whether that return was enqueued. Empty on legacy rows. Leads the
	// composite retry index (see ResidueNextAttemptAt) because it is the selective term.
	ResidueStatus ResidueReturnStatus `gorm:"column:residue_status;default:'';index:idx_swap_residue_retry,priority:1"`
	// ResidueAttempts counts how many times the return has been attempted, and
	// ResidueNextAttemptAt is when the next attempt becomes due (exponential backoff).
	//
	// A failed enqueue creates no bridge position, so the relayer queue — which retries the
	// legs that DID get enqueued — has nothing to pick up. Without these two fields a bank's
	// unspent reserve sits on the CB's Hub address indefinitely, and the bank cannot even see
	// it.
	//
	// The index is COMPOSITE, status first: the retry query filters on residue_status and only
	// then on the schedule. Indexing the schedule alone would be useless — it is NULL for
	// essentially every row (NONE, RETURN_ENQUEUED, legacy), so the planner would fall back to
	// scanning the table on every sweep, at a cost that grows with payment volume.
	ResidueAttempts      int        `gorm:"column:residue_attempts;not null;default:0"`
	ResidueNextAttemptAt *time.Time `gorm:"column:residue_next_attempt_at;index:idx_swap_residue_retry,priority:2"`
	// ResidueDeferredSince is when the CURRENT deferral window opened: the first sweep that
	// found this row's pair halted by governance. It stays put across later deferrals and is
	// cleared by any real attempt, so it measures the whole pause rather than the gap since the
	// last sweep looked.
	//
	// Deferring deliberately does not consume an attempt, so ResidueAttempts cannot bound a
	// pause. Without this column an indefinitely paused pair defers the refund forever: the
	// payer stays over-debited, the value sits on the issuing CB's Hub address, and a log line
	// is the only trace. NULL means the row is not currently deferred.
	ResidueDeferredSince *time.Time `gorm:"column:residue_deferred_since"`
	// BridgeOutStatus tracks whether the delivery notification reached CB-B. Empty on legacy
	// rows. Leads its composite retry index for the same reason ResidueStatus does: it is the
	// selective term, and the schedule alone is NULL for nearly every row.
	BridgeOutStatus BridgeOutDeliveryStatus `gorm:"column:bridge_out_status;default:'';index:idx_swap_bridge_out_retry,priority:1"`
	// BridgeOutAttempts counts delivery attempts and BridgeOutNextAttemptAt is when the next
	// becomes due (exponential backoff).
	//
	// A rejected notification creates no bridge position on CB-B, so the relayer queue — which
	// drives the legs that WERE enqueued — has nothing to pick up, and the relay itself
	// forwards exactly once. Without these two fields the swapped value sits on the Hub with
	// no beneficiary indefinitely, and the only trace is a log line telling an operator to
	// contact support.
	//
	// The retry needs no payload columns: correlation_id, swap_tx_hash, pool_pair, amount_out
	// and beneficiary_bank_id are already here, the AMM address is re-resolved from the pair,
	// and the burn-from address is re-derived by CB-B from the receipt rather than sent.
	BridgeOutAttempts      int        `gorm:"column:bridge_out_attempts;not null;default:0"`
	BridgeOutNextAttemptAt *time.Time `gorm:"column:bridge_out_next_attempt_at;index:idx_swap_bridge_out_retry,priority:2"`
	// BridgeOutDeferredSince is when the current deferral window opened, for a pair halted by
	// governance. Same contract as ResidueDeferredSince: the first stamp wins, any real attempt
	// clears it, and it bounds a pause that the attempt counter cannot, because deferring does
	// not consume an attempt.
	BridgeOutDeferredSince *time.Time `gorm:"column:bridge_out_deferred_since"`
	CreatedAt              time.Time  `gorm:"column:created_at;autoCreateTime;index"`
	CompletedAt            *time.Time `gorm:"column:completed_at"`
}

// TableName overrides GORM's default table name.
func (CrossCurrencySwapOperation) TableName() string {
	return "cross_currency_swap_operations"
}
