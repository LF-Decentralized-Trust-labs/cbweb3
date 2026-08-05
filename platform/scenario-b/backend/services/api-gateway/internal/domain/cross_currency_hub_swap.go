// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// CrossCurrencyHubSwap records a Hub AMM swap the issuing CB executed on behalf of a
// commercial bank (sovereign delegation of Step 2).
//
// Why it exists: the AMM gates both sides of a swap on the Hub IdentityRegistry
// (onlyVerified(msg.sender) / onlyVerified(to)), and only central banks hold a Hub
// identity. A commercial bank therefore cannot be msg.sender. Rather than handing the
// bank the CB's key so it can act *as* the CB, the bank delegates the swap over the same
// authenticated channel it already uses for bridge-in and residue-return, and the CB
// executes it with its own signer, inside its own process.
//
// The bridge-in position is the idempotency anchor: it funds exactly one swap, so
// bridge_in_position_id is the primary key. A retried delegation returns the recorded
// transaction hash instead of running a second swap against the same W-<source> — the
// swap is not idempotent on-chain, and a blind retry would spend the payer's bridged
// balance twice.
type CrossCurrencyHubSwap struct {
	BridgeInPositionID string `gorm:"primaryKey;column:bridge_in_position_id;type:varchar(64)"`
	CorrelationID      string `gorm:"column:correlation_id;not null;index"`
	PayerBankID        string `gorm:"column:payer_bank_id;not null;index"`
	PoolPair           string `gorm:"column:pool_pair;not null"`
	// AmountOut is the exact output requested; AmountIn is the realized cost decoded from
	// the on-chain LogSwap (never the MaxAmountIn cap). Both are empty until the trade returns,
	// because the row is inserted BEFORE it runs.
	AmountOut  string `gorm:"column:amount_out;not null"`
	AmountIn   string `gorm:"column:amount_in;not null"`
	SwapTxHash string `gorm:"column:swap_tx_hash;not null"`
	// Status is what makes the primary key a guard rather than a record of the past.
	//
	// Recording the outcome AFTER the trade deduplicated the WRITE, not the TRADE: two concurrent
	// deliveries of one delegation both found no row, both swapped on-chain, and the second one's
	// insert then failed and was reported as a harmless "duplicate" — masking a second trade that
	// really happened. The row is now claimed PENDING before the swap, so the second delivery is
	// refused by the constraint before reaching the AMM.
	//
	// Legacy rows predate the column and hold ""; a row with a swap_tx_hash is EXECUTED whatever
	// the column says, which is what keeps an in-place upgrade from re-opening those positions.
	Status string `gorm:"column:status;not null;default:''"`
	// FailureReason is why a claim was abandoned, so an operator reconciling the position reads it
	// from the record instead of correlating logs.
	FailureReason string    `gorm:"column:failure_reason;not null;default:''"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName sets the PostgreSQL table name for GORM AutoMigrate.
func (CrossCurrencyHubSwap) TableName() string { return "cross_currency_hub_swaps" }
