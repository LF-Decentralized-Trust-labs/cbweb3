// Package handlers provides the cross-currency swap HTTP handler for commercial banks
// (009-commercial-cross-currency-swap / FR-001).
package handlers

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// CrossCurrencySwapOrchestratorIface is the interface consumed by CrossCurrencySwapHandler.
type CrossCurrencySwapOrchestratorIface interface {
	Execute(ctx context.Context, req services.CrossCurrencySwapRequest) (*services.CrossCurrencySwapResult, error)
	GetStatus(ctx context.Context, swapID string) (*services.CrossCurrencySwapResult, error)
}

// CrossCurrencySwapHandler handles POST /api/v2/amm/swap/cross-currency.
type CrossCurrencySwapHandler struct {
	orchestrator     CrossCurrencySwapOrchestratorIface
	fallbackBankCode string
}

// NewCrossCurrencySwapHandler creates a CrossCurrencySwapHandler.
func NewCrossCurrencySwapHandler(orchestrator CrossCurrencySwapOrchestratorIface, fallbackBankCode string) *CrossCurrencySwapHandler {
	return &CrossCurrencySwapHandler{
		orchestrator:     orchestrator,
		fallbackBankCode: strings.TrimSpace(fallbackBankCode),
	}
}

// crossCurrencySwapRequest is the JSON body for a cross-currency swap request (FR-001).
type crossCurrencySwapRequest struct {
	SourceCurrency string `json:"source_currency"`
	TargetCurrency string `json:"target_currency"`
	PoolPair       string `json:"pool_pair"`
	AmountOut      string `json:"amount_out"`
	MaxAmountIn    string `json:"max_amount_in"`
	// Deprecated: payer_bank_id is derived from authenticated claims.
	PayerBankID       string  `json:"payer_bank_id,omitempty"`
	BeneficiaryBankID string  `json:"beneficiary_bank_id"`
	QuoteID           *string `json:"quote_id,omitempty"`
}

// crossCurrencySwapResponse is the JSON response for a successful swap (FR-001).
type crossCurrencySwapResponse struct {
	SwapID              string  `json:"swap_id"`
	CorrelationID       string  `json:"correlation_id"`
	Status              string  `json:"status"`
	AmountIn            string  `json:"amount_in"`
	AmountOut           string  `json:"amount_out"`
	EffectiveRate       float64 `json:"effective_rate"`
	BridgeInPositionID  string  `json:"bridge_in_position_id"`
	SwapTxHash          string  `json:"swap_tx_hash,omitempty"`
	BridgeOutPositionID string  `json:"bridge_out_position_id,omitempty"`
	CreatedAt           string  `json:"created_at"`
}

// SwapCrossCurrency executes an end-to-end cross-currency swap (bridge-in → swap Hub → bridge-out).
// Requires commercial_bank role (FR-001).
// Rate limited: 10 swaps/min, 100 swaps/hour per payer_bank_id (FR-008).
func (h *CrossCurrencySwapHandler) SwapCrossCurrency(c *fiber.Ctx) error {
	var req crossCurrencySwapRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "invalid request body",
			"error_code": "INVALID_REQUEST",
			"details":    err.Error(),
		})
	}

	// Validate required fields
	if req.SourceCurrency == "" || req.TargetCurrency == "" || req.PoolPair == "" ||
		req.AmountOut == "" || req.MaxAmountIn == "" || req.BeneficiaryBankID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "source_currency, target_currency, pool_pair, amount_out, max_amount_in, beneficiary_bank_id are required",
			"error_code": "INVALID_REQUEST",
		})
	}

	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":      "missing authenticated claims",
			"error_code": "UNAUTHENTICATED",
		})
	}
	payerBankID := strings.TrimSpace(claims.BankID)
	if payerBankID == "" {
		payerBankID = h.fallbackBankCode
	}
	if payerBankID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":      "unable to determine payer bank from authenticated session",
			"error_code": "UNAUTHENTICATED",
		})
	}
	if req.PayerBankID != "" && req.PayerBankID != payerBankID {
		log.Printf("[cross-currency] deprecated payer_bank_id payload (%s) ignored; using authenticated bank %s", req.PayerBankID, payerBankID)
	}

	// Generate swap_id and correlation_id for tracking
	swapID := uuid.NewString()
	correlationID := uuid.NewString()

	// Execute orchestrated swap
	result, err := h.orchestrator.Execute(c.Context(), services.CrossCurrencySwapRequest{
		SwapID:            swapID,
		CorrelationID:     correlationID,
		SourceCurrency:    req.SourceCurrency,
		TargetCurrency:    req.TargetCurrency,
		PoolPair:          req.PoolPair,
		AmountOut:         req.AmountOut,
		MaxAmountIn:       req.MaxAmountIn,
		PayerBankID:       payerBankID,
		BeneficiaryBankID: req.BeneficiaryBankID,
		QuoteID:           req.QuoteID,
	})
	if err != nil {
		return h.handleCrossCurrencySwapError(c, err)
	}

	// Map result to response
	resp := crossCurrencySwapResponse{
		SwapID:              result.SwapID,
		CorrelationID:       result.CorrelationID,
		Status:              string(result.Status),
		AmountIn:            result.AmountIn,
		AmountOut:           result.AmountOut,
		EffectiveRate:       result.EffectiveRate,
		BridgeInPositionID:  result.BridgeInPositionID,
		SwapTxHash:          result.SwapTxHash,
		BridgeOutPositionID: result.BridgeOutPositionID,
		CreatedAt:           result.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return c.JSON(resp)
}

// crossCurrencySwapStatusResponse is the JSON response for GET /swap/cross-currency/:id.
type crossCurrencySwapStatusResponse struct {
	SwapID              string  `json:"swap_id"`
	CorrelationID       string  `json:"correlation_id"`
	Status              string  `json:"status"`
	AmountIn            string  `json:"amount_in,omitempty"`
	AmountOut           string  `json:"amount_out,omitempty"`
	EffectiveRate       float64 `json:"effective_rate,omitempty"`
	BridgeInPositionID  string  `json:"bridge_in_position_id,omitempty"`
	SwapTxHash          string  `json:"swap_tx_hash,omitempty"`
	BridgeOutPositionID string  `json:"bridge_out_position_id,omitempty"`
	CreatedAt           string  `json:"created_at"`
	CompletedAt         string  `json:"completed_at,omitempty"`
	FailureReason       string  `json:"failure_reason,omitempty"`
}

// GetSwapStatus returns the current status of a cross-currency swap by swap_id.
// GET /api/v2/amm/swap/cross-currency/:id
// Requires commercial_bank role (FR-001).
func (h *CrossCurrencySwapHandler) GetSwapStatus(c *fiber.Ctx) error {
	swapID := c.Params("id")
	if swapID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "swap_id is required",
			"error_code": "INVALID_REQUEST",
		})
	}

	result, err := h.orchestrator.GetStatus(c.Context(), swapID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "record not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":      "swap not found",
				"error_code": "SWAP_NOT_FOUND",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      "failed to retrieve swap status",
			"error_code": "INTERNAL_ERROR",
			"details":    err.Error(),
		})
	}

	resp := crossCurrencySwapStatusResponse{
		SwapID:              result.SwapID,
		CorrelationID:       result.CorrelationID,
		Status:              string(result.Status),
		AmountIn:            result.AmountIn,
		AmountOut:           result.AmountOut,
		EffectiveRate:       result.EffectiveRate,
		BridgeInPositionID:  result.BridgeInPositionID,
		SwapTxHash:          result.SwapTxHash,
		BridgeOutPositionID: result.BridgeOutPositionID,
		FailureReason:       result.FailureReason,
		CreatedAt:           result.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if result.CompletedAt != nil {
		resp.CompletedAt = result.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return c.JSON(resp)
}

// handleCrossCurrencySwapError maps service errors to HTTP responses with user-friendly messages (FR-004, T024).
func (h *CrossCurrencySwapHandler) handleCrossCurrencySwapError(c *fiber.Ctx, err error) error {
	// Check for domain-specific errors
	var execErr *domain.SwapExecError
	if errors.As(err, &execErr) {
		return h.mapSwapExecError(c, execErr)
	}

	// R1-10.1: Daily transfer limit exceeded — surface a clear 422 with actionable message.
	var limitErr *services.ErrTransferLimitExceeded
	if errors.As(err, &limitErr) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              err.Error(),
			"error_code":         "TRANSFER_LIMIT_EXCEEDED",
			"recommended_action": "Contact your Central Bank to review or increase the daily transfer limit.",
		})
	}

	// Check for pool not active error (from orchestrator pre-validation)
	if containsError(err, "pool") && containsError(err, "not ACTIVE") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "pool is not active",
			"error_code":         "POOL_NOT_ACTIVE",
			"recommended_action": "Wait for central banks to provision liquidity",
		})
	}

	// Check for circuit breaker halted error
	if containsError(err, "circuit breaker") && containsError(err, "HALTED") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "pool is halted by circuit breaker",
			"error_code":         "CIRCUIT_BREAKER_HALTED",
			"recommended_action": "Wait for governance to resume pool",
		})
	}

	// Check for bridge-in failure
	if containsError(err, "bridge-in failed") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              err.Error(),
			"error_code":         "BRIDGE_IN_FAILED",
			"recommended_action": "Check spoke network connectivity and try again",
		})
	}

	// Check for swap failure
	if containsError(err, "swap failed") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              err.Error(),
			"error_code":         "SWAP_FAILED",
			"recommended_action": "Check pool liquidity and slippage tolerance",
		})
	}

	// Check for bridge-out failure (partial success)
	if containsError(err, "bridge-out failed") {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":              err.Error(),
			"error_code":         "BRIDGE_OUT_FAILED",
			"recommended_action": "Swap succeeded but bridge-out failed. Contact support for manual intervention.",
		})
	}

	// Generic internal error
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error":      "cross-currency swap failed",
		"error_code": "INTERNAL_ERROR",
		"details":    err.Error(),
	})
}

// mapSwapExecError maps domain.SwapExecError codes to HTTP 422 responses.
func (h *CrossCurrencySwapHandler) mapSwapExecError(c *fiber.Ctx, execErr *domain.SwapExecError) error {
	switch execErr.Code {
	case domain.ErrCodePoolNotActive:
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "pool is not active",
			"error_code":         "POOL_NOT_ACTIVE",
			"recommended_action": "Wait for central banks to provision liquidity",
		})

	case domain.ErrCodeCircuitBreakerHalted:
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "pool is halted by circuit breaker",
			"error_code":         "CIRCUIT_BREAKER_HALTED",
			"recommended_action": "Wait for governance to resume pool",
		})

	case domain.ErrCodeSlippageLimitExceeded:
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "slippage limit exceeded",
			"error_code":         "SLIPPAGE_LIMIT_EXCEEDED",
			"recommended_action": "Increase max_amount_in or obtain new quote",
		})

	case domain.ErrCodeInsufficientPoolLiquidity:
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "insufficient pool liquidity",
			"error_code":         "INSUFFICIENT_POOL_LIQUIDITY",
			"recommended_action": "Reduce amount_out or wait for liquidity provision",
		})

	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      "swap execution failed",
			"error_code": "INTERNAL_ERROR",
		})
	}
}

// containsError checks if an error message contains a substring (case-insensitive).
func containsError(err error, substr string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr))
}
