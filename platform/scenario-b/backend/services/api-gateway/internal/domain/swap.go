// SPDX-License-Identifier: Apache-2.0

// Package domain defines Scenario B swap types for the api-gateway.
package domain

// SwapState represents the canonical 5-state machine for a Scenario B swap order (FR-057).
type SwapState string

const (
	SwapStatePending    SwapState = "PENDING"
	SwapStateSubmitted  SwapState = "SUBMITTED"
	SwapStateConfirming SwapState = "CONFIRMING"
	SwapStateCompleted  SwapState = "COMPLETED"
	SwapStateFailed     SwapState = "FAILED"
)

// SwapErrorCode enumerates the canonical error codes for failed swap orders (FR-059).
type SwapErrorCode string

const (
	ErrCodeSlippageLimitExceeded     SwapErrorCode = "SLIPPAGE_LIMIT_EXCEEDED"
	ErrCodeInsufficientPoolLiquidity SwapErrorCode = "INSUFFICIENT_POOL_LIQUIDITY"
	ErrCodeZKValidationFailed        SwapErrorCode = "ZK_VALIDATION_FAILED"
	ErrCodeCircuitBreakerHalted      SwapErrorCode = "CIRCUIT_BREAKER_HALTED"
	// ErrCodePoolNotActive is returned when a swap is attempted on a pool that is
	// EMPTY or PENDING_COUNTERPART (FR-011 / 005-cooperative-liquidity).
	ErrCodePoolNotActive SwapErrorCode = "POOL_NOT_ACTIVE"
)

// SwapExecError wraps a canonical swap execution failure.
type SwapExecError struct {
	Code SwapErrorCode
}

func (e *SwapExecError) Error() string { return string(e.Code) }
