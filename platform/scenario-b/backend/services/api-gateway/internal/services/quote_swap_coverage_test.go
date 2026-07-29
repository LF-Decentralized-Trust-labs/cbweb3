// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"math/big"
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

func (f *fakeAMMQuoter) QuoteExactOutput(_ context.Context, pair, amountOut string, _ bool) (string, string, int64, error) {
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
	reserveA       string
	reserveB       string
	feeBps         uint16
	outputIsTokenA bool
	reserveErr     error
	feeErr         error
}

func (f *fakeReserveReader) GetPoolReserves(_ context.Context, _ string) (string, string, float64, error) {
	return f.reserveA, f.reserveB, 0, f.reserveErr
}
func (f *fakeReserveReader) OutputIsTokenA(_ context.Context, _, _ string) (bool, error) {
	return f.outputIsTokenA, nil
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

// A reverse-direction quote (target is the pair's TOKEN_A, e.g. COP→BRL on a BRL↔COP
// pair) must orient the constant-product formula with reserveB as the input reserve and
// reserveA as the output reserve. With reserveA=10000, reserveB=20000, amountOut=1000,
// fee=0: A→B costs ceil(10000*1000/(20000-1000))=527, while B→A costs
// ceil(20000*1000/(10000-1000))=2223. These are the single-ceiling values (R2-H-3) that equal
// the on-chain charge; the previous floor form under-quoted them as 526 / 2222.
func TestSwapQuoteGenerator_ReverseDirectionOrientsReserves(t *testing.T) {
	repo := &fakeQuoteRepo{}
	fwd := &fakeReserveReader{reserveA: "10000", reserveB: "20000", feeBps: 0, outputIsTokenA: false}
	rev := &fakeReserveReader{reserveA: "10000", reserveB: "20000", feeBps: 0, outputIsTokenA: true}

	fwdRes, err := NewSwapQuoteGenerator(fwd, repo).GenerateQuote(context.Background(),
		QuoteRequest{SourceCurrency: "ARS", TargetCurrency: "BRL", AmountOut: "1000", PoolPair: "W-tCeBM_BRL-W-tCeBM_ARS"})
	if err != nil {
		t.Fatalf("forward quote error: %v", err)
	}
	revRes, err := NewSwapQuoteGenerator(rev, repo).GenerateQuote(context.Background(),
		QuoteRequest{SourceCurrency: "COP", TargetCurrency: "BRL", AmountOut: "1000", PoolPair: "W-tCeBM_BRL-W-tCeBM_COP"})
	if err != nil {
		t.Fatalf("reverse quote error: %v", err)
	}
	if fwdRes.AmountIn != "527" {
		t.Errorf("forward (A→B) amount_in = %s, want 527", fwdRes.AmountIn)
	}
	if revRes.AmountIn != "2223" {
		t.Errorf("reverse (B→A) amount_in = %s, want 2223", revRes.AmountIn)
	}
}

// Reverse direction validates against the OUTPUT reserve (reserveA), not reserveB:
// amountOut just below reserveA must be accepted even when it exceeds reserveB.
func TestSwapQuoteGenerator_ReverseDirectionUsesOutputReserveForLiquidityCheck(t *testing.T) {
	// reserveA=20000 (output), reserveB=10000 (input). amountOut=15000 > reserveB but < reserveA.
	rev := &fakeReserveReader{reserveA: "20000", reserveB: "10000", feeBps: 0, outputIsTokenA: true}
	_, err := NewSwapQuoteGenerator(rev, &fakeQuoteRepo{}).GenerateQuote(context.Background(),
		QuoteRequest{SourceCurrency: "COP", TargetCurrency: "BRL", AmountOut: "15000", PoolPair: "W-tCeBM_BRL-W-tCeBM_COP"})
	if err != nil {
		t.Fatalf("reverse quote should accept amountOut<reserveA: %v", err)
	}
}

func TestSwapQuoteGenerator_ClampToMinimumOne(t *testing.T) {
	// Tiny amount_out relative to a huge pool ratio: the single ceiling division already yields 1
	// (ceil of a positive-but-sub-unit ratio), so a swap always costs at least 1 input base unit.
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

// R2-H-3 regression (finding 2): the gateway quote must NEVER land below the on-chain charge,
// otherwise a client passing quote.amount_in straight through as max_amount_in reverts with
// AMM__SlippageExceeded. Uses the exact scenario from the PR review (reserveA=reserveB=1e23,
// amountOut=1e21, feeBps=30), where the old floor+wrong-grossup under-quoted by ~9.1e12 wei.
// Asserts sufficiency (amountIn*D >= N) and minimality ((amountIn-1)*D < N) — i.e. the quote is
// exactly the single ceiling the contract charges.
func TestSwapQuoteGenerator_QuoteMatchesOnChainCeiling(t *testing.T) {
	const (
		reserve = "100000000000000000000000" // 1e23
		amount  = "1000000000000000000000"   // 1e21
		feeBps  = uint16(30)
	)
	reader := &fakeReserveReader{reserveA: reserve, reserveB: reserve, feeBps: feeBps}
	res, err := NewSwapQuoteGenerator(reader, &fakeQuoteRepo{}).GenerateQuote(context.Background(),
		QuoteRequest{SourceCurrency: "BRL", TargetCurrency: "ARS", AmountOut: amount})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	amountIn, ok := new(big.Int).SetString(res.AmountIn, 10)
	if !ok {
		t.Fatalf("amount_in not an integer: %s", res.AmountIn)
	}

	reserveIn, _ := new(big.Int).SetString(reserve, 10)
	reserveOut, _ := new(big.Int).SetString(reserve, 10)
	amountOut, _ := new(big.Int).SetString(amount, 10)

	// N = reserveIn * amountOut * 10000 ; D = (reserveOut - amountOut) * (10000 - feeBps)
	n := new(big.Int).Mul(reserveIn, amountOut)
	n.Mul(n, big.NewInt(10000))
	d := new(big.Int).Sub(reserveOut, amountOut)
	d.Mul(d, big.NewInt(int64(10000)-int64(feeBps)))

	// Sufficiency: amountIn * D >= N (never undershoots the on-chain charge).
	if new(big.Int).Mul(amountIn, d).Cmp(n) < 0 {
		t.Errorf("quote %s undershoots on-chain charge (amountIn*D < N) — would revert AMM__SlippageExceeded", res.AmountIn)
	}
	// Minimality: (amountIn - 1) * D < N (single ceiling, no double round-up).
	prev := new(big.Int).Sub(amountIn, big.NewInt(1))
	if new(big.Int).Mul(prev, d).Cmp(n) >= 0 {
		t.Errorf("quote %s is not the minimal ceiling ((amountIn-1)*D >= N)", res.AmountIn)
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
