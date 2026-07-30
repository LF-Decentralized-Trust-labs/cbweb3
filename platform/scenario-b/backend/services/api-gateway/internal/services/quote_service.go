// SPDX-License-Identifier: Apache-2.0

// Package services provides the AMM quote service for Scenario B (FR-037 / SC-021).
package services

import (
	"context"
	"fmt"
	"time"
)

// AMMQuoter is the interface for querying exact-output quotes from the Hub AMM.
// outputIsTokenA selects the direction (false = A→B, output TOKEN_B; true = B→A).
type AMMQuoter interface {
	QuoteExactOutput(ctx context.Context, pair, amountOut string, outputIsTokenA bool) (requiredInput, priceImpact string, quoteTimestamp int64, err error)
}

// QuoteResponse holds the result of an exact-output quote request.
type QuoteResponse struct {
	RequiredInput  string    `json:"required_input"`
	PriceImpact    string    `json:"price_impact"`
	QuoteTimestamp time.Time `json:"quote_timestamp"`
}

// QuoteService retrieves AMM quotes for exact-output swaps (FR-037).
// Performance gate: p95 <= 300ms (SC-021).
type QuoteService struct {
	quoter AMMQuoter
}

// NewQuoteService creates a QuoteService backed by the given AMM quoter.
func NewQuoteService(quoter AMMQuoter) *QuoteService {
	return &QuoteService{quoter: quoter}
}

// GetExactOutputQuote retrieves the required input amount for an exact-output swap.
func (s *QuoteService) GetExactOutputQuote(ctx context.Context, pair, amountOut string) (*QuoteResponse, error) {
	if pair == "" || amountOut == "" {
		return nil, fmt.Errorf("pair and amount_out are required")
	}

	// Direct exact-output quote is pair-oriented (buy TOKEN_B); the cross-currency
	// direction is handled by the swap quote generator / orchestrator instead.
	requiredInput, priceImpact, ts, err := s.quoter.QuoteExactOutput(ctx, pair, amountOut, false)
	if err != nil {
		return nil, fmt.Errorf("amm quote failed: %w", err)
	}

	return &QuoteResponse{
		RequiredInput:  requiredInput,
		PriceImpact:    priceImpact,
		QuoteTimestamp: time.Unix(ts, 0),
	}, nil
}
