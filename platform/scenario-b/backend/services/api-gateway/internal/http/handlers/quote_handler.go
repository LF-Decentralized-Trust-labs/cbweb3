// Package handlers provides the AMM quote HTTP handler for Scenario B (FR-027 / SC-013).
package handlers

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// QuoteServiceIface is the interface consumed by QuoteHandler.
type QuoteServiceIface interface {
	GetExactOutputQuote(ctx context.Context, pair, amountOut string) (*services.QuoteResponse, error)
}

// SwapQuoteGeneratorIface generates cross-currency swap quotes with TTL (T030).
type SwapQuoteGeneratorIface interface {
	GenerateQuote(ctx context.Context, req services.QuoteRequest) (*services.QuoteResult, error)
}

// QuoteHandler handles GET /api/v2/amm/quote/exact-output and /quote/cross-currency.
type QuoteHandler struct {
	svc           QuoteServiceIface
	quoteGen      SwapQuoteGeneratorIface
}

// NewQuoteHandler creates a QuoteHandler.
func NewQuoteHandler(svc QuoteServiceIface, quoteGen SwapQuoteGeneratorIface) *QuoteHandler {
	return &QuoteHandler{
		svc:      svc,
		quoteGen: quoteGen,
	}
}

// GetExactOutputQuote returns the required input amount for an exact-output swap.
// No authentication required — public consultation endpoint (FR-027).
func (h *QuoteHandler) GetExactOutputQuote(c *fiber.Ctx) error {
	pair := c.Query("pair")
	amountOut := c.Query("amount_out")

	if pair == "" || amountOut == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "pair and amount_out are required query parameters",
			"error_code": "INVALID_REQUEST",
		})
	}

	resp, err := h.svc.GetExactOutputQuote(c.Context(), pair, amountOut)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      err.Error(),
			"error_code": "INSUFFICIENT_POOL_LIQUIDITY",
		})
	}

	return c.JSON(fiber.Map{
		"required_input":  resp.RequiredInput,
		"price_impact":    resp.PriceImpact,
		"quote_timestamp": resp.QuoteTimestamp,
	})
}

// GetCrossCurrencyQuote generates a cross-currency swap quote with 15s TTL (T032).
// Returns quote_id, created_at, valid_until, time_remaining_seconds for server-side expiry validation.
func (h *QuoteHandler) GetCrossCurrencyQuote(c *fiber.Ctx) error {
	sourceCurrency := c.Query("source_currency")
	targetCurrency := c.Query("target_currency")
	amountOut := c.Query("amount_out")
	maxSlippagePct := c.QueryFloat("max_slippage_pct", 0.01) // Default 1%

	if sourceCurrency == "" || targetCurrency == "" || amountOut == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "source_currency, target_currency, and amount_out are required query parameters",
			"error_code": "INVALID_REQUEST",
		})
	}

	req := services.QuoteRequest{
		SourceCurrency: sourceCurrency,
		TargetCurrency: targetCurrency,
		AmountOut:      amountOut,
		MaxSlippagePct: maxSlippagePct,
	}

	result, err := h.quoteGen.GenerateQuote(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      err.Error(),
			"error_code": "INSUFFICIENT_POOL_LIQUIDITY",
		})
	}

	return c.JSON(fiber.Map{
		"quote_id":              result.QuoteID,
		"pool_pair":             result.PoolPair,
		"amount_out":            result.AmountOut,
		"amount_in":             result.AmountIn,
		"effective_rate":        result.EffectiveRate,
		"fee_bps":               result.FeeBps,
		"max_slippage_pct":      result.MaxSlippagePct,
		"reserve_a_snapshot":    result.ReserveASnapshot,
		"reserve_b_snapshot":    result.ReserveBSnapshot,
		"created_at":            result.CreatedAt.Unix(),
		"valid_until":           result.ValidUntil.Unix(),
		"time_remaining_seconds": result.TimeRemainingSeconds,
	})
}

