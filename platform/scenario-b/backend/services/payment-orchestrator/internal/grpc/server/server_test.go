// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// noopRelay is a test double that satisfies InteroperabilityPort with no-op
// behaviour. All subscription, relay, and verify calls succeed silently.
type noopRelay struct{}

func (noopRelay) SubscribeLockEvents(_ context.Context, _ func(ports.InteroperabilityProof) error) error {
	return nil
}
func (noopRelay) SubscribeSettleEvents(_ context.Context, _ func(ports.InteroperabilityProof) error) error {
	return nil
}
func (noopRelay) RelayProof(_ context.Context, _ ports.InteroperabilityProof) (string, error) {
	return "", nil
}
func (noopRelay) VerifyProof(_ context.Context, _ ports.InteroperabilityProof) (bool, error) {
	return true, nil
}

// mockToken is a test double for TCeBMPort.
type mockToken struct {
	balance   string
	err       error
	symbol    string
	symbolErr error
	mintCalls int
	burnCalls int
}

func (m *mockToken) Mint(_ context.Context, _, _ string) (string, error) {
	m.mintCalls++
	return "mock-mint-tx", nil
}
func (m *mockToken) Burn(_ context.Context, _, _ string) (string, error) {
	m.burnCalls++
	return "mock-burn-tx", nil
}
func (m *mockToken) BalanceOf(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.balance == "" {
		return "0", nil
	}
	return m.balance, nil
}
func (m *mockToken) GetBalance(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.balance == "" {
		return "0", nil
	}
	return m.balance, nil
}
func (m *mockToken) Decimals(_ context.Context) (uint8, error) { return 18, nil }
func (m *mockToken) Symbol(_ context.Context) (string, error) {
	if m.symbolErr != nil {
		return "", m.symbolErr
	}
	if m.symbol == "" {
		return "tCeBM_BRL", nil
	}
	return m.symbol, nil
}

// mockFiat is a test double for FiatTokenPort.
type mockFiat struct {
	balanceCalled int
	balance       string
	err           error
	symbol        string
	symbolErr     error
	mintCalls     int
	burnCalls     int
}

func (m *mockFiat) Mint(_ context.Context, _, _ string) (string, error) {
	m.mintCalls++
	return "mock-fiat-mint-tx", nil
}

func (m *mockFiat) Burn(_ context.Context, _, _ string) (string, error) {
	m.burnCalls++
	return "mock-fiat-burn-tx", nil
}

func (m *mockFiat) BalanceOf(_ context.Context, _ string) (string, error) {
	return m.balance, m.err
}

func (m *mockFiat) GetFiatBalance(_ context.Context) (string, error) {
	m.balanceCalled++
	if m.err != nil {
		return "", m.err
	}
	if m.balance == "" {
		return "0", nil
	}
	return m.balance, nil
}
func (m *mockFiat) Decimals(_ context.Context) (uint8, error) { return 18, nil }
func (m *mockFiat) Symbol(_ context.Context) (string, error) {
	if m.symbolErr != nil {
		return "", m.symbolErr
	}
	if m.symbol == "" {
		return "fCeBM_BRL", nil
	}
	return m.symbol, nil
}

type testEnv struct {
	client pb.PaymentOrchestratorServiceClient
	fiat   *mockFiat
	cancel context.CancelFunc
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	return setupTestEnvFull(t, nil, nil)
}

func setupTestEnvWithFiat(t *testing.T, fiat *mockFiat) *testEnv {
	t.Helper()
	return setupTestEnvFull(t, fiat, nil)
}

func setupTestEnvWithToken(t *testing.T, token *mockToken) *testEnv {
	t.Helper()
	return setupTestEnvFull(t, nil, token)
}

func setupTestEnvFull(t *testing.T, fiat *mockFiat, token *mockToken) *testEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	var fiatPort ports.FiatTokenPort
	if fiat != nil {
		fiatPort = fiat
	}
	if token == nil {
		token = &mockToken{balance: "1000000"}
	}

	grpcServer, newErr := server.New(server.Config{
		Token:  token,
		Relay:  noopRelay{},
		Fiat:   fiatPort,
		Logger: logger,
	})
	if newErr != nil {
		t.Fatalf("server.New: %v", newErr)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() { _ = grpcServer.Serve(lis) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//nolint:staticcheck
	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cancel()
		grpcServer.Stop()
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
		grpcServer.Stop()
	})

	return &testEnv{
		client: pb.NewPaymentOrchestratorServiceClient(conn),
		fiat:   fiat,
		cancel: cancel,
	}
}

func TestGetBalance_Success(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	resp, err := env.client.GetBalance(ctx, &pb.GetBalanceRequest{})
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if resp.Balance != "1000000" {
		t.Errorf("expected 1000000, got %s", resp.Balance)
	}
	if resp.Decimals != 18 {
		t.Errorf("expected decimals 18, got %d", resp.Decimals)
	}
	// Symbol is read from the token contract and is the source of truth for the UI currency code.
	if resp.Symbol != "tCeBM_BRL" {
		t.Errorf("expected symbol tCeBM_BRL, got %q", resp.Symbol)
	}
}

// A failed symbol read must not fail the whole balance call: the symbol is best-effort,
// and clients fall back to their configured fiat symbol when it is empty.
func TestGetBalance_SymbolReadFailureIsNonFatal(t *testing.T) {
	env := setupTestEnvWithToken(t, &mockToken{balance: "1000000", symbolErr: errors.New("call symbol: revert")})
	ctx := context.Background()

	resp, err := env.client.GetBalance(ctx, &pb.GetBalanceRequest{})
	if err != nil {
		t.Fatalf("GetBalance should succeed despite symbol read failure: %v", err)
	}
	if resp.Balance != "1000000" {
		t.Errorf("expected 1000000, got %s", resp.Balance)
	}
	if resp.Symbol != "" {
		t.Errorf("expected empty symbol on read failure, got %q", resp.Symbol)
	}
}

func TestGetFiatBalance_Success(t *testing.T) {
	env := setupTestEnvWithFiat(t, &mockFiat{balance: "1250000"})
	ctx := context.Background()

	resp, err := env.client.GetFiatBalance(ctx, &pb.GetFiatBalanceRequest{})
	if err != nil {
		t.Fatalf("GetFiatBalance: %v", err)
	}
	if resp.Balance != "1250000" {
		t.Errorf("expected 1250000, got %s", resp.Balance)
	}
	if env.fiat.balanceCalled != 1 {
		t.Errorf("expected fiat.GetFiatBalance called 1 time, got %d", env.fiat.balanceCalled)
	}
	if resp.Decimals != 18 {
		t.Errorf("expected decimals 18, got %d", resp.Decimals)
	}
	// Symbol is read from the fiat token contract and is the source of truth for the UI currency code.
	if resp.Symbol != "fCeBM_BRL" {
		t.Errorf("expected symbol fCeBM_BRL, got %q", resp.Symbol)
	}
}

// A failed fiat symbol read must not fail the balance call: symbol is best-effort.
func TestGetFiatBalance_SymbolReadFailureIsNonFatal(t *testing.T) {
	env := setupTestEnvWithFiat(t, &mockFiat{balance: "1250000", symbolErr: errors.New("call symbol: revert")})
	ctx := context.Background()

	resp, err := env.client.GetFiatBalance(ctx, &pb.GetFiatBalanceRequest{})
	if err != nil {
		t.Fatalf("GetFiatBalance should succeed despite symbol read failure: %v", err)
	}
	if resp.Balance != "1250000" {
		t.Errorf("expected 1250000, got %s", resp.Balance)
	}
	if resp.Symbol != "" {
		t.Errorf("expected empty symbol on read failure, got %q", resp.Symbol)
	}
}

func TestGetFiatBalance_FiatNil(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.client.GetFiatBalance(ctx, &pb.GetFiatBalanceRequest{})
	if err == nil {
		t.Fatal("expected error when fiat adapter is nil")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected codes.Unavailable, got %s", status.Code(err))
	}
}
