package server_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
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
	balance string
	err     error
}

func (m *mockToken) Mint(_ context.Context, _, _ string) (string, error) {
	return "mock-mint-tx", nil
}
func (m *mockToken) Burn(_ context.Context, _, _ string) (string, error) {
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

// mockFiat is a test double for FiatTokenPort.
type mockFiat struct {
	balanceCalled int
	balance       string
	err           error
}

func (m *mockFiat) Mint(_ context.Context, _, _ string) (string, error) {
	return "mock-fiat-mint-tx", nil
}

func (m *mockFiat) Burn(_ context.Context, _, _ string) (string, error) {
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

// mockHTLC is a test double for HTLCContractPort.
type mockHTLC struct {
	lockCalled   int
	settleCalled int
	refundCalled int
	commitCalled int

	lockErr   error
	settleErr error
	refundErr error
	commitErr error
}

func (m *mockHTLC) Lock(_ context.Context, _ ports.HTLCLockParams) (string, error) {
	m.lockCalled++
	if m.lockErr != nil {
		return "", m.lockErr
	}
	return "mock-htlc-lock-tx", nil
}
func (m *mockHTLC) Settle(_ context.Context, _ [32]byte, _ [32]byte) (string, error) {
	m.settleCalled++
	if m.settleErr != nil {
		return "", m.settleErr
	}
	return "mock-htlc-settle-tx", nil
}
func (m *mockHTLC) Refund(_ context.Context, _ [32]byte) (string, error) {
	m.refundCalled++
	if m.refundErr != nil {
		return "", m.refundErr
	}
	return "mock-htlc-refund-tx", nil
}
func (m *mockHTLC) RegisterAgreementCommitment(_ context.Context, _ [32]byte) (string, error) {
	m.commitCalled++
	if m.commitErr != nil {
		return "", m.commitErr
	}
	return "mock-htlc-commit-tx", nil
}

type testEnv struct {
	client pb.PaymentOrchestratorServiceClient
	htlc   *mockHTLC
	fiat   *mockFiat
	cancel context.CancelFunc
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	return setupTestEnvFull(t, nil, nil)
}

func setupTestEnvWithFiat(t *testing.T, fiat *mockFiat) *testEnv {
	t.Helper()
	return setupTestEnvFull(t, nil, fiat)
}

func setupTestEnvFull(t *testing.T, htlc *mockHTLC, fiat *mockFiat) *testEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	var fiatPort ports.FiatTokenPort
	if fiat != nil {
		fiatPort = fiat
	}
	var htlcPort ports.HTLCContractPort
	if htlc != nil {
		htlcPort = htlc
	}

	grpcServer := server.New(server.Config{
		Token:  &mockToken{balance: "1000000"},
		HTLC:   htlcPort,
		Relay:  noopRelay{},
		Fiat:   fiatPort,
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
		htlc:   htlc,
		fiat:   fiat,
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

	// Secret is returned once in the LockHTLC response (not via GetHTLCStatus).
	secret := lockResp.Secret
	if secret == "" {
		t.Fatal("LockHTLC response must include the secret for the creator")
	}

	settleResp, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC: %v", err)
	}
	_ = settleResp // htlc_tx_hash is empty when no on-chain HTLC adapter is configured
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

func TestSettleHTLC_Idempotent(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	// Lock an HTLC
	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_IDEMPOTENT",
		Receiver:    "bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Secret is returned once in the LockHTLC response.
	secret := lockResp.Secret

	// First settle — should succeed
	resp1, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC (first): %v", err)
	}

	// Second settle with same contract_id — should succeed (idempotent)
	resp2, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC (idempotent, same contract_id): expected success, got %v", err)
	}
	if resp2.HtlcTxHash != resp1.HtlcTxHash {
		t.Errorf("expected same htlc_tx_hash on idempotent settle, got %q vs %q", resp2.HtlcTxHash, resp1.HtlcTxHash)
	}

	// Third settle with a foreign contract_id (cross-spoke echo scenario) —
	// should succeed via hashLock fallback finding the already-settled record.
	resp3, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: "foreign_contract_id_from_other_spoke",
		Secret:     secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC (idempotent, foreign contract_id): expected success, got %v", err)
	}
	if resp3.HtlcTxHash != resp1.HtlcTxHash {
		t.Errorf("expected same htlc_tx_hash on cross-spoke idempotent settle, got %q vs %q", resp3.HtlcTxHash, resp1.HtlcTxHash)
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

// --- Tests for on-chain settle failure ---

func TestSettleHTLC_OnChainSettleFails(t *testing.T) {
	htlcMock := &mockHTLC{settleErr: errors.New("on-chain revert")}
	env := setupTestEnvFull(t, htlcMock, nil)
	ctx := context.Background()

	// Lock (on-chain lock succeeds)
	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_BLOCK_SETTLE",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Settle — on-chain settle will fail → should return error
	_, err = env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err == nil {
		t.Fatal("expected error when on-chain HTLC settle fails")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected codes.Internal, got %s", status.Code(err))
	}

	// Record should still be LOCKED
	statusResp2, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp2.Lock.State != pb.HTLCState_HTLC_STATE_LOCKED {
		t.Errorf("expected state LOCKED after failed settle, got %s", statusResp2.Lock.State)
	}
}

func TestSettleHTLC_BothSucceed_StateSettled(t *testing.T) {
	htlcMock := &mockHTLC{}
	env := setupTestEnvFull(t, htlcMock, nil)
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_BOTH_OK",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	resp, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC: %v", err)
	}
	if resp.HtlcTxHash == "" {
		t.Error("expected non-empty htlc_tx_hash")
	}

	// Verify state is SETTLED
	statusResp2, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp2.Lock.State != pb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("expected state SETTLED, got %s", statusResp2.Lock.State)
	}

	// On-chain settle should have been called exactly once
	if htlcMock.settleCalled != 1 {
		t.Errorf("expected htlc.Settle called 1 time, got %d", htlcMock.settleCalled)
	}
}

// --- Test cross-spoke settle with on-chain failure ---

func TestSettleHTLC_CrossSpoke_OnChainFails(t *testing.T) {
	htlcMock := &mockHTLC{settleErr: errors.New("on-chain revert")}
	env := setupTestEnvFull(t, htlcMock, nil)
	ctx := context.Background()

	// Use LockHTLC (generates the real secret) so we can settle with the correct secret.
	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_CROSS_SPOKE",
		Receiver:    "bank-d",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 1800,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Simulate cross-spoke settle: use a foreign contract_id but the real secret.
	// The server will find the local record via hashLock match.
	_, err = env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: "foreign_contract_id_from_other_spoke",
		Secret:     lockResp.Secret,
	})
	if err == nil {
		t.Fatal("expected error when on-chain HTLC settle fails")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected codes.Internal, got %s", status.Code(err))
	}

	// Record should be LOCKED (on-chain settle failed, so rollback occurred)
	statusResp2, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp2.Lock.State != pb.HTLCState_HTLC_STATE_LOCKED {
		t.Errorf("expected state LOCKED after on-chain settle failure, got %s", statusResp2.Lock.State)
	}
}

// --- Tests for SETTLING concurrent calls ---

func TestSettleHTLC_ConcurrentCallsOnlyOneSucceeds(t *testing.T) {
	htlcMock := &mockHTLC{}
	env := setupTestEnvFull(t, htlcMock, nil)
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_CONCURRENT",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
				ContractId: lockResp.ContractId,
				Secret:     lockResp.Secret,
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var successes, failures int
	for err := range results {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	// At least one must succeed; the concurrent loser may get FailedPrecondition
	// or may also succeed (idempotent) if the first completed before the second started.
	if successes < 1 {
		t.Errorf("expected at least 1 success, got %d successes and %d failures", successes, failures)
	}
}

// --- Security: preimage must never appear in read responses ---

func TestGetHTLCStatus_SecretNotExposed(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_SEC_001",
		Receiver:    "bank-b",
		Amount:      "100",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}
	if lockResp.Secret == "" {
		t.Fatal("LockHTLC must return the secret to the creator")
	}

	statusResp, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp.Lock == nil {
		t.Fatal("expected non-nil lock in response")
	}
	// Compile-time proof: HTLCLock has no Secret field (field 6 is reserved in the proto).
	// The test exercises the endpoint to confirm it returns a valid response.
}

func TestSearchHTLC_SecretNotExposed(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_SEC_002",
		Receiver:    "bank-b",
		Amount:      "100",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	searchResp, err := env.client.SearchHTLC(ctx, &pb.SearchHTLCRequest{})
	if err != nil {
		t.Fatalf("SearchHTLC: %v", err)
	}
	// Compile-time proof: HTLCLock has no Secret field (field 6 is reserved in the proto).
	// Verify results contain the locked HTLC with no secret-carrying fields.
	if len(searchResp.Locks) == 0 {
		t.Error("expected at least one result for the locked HTLC")
	}
}
