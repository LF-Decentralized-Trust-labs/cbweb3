// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ---------------------------------------------------------------------------
// fakes for SwapService
// ---------------------------------------------------------------------------

type fakeAMMSwapper struct {
	quoteInput string
	quoteErr   error

	orderID  string
	txHash   string
	amountIn string
	swapErr  error

	swapCalled bool
}

func (f *fakeAMMSwapper) QuoteExactOutput(_ context.Context, _, _ string) (string, string, int64, error) {
	return f.quoteInput, "0", 0, f.quoteErr
}
func (f *fakeAMMSwapper) SwapExactOutput(_ context.Context, _, _, _, _, _, _, _ string) (string, string, string, error) {
	f.swapCalled = true
	return f.orderID, f.txHash, f.amountIn, f.swapErr
}

type fakePoolStatusGate struct {
	active bool
	err    error
}

func (f fakePoolStatusGate) IsActive(_ context.Context, _ string) (bool, error) {
	return f.active, f.err
}

type fakeBreakerGate struct {
	halted bool
	err    error
}

func (f fakeBreakerGate) IsHalted(_ context.Context, _ string) (bool, error) {
	return f.halted, f.err
}

type fakeComplianceGate struct {
	err error
}

func (f fakeComplianceGate) ValidateZKPointer(_ context.Context, _, _, _ string) error {
	return f.err
}

type fakeFeeReader struct {
	feeBps uint64
	err    error
}

func (f fakeFeeReader) GetFeeBps(_ context.Context) (uint64, error) { return f.feeBps, f.err }

type fakeFeeRecorder struct {
	called bool
	feeA   *big.Int
	err    error
}

func (f *fakeFeeRecorder) RecordSwapFee(_ context.Context, _, _ string, feeA, _ *big.Int) error {
	f.called = true
	f.feeA = feeA
	return f.err
}

func okSwapper() *fakeAMMSwapper {
	return &fakeAMMSwapper{quoteInput: "950", orderID: "ord-1", txHash: "0xabc", amountIn: "950"}
}

func baseReq() SwapRequest {
	return SwapRequest{
		Pair:          "W-BRL-W-ARS",
		AmountOut:     "1000",
		MaxAmountIn:   "1000",
		PayerID:       "bank-a",
		BeneficiaryID: "bank-b",
		CorrelationID: "corr-x",
	}
}

func TestSwapService_Execute_HappyPath_NoGates(t *testing.T) {
	sw := okSwapper()
	svc := NewSwapService(sw, SwapGates{})
	res, err := svc.Execute(context.Background(), baseReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrderID != "ord-1" || res.State != string(domain.SwapStateCompleted) {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !sw.swapCalled {
		t.Fatal("expected swap to be submitted")
	}
}

func TestSwapService_Execute_AllGatesPass(t *testing.T) {
	svc := NewSwapService(okSwapper(), SwapGates{
		PoolStatusGate:     fakePoolStatusGate{active: true},
		CircuitBreakerGate: fakeBreakerGate{halted: false},
		ComplianceGate:     fakeComplianceGate{},
	})
	if _, err := svc.Execute(context.Background(), baseReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSwapService_Execute_PoolNotActive(t *testing.T) {
	svc := NewSwapService(okSwapper(), SwapGates{PoolStatusGate: fakePoolStatusGate{active: false}})
	_, err := svc.Execute(context.Background(), baseReq())
	var se *domain.SwapExecError
	if !errors.As(err, &se) || se.Code != domain.ErrCodePoolNotActive {
		t.Fatalf("expected POOL_NOT_ACTIVE, got %v", err)
	}
}

func TestSwapService_Execute_PoolStatusError(t *testing.T) {
	svc := NewSwapService(okSwapper(), SwapGates{PoolStatusGate: fakePoolStatusGate{err: errors.New("rpc")}})
	if _, err := svc.Execute(context.Background(), baseReq()); err == nil {
		t.Fatal("expected pool status error")
	}
}

func TestSwapService_Execute_BreakerHalted(t *testing.T) {
	svc := NewSwapService(okSwapper(), SwapGates{CircuitBreakerGate: fakeBreakerGate{halted: true}})
	_, err := svc.Execute(context.Background(), baseReq())
	var se *domain.SwapExecError
	if !errors.As(err, &se) || se.Code != domain.ErrCodeCircuitBreakerHalted {
		t.Fatalf("expected CIRCUIT_BREAKER_HALTED, got %v", err)
	}
}

func TestSwapService_Execute_BreakerError(t *testing.T) {
	svc := NewSwapService(okSwapper(), SwapGates{CircuitBreakerGate: fakeBreakerGate{err: errors.New("rpc")}})
	if _, err := svc.Execute(context.Background(), baseReq()); err == nil {
		t.Fatal("expected breaker check error")
	}
}

func TestSwapService_Execute_ComplianceDenyPayer(t *testing.T) {
	svc := NewSwapService(okSwapper(), SwapGates{ComplianceGate: fakeComplianceGate{err: errors.New("denied")}})
	_, err := svc.Execute(context.Background(), baseReq())
	var zk *ZKValidationError
	if !errors.As(err, &zk) {
		t.Fatalf("expected ZKValidationError, got %v", err)
	}
}

func TestSwapService_Execute_QuoteFails(t *testing.T) {
	sw := &fakeAMMSwapper{quoteErr: errors.New("no liquidity")}
	svc := NewSwapService(sw, SwapGates{})
	_, err := svc.Execute(context.Background(), baseReq())
	var se *domain.SwapExecError
	if !errors.As(err, &se) || se.Code != domain.ErrCodeInsufficientPoolLiquidity {
		t.Fatalf("expected INSUFFICIENT_POOL_LIQUIDITY, got %v", err)
	}
}

func TestSwapService_Execute_EmptyQuote(t *testing.T) {
	sw := &fakeAMMSwapper{quoteInput: ""}
	svc := NewSwapService(sw, SwapGates{})
	_, err := svc.Execute(context.Background(), baseReq())
	var se *domain.SwapExecError
	if !errors.As(err, &se) || se.Code != domain.ErrCodeInsufficientPoolLiquidity {
		t.Fatalf("expected INSUFFICIENT_POOL_LIQUIDITY for empty quote, got %v", err)
	}
}

func TestSwapService_Execute_SlippageExceeded(t *testing.T) {
	sw := &fakeAMMSwapper{quoteInput: "2000", orderID: "o", txHash: "h", amountIn: "2000"}
	svc := NewSwapService(sw, SwapGates{})
	req := baseReq()
	req.MaxAmountIn = "1000" // quote 2000 > cap 1000
	_, err := svc.Execute(context.Background(), req)
	var se *domain.SwapExecError
	if !errors.As(err, &se) || se.Code != domain.ErrCodeSlippageLimitExceeded {
		t.Fatalf("expected SLIPPAGE_LIMIT_EXCEEDED, got %v", err)
	}
	if sw.swapCalled {
		t.Fatal("swap should not be submitted when slippage exceeds cap")
	}
}

func TestSwapService_Execute_SlippageBadFormat(t *testing.T) {
	sw := &fakeAMMSwapper{quoteInput: "abc", orderID: "o", txHash: "h", amountIn: "1"}
	svc := NewSwapService(sw, SwapGates{})
	req := baseReq()
	req.MaxAmountIn = "1000"
	_, err := svc.Execute(context.Background(), req)
	var se *domain.SwapExecError
	if !errors.As(err, &se) || se.Code != domain.ErrCodeSlippageLimitExceeded {
		t.Fatalf("expected slippage error for bad format, got %v", err)
	}
}

func TestSwapService_Execute_SubmitFails(t *testing.T) {
	sw := &fakeAMMSwapper{quoteInput: "950", swapErr: errors.New("revert")}
	svc := NewSwapService(sw, SwapGates{})
	if _, err := svc.Execute(context.Background(), baseReq()); err == nil {
		t.Fatal("expected submit error")
	}
}

func TestSwapService_Execute_FeeDistribution(t *testing.T) {
	sw := &fakeAMMSwapper{quoteInput: "950", orderID: "ord-1", txHash: "0xabc", amountIn: "10000"}
	rec := &fakeFeeRecorder{}
	svc := NewSwapService(sw, SwapGates{}).
		WithFeeReader(fakeFeeReader{feeBps: 30}).
		WithFeeRecorder(rec)

	if _, err := svc.Execute(context.Background(), baseReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rec.called {
		t.Fatal("expected fee recorder to be called")
	}
	// fee = 10000 * 30 / 10000 = 30
	if rec.feeA.Cmp(big.NewInt(30)) != 0 {
		t.Fatalf("expected fee of 30, got %s", rec.feeA)
	}
}

func TestSwapService_Execute_FeeDistributionFailureNonBlocking(t *testing.T) {
	sw := &fakeAMMSwapper{quoteInput: "950", orderID: "ord-1", txHash: "0xabc", amountIn: "10000"}
	rec := &fakeFeeRecorder{err: errors.New("fee db down")}
	svc := NewSwapService(sw, SwapGates{}).
		WithFeeReader(fakeFeeReader{feeBps: 30}).
		WithFeeRecorder(rec)

	// Fee failure must not fail the swap.
	if _, err := svc.Execute(context.Background(), baseReq()); err != nil {
		t.Fatalf("fee distribution failure should be non-blocking, got: %v", err)
	}
}

func TestSwapService_Execute_FeeReaderZeroFeeSkipsRecorder(t *testing.T) {
	sw := okSwapper()
	rec := &fakeFeeRecorder{}
	svc := NewSwapService(sw, SwapGates{}).
		WithFeeReader(fakeFeeReader{feeBps: 0}).
		WithFeeRecorder(rec)
	if _, err := svc.Execute(context.Background(), baseReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.called {
		t.Fatal("recorder should not be called when fee bps is zero")
	}
}
