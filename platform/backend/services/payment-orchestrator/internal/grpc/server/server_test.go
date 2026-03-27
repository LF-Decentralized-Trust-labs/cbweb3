package server_test

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/adapters/cacti"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mockZeto is a test double for ZetoOperator.
type mockZeto struct {
	mintCalled           int
	transferCalled       int
	lockCalled           int
	unlockCalled         int
	transferLockedCalled int
	balanceCalled        int
}

func (m *mockZeto) Mint(_ context.Context, _, _ string) (string, error) {
	m.mintCalled++
	return "mock-mint-tx", nil
}
func (m *mockZeto) Transfer(_ context.Context, _, _ string) (string, error) {
	m.transferCalled++
	return "mock-transfer-tx", nil
}
func (m *mockZeto) Lock(_ context.Context, _, _ string) (*ports.ZetoLockResult, error) {
	m.lockCalled++
	return &ports.ZetoLockResult{TxHash: "mock-lock-tx", ZetoLockRef: "mock-lock-ref-001"}, nil
}
func (m *mockZeto) Unlock(_ context.Context, _ string) (string, error) {
	m.unlockCalled++
	return "mock-unlock-tx", nil
}
func (m *mockZeto) TransferLocked(_ context.Context, _, _, _ string) (string, error) {
	m.transferLockedCalled++
	return "mock-transfer-locked-tx", nil
}
func (m *mockZeto) Balance(_ context.Context, _ string) (string, error) {
	m.balanceCalled++
	return "1000000", nil
}

type testEnv struct {
	client pb.PaymentOrchestratorServiceClient
	zeto   *mockZeto
	relay  *cacti.StubRelay
	cancel context.CancelFunc
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := &mockZeto{}
	relay := cacti.NewStubRelay(logger)

	grpcServer := server.New(server.Config{
		Zeto:   mock,
		Relay:  relay,
		Logger: logger,
	})

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
		zeto:   mock,
		relay:  relay,
		cancel: cancel,
	}
}

func TestLockHTLC_Success(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	resp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_001",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}
	if resp.ContractId == "" {
		t.Error("expected non-empty contract_id")
	}
	if resp.HashLock == "" {
		t.Error("expected non-empty hash_lock")
	}
	if env.zeto.lockCalled != 1 {
		t.Errorf("expected zeto.Lock called 1 time, got %d", env.zeto.lockCalled)
	}
}

func TestLockHTLC_InvalidArgs(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{})
	if err == nil {
		t.Fatal("expected error for empty request")
	}
}

func TestSettleHTLC_Success(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_002",
		Receiver:    "bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Get secret from status (stored in off-chain record)
	statusResp, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	secret := statusResp.Lock.Secret

	settleResp, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC: %v", err)
	}
	if settleResp.ZetoTxHash == "" {
		t.Error("expected non-empty zeto_tx_hash")
	}
	if env.zeto.transferLockedCalled != 1 {
		t.Errorf("expected zeto.TransferLocked called 1 time, got %d", env.zeto.transferLockedCalled)
	}
}

func TestRefundHTLC_TimeLockNotExpired(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_003",
		Receiver:    "bank-b",
		Amount:      "200",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	_, err = env.client.RefundHTLC(ctx, &pb.RefundHTLCRequest{
		ContractId: lockResp.ContractId,
	})
	if err == nil {
		t.Fatal("expected error for non-expired timelock")
	}
}

func TestGetHTLCStatus_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent contract")
	}
}

func TestSearchHTLC_ByAgreement(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_SEARCH",
		Receiver:    "bank-c",
		Amount:      "300",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	searchResp, err := env.client.SearchHTLC(ctx, &pb.SearchHTLCRequest{
		AgreementId: "FX_SEARCH",
	})
	if err != nil {
		t.Fatalf("SearchHTLC: %v", err)
	}
	if len(searchResp.Locks) != 1 {
		t.Errorf("expected 1 result, got %d", len(searchResp.Locks))
	}
}

func TestMintToken_Success(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	resp, err := env.client.MintToken(ctx, &pb.MintTokenRequest{
		To:     "bank-a",
		Amount: "10000",
	})
	if err != nil {
		t.Fatalf("MintToken: %v", err)
	}
	if resp.TxHash != "mock-mint-tx" {
		t.Errorf("expected mock-mint-tx, got %s", resp.TxHash)
	}
	if env.zeto.mintCalled != 1 {
		t.Errorf("expected zeto.Mint called 1 time, got %d", env.zeto.mintCalled)
	}
}

func TestTransferToken_Success(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	resp, err := env.client.TransferToken(ctx, &pb.TransferTokenRequest{
		To:     "bank-b",
		Amount: "500",
	})
	if err != nil {
		t.Fatalf("TransferToken: %v", err)
	}
	if resp.TxHash != "mock-transfer-tx" {
		t.Errorf("expected mock-transfer-tx, got %s", resp.TxHash)
	}
}

func TestGetBalance_Success(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	resp, err := env.client.GetBalance(ctx, &pb.GetBalanceRequest{
		Identity: "funded_operator@spoke-a-cb",
	})
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if resp.Balance != "1000000" {
		t.Errorf("expected 1000000, got %s", resp.Balance)
	}
}
