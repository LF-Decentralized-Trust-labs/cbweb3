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
	SwapStatusQuoting            SwapOperationStatus = "QUOTING"
	SwapStatusBridgeInProgress   SwapOperationStatus = "BRIDGE_IN_PROGRESS"
	SwapStatusSwapInProgress     SwapOperationStatus = "SWAP_IN_PROGRESS"
	SwapStatusBridgeOutProgress  SwapOperationStatus = "BRIDGE_OUT_PROGRESS"
	SwapStatusCompleted          SwapOperationStatus = "COMPLETED"
	SwapStatusFailed             SwapOperationStatus = "FAILED"
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
	CreatedAt           time.Time           `gorm:"column:created_at;autoCreateTime;index"`
	CompletedAt         *time.Time          `gorm:"column:completed_at"`
}

// TableName overrides GORM's default table name.
func (CrossCurrencySwapOperation) TableName() string {
	return "cross_currency_swap_operations"
}
