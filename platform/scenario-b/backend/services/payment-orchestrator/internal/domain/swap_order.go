// SPDX-License-Identifier: Apache-2.0

// Package domain defines Scenario B core business types for the payment-orchestrator.
package domain

import "time"

// SwapState represents the canonical 5-state machine for a Scenario B swap order (FR-057).
type SwapState string

const (
	SwapStatePending    SwapState = "PENDING"    // order created, awaiting on-chain submission
	SwapStateSubmitted  SwapState = "SUBMITTED"  // transaction sent to the Hub
	SwapStateConfirming SwapState = "CONFIRMING" // awaiting block confirmation
	SwapStateCompleted  SwapState = "COMPLETED"  // block confirmed
	SwapStateFailed     SwapState = "FAILED"     // error at any stage
)

// SwapErrorCode enumerates the canonical error codes for failed swap orders (FR-059).
type SwapErrorCode string

const (
	ErrCodeSlippageLimitExceeded     SwapErrorCode = "SLIPPAGE_LIMIT_EXCEEDED"
	ErrCodeInsufficientPoolLiquidity SwapErrorCode = "INSUFFICIENT_POOL_LIQUIDITY"
	ErrCodeZKValidationFailed        SwapErrorCode = "ZK_VALIDATION_FAILED"
	ErrCodeCircuitBreakerHalted      SwapErrorCode = "CIRCUIT_BREAKER_HALTED"
)

// SwapExecError wraps a canonical swap execution failure code.
type SwapExecError struct {
	Code SwapErrorCode
}

func (e *SwapExecError) Error() string { return string(e.Code) }

// SwapOrderScenarioB represents an Exact-Output swap order submitted by a payer bank (FR-057).
type SwapOrderScenarioB struct {
	OrderID              string        `gorm:"primaryKey;column:order_id;type:varchar(64)"`
	PayerBankID          string        `gorm:"column:payer_bank_id;not null"`
	BeneficiaryBankID    string        `gorm:"column:beneficiary_bank_id;not null"`
	PoolPair             string        `gorm:"column:pool_pair;not null"`
	ExactOutputAmount    string        `gorm:"column:exact_output_amount;not null"`
	MaxAmountIn          string        `gorm:"column:max_amount_in;not null"`
	QuoteRef             string        `gorm:"column:quote_ref"`
	ZKPointerPayer       string        `gorm:"column:zk_pointer_payer"`
	ZKPointerBeneficiary string        `gorm:"column:zk_pointer_beneficiary"`
	State                SwapState     `gorm:"column:state;not null;default:'PENDING'"`
	ErrorCode            SwapErrorCode `gorm:"column:error_code"`
	SubmittedAt          *time.Time    `gorm:"column:submitted_at"`
	ConfirmedAt          *time.Time    `gorm:"column:confirmed_at"`
	CreatedAt            time.Time     `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time     `gorm:"column:updated_at;autoUpdateTime"`
}
