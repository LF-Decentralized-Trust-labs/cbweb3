// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ---------------------------------------------------------------------------
// QuoteService (quote_service.go)
// ---------------------------------------------------------------------------

type fakeAMMQuoter struct {
	input     string
	impact    string
	ts        int64
	err       error
	gotPair   string
	gotAmount string
}

func (f *fakeAMMQuoter) QuoteExactOutput(_ context.Context, pair, amountOut string) (string, string, int64, error) {
	f.gotPair = pair
	f.gotAmount = amountOut
	return f.input, f.impact, f.ts, f.err
}

func TestQuoteService_GetExactOutputQuote_Success(t *testing.T) {
	q := &fakeAMMQuoter{input: "1050", impact: "0.5", ts: 1700000000}
	svc := NewQuoteService(q)

	resp, err := svc.GetExactOutputQuote(context.Background(), "W-BRL-W-ARS", "1000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RequiredInput != "1050" || resp.PriceImpact != "0.5" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !resp.QuoteTimestamp.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("unexpected timestamp: %v", resp.QuoteTimestamp)
	}
	if q.gotPair != "W-BRL-W-ARS" || q.gotAmount != "1000" {
		t.Fatalf("quoter received wrong args: %s %s", q.gotPair, q.gotAmount)
	}
}

func TestQuoteService_GetExactOutputQuote_ValidationError(t *testing.T) {
	svc := NewQuoteService(&fakeAMMQuoter{})
	if _, err := svc.GetExactOutputQuote(context.Background(), "", "1000"); err == nil {
		t.Fatal("expected error for empty pair")
	}
	if _, err := svc.GetExactOutputQuote(context.Background(), "PAIR", ""); err == nil {
		t.Fatal("expected error for empty amount")
	}
}

func TestQuoteService_GetExactOutputQuote_QuoterError(t *testing.T) {
	svc := NewQuoteService(&fakeAMMQuoter{err: errors.New("boom")})
	if _, err := svc.GetExactOutputQuote(context.Background(), "PAIR", "1000"); err == nil {
		t.Fatal("expected error when quoter fails")
	}
}

// ---------------------------------------------------------------------------
// SwapQuoteGenerator (swap_quote_generator.go)
// ---------------------------------------------------------------------------

type fakeReserveReader struct {
	reserveA   string
	reserveB   string
	feeBps     uint16
	reserveErr error
	feeErr     error
}

func (f *fakeReserveReader) GetPoolReserves(_ context.Context, _ string) (string, string, float64, error) {
	return f.reserveA, f.reserveB, 0, f.reserveErr
}
func (f *fakeReserveReader) GetFeeBps(_ context.Context, _ string) (uint16, error) {
	return f.feeBps, f.feeErr
}

type fakeQuoteRepo struct {
	created   *apidomain.SwapQuote
	createErr error
}

func (f *fakeQuoteRepo) Create(_ context.Context, q *apidomain.SwapQuote) error {
	f.created = q
	return f.createErr
}
func (f *fakeQuoteRepo) FindByID(_ context.Context, _ string) (*apidomain.SwapQuote, error) {
	return nil, nil
}
func (f *fakeQuoteRepo) DeleteExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func TestSwapQuoteGenerator_Success(t *testing.T) {
	reader := &fakeReserveReader{reserveA: "10000", reserveB: "20000", feeBps: 30}
	repo := &fakeQuoteRepo{}
	g := NewSwapQuoteGenerator(reader, repo)

	res, err := g.GenerateQuote(context.Background(), QuoteRequest{
		SourceCurrency: "BRL",
		TargetCurrency: "ARS",
		AmountOut:      "1000",
		MaxSlippagePct: 0.01,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PoolPair != "W-BRL-W-ARS" {
		t.Fatalf("unexpected pool pair: %s", res.PoolPair)
	}
	if res.AmountIn == "" || res.AmountIn == "0" {
		t.Fatalf("expected non-zero amount in, got %s", res.AmountIn)
	}
	if res.FeeBps != 30 {
		t.Fatalf("unexpected fee bps: %d", res.FeeBps)
	}
	if repo.created == nil {
		t.Fatal("expected quote to be persisted")
	}
	if res.ValidUntil.Before(res.CreatedAt) {
		t.Fatal("valid_until should be after created_at")
	}
}

func TestSwapQuoteGenerator_ClampToMinimumOne(t *testing.T) {
	// Tiny amount_out relative to a huge pool ratio → integer division floors to 0,
	// then clamps to 1 (the getAmountIn floor).
	reader := &fakeReserveReader{reserveA: "10000", reserveB: "20000", feeBps: 0}
	g := NewSwapQuoteGenerator(reader, &fakeQuoteRepo{})
	res, err := g.GenerateQuote(context.Background(), QuoteRequest{
		SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AmountIn != "1" {
		t.Fatalf("expected clamped amount_in of 1, got %s", res.AmountIn)
	}
}

func TestSwapQuoteGenerator_ReserveError(t *testing.T) {
	g := NewSwapQuoteGenerator(&fakeReserveReader{reserveErr: errors.New("rpc down")}, &fakeQuoteRepo{})
	if _, err := g.GenerateQuote(context.Background(), QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "1"}); err == nil {
		t.Fatal("expected reserve fetch error")
	}
}

func TestSwapQuoteGenerator_FeeError(t *testing.T) {
	g := NewSwapQuoteGenerator(&fakeReserveReader{reserveA: "1", reserveB: "1", feeErr: errors.New("fee down")}, &fakeQuoteRepo{})
	if _, err := g.GenerateQuote(context.Background(), QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "1"}); err == nil {
		t.Fatal("expected fee fetch error")
	}
}

func TestSwapQuoteGenerator_InvalidReserves(t *testing.T) {
	g := NewSwapQuoteGenerator(&fakeReserveReader{reserveA: "notanumber", reserveB: "20000"}, &fakeQuoteRepo{})
	if _, err := g.GenerateQuote(context.Background(), QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "1"}); err == nil {
		t.Fatal("expected invalid reserve_a error")
	}
	g = NewSwapQuoteGenerator(&fakeReserveReader{reserveA: "10000", reserveB: "bad"}, &fakeQuoteRepo{})
	if _, err := g.GenerateQuote(context.Background(), QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "1"}); err == nil {
		t.Fatal("expected invalid reserve_b error")
	}
}

func TestSwapQuoteGenerator_InvalidAmountOut(t *testing.T) {
	g := NewSwapQuoteGenerator(&fakeReserveReader{reserveA: "10000", reserveB: "20000"}, &fakeQuoteRepo{})
	if _, err := g.GenerateQuote(context.Background(), QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "bad"}); err == nil {
		t.Fatal("expected invalid amount_out error")
	}
}

func TestSwapQuoteGenerator_InsufficientLiquidity(t *testing.T) {
	// amount_out >= reserve_b → cannot drain pool
	g := NewSwapQuoteGenerator(&fakeReserveReader{reserveA: "10000", reserveB: "1000"}, &fakeQuoteRepo{})
	if _, err := g.GenerateQuote(context.Background(), QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "2000"}); err == nil {
		t.Fatal("expected insufficient liquidity error")
	}
}

func TestSwapQuoteGenerator_PersistError(t *testing.T) {
	g := NewSwapQuoteGenerator(
		&fakeReserveReader{reserveA: "10000", reserveB: "20000", feeBps: 30},
		&fakeQuoteRepo{createErr: errors.New("db down")},
	)
	if _, err := g.GenerateQuote(context.Background(), QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: "1000"}); err == nil {
		t.Fatal("expected persist error")
	}
}
