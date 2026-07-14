// SPDX-License-Identifier: Apache-2.0

// Package services provides the AMM swap execution service for Scenario B (FR-027 / FR-057).
package services

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ComplianceGate is the interface for ZK-Pointer validation (FR-025 / FR-058).
type ComplianceGate interface {
	ValidateZKPointer(ctx context.Context, bankID, txRef, commitmentHash string) error
}

// CircuitBreakerGate is the interface for checking whether the AMM is halted (FR-030).
type CircuitBreakerGate interface {
	IsHalted(ctx context.Context, pair string) (bool, error)
}

// AMMSwapper executes an exact-output swap on the Hub AMM contract.
type AMMSwapper interface {
	SwapExactOutput(ctx context.Context, pair, amountOut, maxAmountIn, payerID, beneficiaryID, zkPayer, zkBeneficiary string) (orderID, txHash, amountIn string, err error)
	QuoteExactOutput(ctx context.Context, pair, amountOut string) (requiredInput, priceImpact string, quoteTimestamp int64, err error)
}

// AMMFeeReader reads the current swap fee rate from the AMM contract (T058 / FR-006).
type AMMFeeReader interface {
	GetFeeBps(ctx context.Context, pair string) (uint64, error)
}

// SwapFeeRecorder distributes swap fees to active LP positions synchronously (T058 / FR-006 / D14).
type SwapFeeRecorder interface {
	RecordSwapFee(ctx context.Context, poolPair, swapOrderID string, feeAmountA, feeAmountB *big.Int) error
}

// SwapGates groups the compliance, pool-status, and circuit-breaker gates injected into SwapService.
type SwapGates struct {
	ComplianceGate     ComplianceGate
	CircuitBreakerGate CircuitBreakerGate
	// PoolStatusGate blocks swaps when the pool is EMPTY or PENDING_COUNTERPART (FR-011).
	PoolStatusGate PoolStatusGate
}

// SwapRequest carries the parameters of an exact-output swap (FR-027).
type SwapRequest struct {
	Pair                 string
	AmountOut            string
	MaxAmountIn          string
	PayerID              string
	BeneficiaryID        string
	ZKPointerPayer       string
	ZKPointerBeneficiary string
	CorrelationID        string // Optional: for tracing cross-currency swap flows (009-commercial-cross-currency-swap)
}

// SwapResult carries the outcome of a successful swap.
type SwapResult struct {
	OrderID     string    `json:"order_id"`
	TxHash      string    `json:"tx_hash"`
	AmountIn    string    `json:"amount_in"`
	State       string    `json:"state"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// ZKValidationError wraps a ZK-Pointer validation failure (FR-058).
type ZKValidationError struct {
	Msg string
}

func (e *ZKValidationError) Error() string { return e.Msg }

// SwapService executes exact-output swaps with slippage protection and ZK compliance gates.
// Performance gate: p95 <= 6s (SC-022).
type SwapService struct {
	swapper     AMMSwapper
	gates       SwapGates
	feeReader   AMMFeeReader    // optional: reads feeBps for fee distribution (T058)
	feeRecorder SwapFeeRecorder // optional: distributes fees to LPs after swap (T058)
}

// NewSwapService creates a SwapService.
func NewSwapService(swapper AMMSwapper, gates SwapGates) *SwapService {
	return &SwapService{swapper: swapper, gates: gates}
}

// WithFeeReader attaches an AMMFeeReader for swap fee basis-point lookup (T058 / FR-006).
func (s *SwapService) WithFeeReader(r AMMFeeReader) *SwapService {
	s.feeReader = r
	return s
}

// WithFeeRecorder attaches a SwapFeeRecorder to distribute fees to LPs after each swap (T058 / FR-006).
func (s *SwapService) WithFeeRecorder(r SwapFeeRecorder) *SwapService {
	s.feeRecorder = r
	return s
}

// Execute runs the full swap pipeline: pool-status check → breaker check → ZK validation → AMM quote → slippage check → submit.
func (s *SwapService) Execute(ctx context.Context, req SwapRequest) (*SwapResult, error) {
	logPrefix := ""
	if req.CorrelationID != "" {
		logPrefix = fmt.Sprintf("[correlation_id=%s] ", req.CorrelationID)
	}
	log.Printf("%sswap initiated: pair=%s, amount_out=%s, max_amount_in=%s, payer=%s, beneficiary=%s",
		logPrefix, req.Pair, req.AmountOut, req.MaxAmountIn, req.PayerID, req.BeneficiaryID)

	// 0. Pool Status gate — block swaps on EMPTY / PENDING_COUNTERPART pools (FR-011).
	if s.gates.PoolStatusGate != nil {
		active, err := s.gates.PoolStatusGate.IsActive(ctx, req.Pair)
		if err != nil {
			return nil, fmt.Errorf("pool status check failed: %w", err)
		}
		if !active {
			return nil, &domain.SwapExecError{Code: domain.ErrCodePoolNotActive}
		}
	}

	// 1. Circuit Breaker gate (FR-030)
	if s.gates.CircuitBreakerGate != nil {
		halted, err := s.gates.CircuitBreakerGate.IsHalted(ctx, req.Pair)
		if err != nil {
			return nil, fmt.Errorf("circuit breaker check failed: %w", err)
		}
		if halted {
			return nil, &domain.SwapExecError{Code: domain.ErrCodeCircuitBreakerHalted}
		}
	}

	// 2. ZK-Pointer compliance gate (FR-058)
	if s.gates.ComplianceGate != nil {
		if err := s.gates.ComplianceGate.ValidateZKPointer(ctx, req.PayerID, "", req.ZKPointerPayer); err != nil {
			return nil, &ZKValidationError{Msg: "zk validation failed for payer: " + err.Error()}
		}
		if err := s.gates.ComplianceGate.ValidateZKPointer(ctx, req.BeneficiaryID, "", req.ZKPointerBeneficiary); err != nil {
			return nil, &ZKValidationError{Msg: "zk validation failed for beneficiary: " + err.Error()}
		}
	}

	// 3. AMM quote to get required input
	requiredInput, _, _, err := s.swapper.QuoteExactOutput(ctx, req.Pair, req.AmountOut)
	if err != nil {
		return nil, &domain.SwapExecError{Code: domain.ErrCodeInsufficientPoolLiquidity}
	}
	if requiredInput == "" {
		return nil, &domain.SwapExecError{Code: domain.ErrCodeInsufficientPoolLiquidity}
	}

	// 4. Slippage protection: input must not exceed max_amount_in (FR-059)
	if req.MaxAmountIn != "" {
		reqIn, ok1 := new(big.Int).SetString(requiredInput, 10)
		maxIn, ok2 := new(big.Int).SetString(req.MaxAmountIn, 10)
		if !ok1 || !ok2 {
			return nil, &domain.SwapExecError{Code: domain.ErrCodeSlippageLimitExceeded}
		}
		if reqIn.Cmp(maxIn) > 0 {
			return nil, &domain.SwapExecError{Code: domain.ErrCodeSlippageLimitExceeded}
		}
	}

	// 5. Submit swap to AMM (state → SUBMITTED → COMPLETED)
	orderID, txHash, amountIn, err := s.swapper.SwapExactOutput(ctx,
		req.Pair, req.AmountOut, req.MaxAmountIn,
		req.PayerID, req.BeneficiaryID,
		req.ZKPointerPayer, req.ZKPointerBeneficiary)
	if err != nil {
		log.Printf("%sswap failed: %v", logPrefix, err)
		return nil, fmt.Errorf("swap execution failed: %w", err)
	}

	log.Printf("%sswap completed: order_id=%s, tx_hash=%s, amount_in=%s", logPrefix, orderID, txHash, amountIn)

	result := &SwapResult{
		OrderID:     orderID,
		TxHash:      txHash,
		AmountIn:    amountIn,
		State:       string(domain.SwapStateCompleted),
		ConfirmedAt: time.Now(),
	}

	// D14 / FR-006: Distribute swap fee to active LPs synchronously.
	// Failure is non-blocking — logged as warning, does not affect the swap response.
	if s.feeRecorder != nil && s.feeReader != nil {
		if feeBps, feeErr := s.feeReader.GetFeeBps(ctx, req.Pair); feeErr == nil && feeBps > 0 {
			if amtIn, ok := new(big.Int).SetString(amountIn, 10); ok {
				feeA := new(big.Int).Mul(amtIn, new(big.Int).SetUint64(feeBps))
				feeA.Div(feeA, big.NewInt(10000))
				if err := s.feeRecorder.RecordSwapFee(ctx, req.Pair, orderID, feeA, big.NewInt(0)); err != nil {
					log.Printf("[swap] fee distribution warning order=%s pair=%s: %v", orderID, req.Pair, err)
				}
			}
		}
	}

	return result, nil
}
