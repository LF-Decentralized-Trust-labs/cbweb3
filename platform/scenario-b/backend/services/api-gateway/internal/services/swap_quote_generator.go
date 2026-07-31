// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"fmt"
	"math/big"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
)

// SwapQuoteGenerator generates swap quotes with 15s TTL (T030).
// Calculates required amount_in via x·y=k + fee from pool reserves.
type SwapQuoteGenerator struct {
	ammAdapter AMMReserveReader
	quoteRepo  SwapQuoteRepository
	quoteTTL   time.Duration
}

// AMMReserveReader reads current pool reserves for quote calculation.
type AMMReserveReader interface {
	GetPoolReserves(ctx context.Context, pair string) (string, string, float64, error)
	GetFeeBps(ctx context.Context, pair string) (uint16, error)
	// OutputIsTokenA reports whether buying targetCurrency on pair outputs TOKEN_A,
	// so the quote is oriented to the requested direction (bidirectional pair).
	OutputIsTokenA(ctx context.Context, pair, targetCurrency string) (bool, error)
}

// SwapQuoteRepository persists and reads swap quotes (T031, T034, T036).
type SwapQuoteRepository interface {
	Create(ctx context.Context, quote *apidomain.SwapQuote) error
	FindByID(ctx context.Context, quoteID string) (*apidomain.SwapQuote, error)
	DeleteExpired(ctx context.Context, cutoffTime time.Time) (int64, error)
}

// NewSwapQuoteGenerator creates a new quote generator with 15s TTL.
func NewSwapQuoteGenerator(ammAdapter AMMReserveReader, quoteRepo SwapQuoteRepository) *SwapQuoteGenerator {
	return &SwapQuoteGenerator{
		ammAdapter: ammAdapter,
		quoteRepo:  quoteRepo,
		quoteTTL:   15 * time.Second, // FR-012: 15s expiry
	}
}

// QuoteRequest contains quote generation parameters.
type QuoteRequest struct {
	SourceCurrency string  // Native spoke currency (e.g., "BRL")
	TargetCurrency string  // Native spoke currency (e.g., "ARS")
	AmountOut      string  // Exact amount beneficiary receives (wei)
	MaxSlippagePct float64 // Tolerance (e.g., 0.01 = 1%)
	// PoolPair is the exact on-chain pair_id to quote against. When empty, it is
	// derived as "W-{source}-W-{target}" for backward compatibility, but callers
	// that know the registered pair_id (e.g. the bank Swap page) should pass it —
	// pair ids do not always follow the derived convention (e.g. "W-tCeBM_BRL-...").
	PoolPair string
}

// QuoteResult contains the generated quote with TTL.
type QuoteResult struct {
	QuoteID              string  // UUID for tracking
	PoolPair             string  // W-BRL-W-ARS
	AmountOut            string  // Requested amount (wei)
	AmountIn             string  // Required input (wei, including fee)
	EffectiveRate        float64 // amount_out / amount_in
	FeeBps               uint16  // Pool fee in basis points
	MaxSlippagePct       float64 // User tolerance
	ReserveASnapshot     string  // Reserve A at quote time
	ReserveBSnapshot     string  // Reserve B at quote time
	CreatedAt            time.Time
	ValidUntil           time.Time
	TimeRemainingSeconds int // Time until expiry
}

// GenerateQuote creates a new swap quote with 15s TTL (T030).
// Calculates amount_in using constant-product formula x·y=k with fees:
//
//	amount_in_no_fee = (reserve_in × amount_out) / (reserve_out - amount_out)
//	amount_in = amount_in_no_fee × (1 + fee_bps / 10000)
func (g *SwapQuoteGenerator) GenerateQuote(ctx context.Context, req QuoteRequest) (*QuoteResult, error) {
	// Use the explicit pair_id when provided; otherwise derive W-{source}-W-{target}.
	poolPair := req.PoolPair
	if poolPair == "" {
		poolPair = fmt.Sprintf("W-%s-W-%s", req.SourceCurrency, req.TargetCurrency)
	}

	// Fetch current pool reserves
	reserveAStr, reserveBStr, _, err := g.ammAdapter.GetPoolReserves(ctx, poolPair)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pool reserves for %s: %w", poolPair, err)
	}

	// Fetch pool fee
	feeBps, err := g.ammAdapter.GetFeeBps(ctx, poolPair)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fee for %s: %w", poolPair, err)
	}

	// Parse reserves (big.Int for precision)
	reserveA, ok := new(big.Int).SetString(reserveAStr, 10)
	if !ok {
		return nil, fmt.Errorf("invalid reserve_a: %s", reserveAStr)
	}
	reserveB, ok := new(big.Int).SetString(reserveBStr, 10)
	if !ok {
		return nil, fmt.Errorf("invalid reserve_b: %s", reserveBStr)
	}

	// Parse amount_out
	amountOut, ok := new(big.Int).SetString(req.AmountOut, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount_out: %s", req.AmountOut)
	}

	// Orient reserves by swap direction. amount_out is always in the TARGET token; when the
	// target is the pair's TOKEN_A (reverse corridor, e.g. COP→BRL on a BRL↔COP pair) the
	// input reserve is B and the output reserve is A. Defaults to A→B when unresolved.
	reserveIn, reserveOut := reserveA, reserveB
	if isA, dErr := g.ammAdapter.OutputIsTokenA(ctx, poolPair, req.TargetCurrency); dErr == nil && isA {
		reserveIn, reserveOut = reserveB, reserveA
	}

	// Validate amount_out < reserve_out (can't drain pool)
	if amountOut.Cmp(reserveOut) >= 0 {
		return nil, fmt.Errorf("amount_out %s >= output reserve %s (insufficient liquidity)", req.AmountOut, reserveOut.String())
	}

	// R2-H-3: fee-aware exact-output quote as a SINGLE ceiling division, identical to the on-chain
	// AutomatedMarketMaker.quoteExactOutput / swapTokensForExactTokens charge:
	//   amount_in = ceil( reserve_in * amount_out * 10000 / ((reserve_out - amount_out) * (10000 - fee_bps)) )
	// The prior two-step form took the FLOOR of the constant-product input and then grossed up by
	// (10000 + fee)/10000 — the wrong direction — so the quote could land BELOW the on-chain
	// charge. A client passing it straight through as max_amount_in would then revert with
	// AMM__SlippageExceeded. One ceiling rounds up exactly once, in favor of the pool, so the
	// quoted amount_in is always sufficient.
	numerator := new(big.Int).Mul(reserveIn, amountOut)
	numerator.Mul(numerator, big.NewInt(10000))
	denominator := new(big.Int).Sub(reserveOut, amountOut)
	denominator.Mul(denominator, big.NewInt(int64(10000)-int64(feeBps)))

	// ceil(numerator / denominator) = (numerator + denominator - 1) / denominator. Both are > 0:
	// amount_out < reserve_out was validated above, and fee_bps <= MAX_FEE_BPS (1000) keeps
	// (10000 - fee_bps) >= 9000.
	amountInWithFee := new(big.Int).Add(numerator, new(big.Int).Sub(denominator, big.NewInt(1)))
	amountInWithFee.Div(amountInWithFee, denominator)

	// A swap always costs at least 1 base unit of the input token. The ceiling already guarantees
	// this for any positive amount_out; this only covers a degenerate amount_out == 0 quote.
	if amountInWithFee.Sign() == 0 {
		amountInWithFee.SetInt64(1)
	}

	// Calculate effective rate (amount_out / amount_in)
	amountOutFloat := new(big.Float).SetInt(amountOut)
	amountInFloat := new(big.Float).SetInt(amountInWithFee)
	effectiveRate, _ := new(big.Float).Quo(amountOutFloat, amountInFloat).Float64()

	// Generate quote ID and timestamps
	quoteID := uuid.New().String()
	createdAt := time.Now().UTC()
	validUntil := createdAt.Add(g.quoteTTL)
	timeRemaining := int(time.Until(validUntil).Seconds())

	// Create domain entity
	quote := &apidomain.SwapQuote{
		QuoteID:          quoteID,
		PoolPair:         poolPair,
		AmountOut:        req.AmountOut,
		AmountIn:         amountInWithFee.String(),
		EffectiveRate:    effectiveRate,
		FeeBps:           int(feeBps),
		MaxSlippagePct:   req.MaxSlippagePct,
		ReserveASnapshot: reserveAStr,
		ReserveBSnapshot: reserveBStr,
		CreatedAt:        createdAt,
		ValidUntil:       validUntil,
	}

	// Persist to database (T031)
	if err := g.quoteRepo.Create(ctx, quote); err != nil {
		return nil, fmt.Errorf("failed to persist quote: %w", err)
	}

	// Return result
	return &QuoteResult{
		QuoteID:              quoteID,
		PoolPair:             poolPair,
		AmountOut:            req.AmountOut,
		AmountIn:             amountInWithFee.String(),
		EffectiveRate:        effectiveRate,
		FeeBps:               feeBps,
		MaxSlippagePct:       req.MaxSlippagePct,
		ReserveASnapshot:     reserveAStr,
		ReserveBSnapshot:     reserveBStr,
		CreatedAt:            createdAt,
		ValidUntil:           validUntil,
		TimeRemainingSeconds: timeRemaining,
	}, nil
}
