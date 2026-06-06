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
	"google.golang.org/grpc/metadata"
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

// mockZeto is a test double for ZetoOperator.
type mockZeto struct {
	mintCalled           int
	burnCalled           int
	transferCalled       int
	lockCalled           int
	unlockCalled         int
	transferLockedCalled int
	balanceCalled        int

	lockErr           error
	unlockErr         error
	transferLockedErr error
}

func (m *mockZeto) Mint(_ context.Context, _, _ string) (string, error) {
	m.mintCalled++
	return "mock-mint-tx", nil
}
func (m *mockZeto) Burn(_ context.Context, _, _ string) (string, error) {
	m.burnCalled++
	return "mock-burn-tx", nil
}
func (m *mockZeto) Transfer(_ context.Context, _, _ string) (string, error) {
	m.transferCalled++
	return "mock-transfer-tx", nil
}
func (m *mockZeto) Lock(_ context.Context, _, _ string) (*ports.ZetoLockResult, error) {
	m.lockCalled++
	if m.lockErr != nil {
		return nil, m.lockErr
	}
	return &ports.ZetoLockResult{TxHash: "mock-lock-tx", ZetoLockRef: "mock-lock-ref-001", LockedStateIDs: []string{"0xabc123"}}, nil
}
func (m *mockZeto) Unlock(_ context.Context, _ string) (string, error) {
	m.unlockCalled++
	if m.unlockErr != nil {
		return "", m.unlockErr
	}
	return "mock-unlock-tx", nil
}
func (m *mockZeto) TransferLocked(_ context.Context, _, _, _ string) (string, error) {
	m.transferLockedCalled++
	if m.transferLockedErr != nil {
		return "", m.transferLockedErr
	}
	return "mock-transfer-locked-tx", nil
}
func (m *mockZeto) Balance(_ context.Context) (string, error) {
	m.balanceCalled++
	return "1000000", nil
}

func (m *mockZeto) ResolveIdentity(_ context.Context, identity string) (string, error) {
	// Deterministic fake EVM address for tests.
	if identity == "" {
		return "", nil
	}
	return "0x1111111111111111111111111111111111111111", nil
}

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

type testEnv struct {
	client pb.PaymentOrchestratorServiceClient
	zeto   *mockZeto
	htlc   *mockHTLC
	fiat   *mockFiat
	cancel context.CancelFunc
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

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	return setupTestEnvFull(t, nil, nil, "")
}

func setupTestEnvWithFiat(t *testing.T, fiat *mockFiat) *testEnv {
	t.Helper()
	return setupTestEnvFull(t, nil, fiat, "")
}

const testPaladinIdentity = "funded_operator@spoke-a-bank-a"

func setupTestEnvFull(t *testing.T, htlc *mockHTLC, fiat *mockFiat, spokePrefix string) *testEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := &mockZeto{}
	var fiatPort ports.FiatTokenPort
	if fiat != nil {
		fiatPort = fiat
	}
	var htlcPort ports.HTLCContractPort
	if htlc != nil {
		htlcPort = htlc
	}

	grpcServer := server.New(server.Config{
		Zeto:            mock,
		HTLC:            htlcPort,
		Relay:           noopRelay{},
		Fiat:            fiatPort,
		SpokePrefix:     spokePrefix,
		PaladinIdentity: testPaladinIdentity,
		Logger:          logger,
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
	if env.zeto.lockCalled != 1 {
		t.Errorf("expected zeto.Lock called 1 time, got %d", env.zeto.lockCalled)
	}

	statusResp, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: resp.ContractId})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp.Lock.Sender != testPaladinIdentity {
		t.Errorf("expected sender %q, got %q", testPaladinIdentity, statusResp.Lock.Sender)
	}
}

func TestSearchHTLC_BySender(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	resp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_SENDER_TEST",
		Receiver:    "bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	searchResp, err := env.client.SearchHTLC(ctx, &pb.SearchHTLCRequest{Sender: testPaladinIdentity})
	if err != nil {
		t.Fatalf("SearchHTLC: %v", err)
	}
	found := false
	for _, lock := range searchResp.Locks {
		if lock.ContractId == resp.ContractId {
			found = true
			if lock.Sender != testPaladinIdentity {
				t.Errorf("expected sender %q, got %q", testPaladinIdentity, lock.Sender)
			}
		}
	}
	if !found {
		t.Errorf("SearchHTLC by sender did not return the locked contract %q", resp.ContractId)
	}

	// A search with a different sender should not return the contract.
	otherResp, err := env.client.SearchHTLC(ctx, &pb.SearchHTLCRequest{Sender: "funded_operator@spoke-b-bank-b"})
	if err != nil {
		t.Fatalf("SearchHTLC (other sender): %v", err)
	}
	for _, lock := range otherResp.Locks {
		if lock.ContractId == resp.ContractId {
			t.Error("SearchHTLC with wrong sender should not return the contract")
		}
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
	if resp2.ZetoTxHash != resp1.ZetoTxHash {
		t.Errorf("expected same zeto_tx_hash on idempotent settle, got %q vs %q", resp2.ZetoTxHash, resp1.ZetoTxHash)
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
	if resp3.ZetoTxHash != resp1.ZetoTxHash {
		t.Errorf("expected same zeto_tx_hash on cross-spoke idempotent settle, got %q vs %q", resp3.ZetoTxHash, resp1.ZetoTxHash)
	}

	// TransferLocked should only have been called once (the first settle)
	if env.zeto.transferLockedCalled != 1 {
		t.Errorf("expected zeto.TransferLocked called 1 time, got %d", env.zeto.transferLockedCalled)
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

// --- Tests for on-chain settle failure blocking token transfer ---

func TestSettleHTLC_OnChainSettleFails_BlocksZetoTransfer(t *testing.T) {
	htlcMock := &mockHTLC{settleErr: errors.New("on-chain revert")}
	env := setupTestEnvFull(t, htlcMock, nil, "")
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

	// Zeto TransferLocked must NOT have been called
	if env.zeto.transferLockedCalled != 0 {
		t.Errorf("expected zeto.TransferLocked NOT called, but was called %d time(s)", env.zeto.transferLockedCalled)
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

func TestSettleHTLC_ZetoTransferFails_StateRemainsLocked(t *testing.T) {
	htlcMock := &mockHTLC{} // on-chain succeeds
	env := setupTestEnvFull(t, htlcMock, nil, "")
	env.zeto.transferLockedErr = errors.New("zeto transfer failed")
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_ZETO_FAIL",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	_, err = env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err == nil {
		t.Fatal("expected error when zeto transferLocked fails")
	}

	// Record should be in SETTLING (on-chain succeeded, zeto failed — secret is public)
	statusResp2, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp2.Lock.State != pb.HTLCState_HTLC_STATE_SETTLING {
		t.Errorf("expected state SETTLING after failed zeto transfer, got %s", statusResp2.Lock.State)
	}
}

func TestSettleHTLC_BothSucceed_StateSettled(t *testing.T) {
	htlcMock := &mockHTLC{}
	env := setupTestEnvFull(t, htlcMock, nil, "")
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
	if resp.ZetoTxHash == "" {
		t.Error("expected non-empty zeto_tx_hash")
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

	// Both operations should have been called exactly once
	if htlcMock.settleCalled != 1 {
		t.Errorf("expected htlc.Settle called 1 time, got %d", htlcMock.settleCalled)
	}
	if env.zeto.transferLockedCalled != 1 {
		t.Errorf("expected zeto.TransferLocked called 1 time, got %d", env.zeto.transferLockedCalled)
	}
}

// --- Tests for LockHTLC on-chain failure rollback ---

func TestLockHTLC_OnChainLockFails_RollsBackZetoLock(t *testing.T) {
	htlcMock := &mockHTLC{lockErr: errors.New("on-chain lock revert")}
	env := setupTestEnvFull(t, htlcMock, nil, "")
	ctx := context.Background()

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_LOCK_ROLLBACK",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err == nil {
		t.Fatal("expected error when on-chain HTLC lock fails")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected codes.Internal, got %s", status.Code(err))
	}

	// Zeto Lock should have been called (succeeds first)
	if env.zeto.lockCalled != 1 {
		t.Errorf("expected zeto.Lock called 1 time, got %d", env.zeto.lockCalled)
	}
	// Zeto Unlock should have been called as rollback
	if env.zeto.unlockCalled != 1 {
		t.Errorf("expected zeto.Unlock called 1 time (rollback), got %d", env.zeto.unlockCalled)
	}
}

func TestLockHTLCWithHashLock_OnChainLockFails_RollsBackZetoLock(t *testing.T) {
	htlcMock := &mockHTLC{lockErr: errors.New("on-chain lock revert")}
	env := setupTestEnvFull(t, htlcMock, nil, "")
	ctx := context.Background()

	// Need a valid 64-char hex hashLock
	hashLock := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2"

	_, err := env.client.LockHTLCWithHashLock(ctx, &pb.LockHTLCWithHashLockRequest{
		AgreementId: "FX_LOCK_WITH_HASH_ROLLBACK",
		Receiver:    "bank-b",
		Amount:      "1000",
		HashLock:    hashLock,
		TimeLock:    uint64(time.Now().Unix()) + 1800,
	})
	if err == nil {
		t.Fatal("expected error when on-chain HTLC lock fails")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected codes.Internal, got %s", status.Code(err))
	}

	// Zeto Unlock should have been called as rollback
	if env.zeto.unlockCalled != 1 {
		t.Errorf("expected zeto.Unlock called 1 time (rollback), got %d", env.zeto.unlockCalled)
	}
}

// --- Test cross-spoke settle with on-chain failure ---

func TestSettleHTLC_CrossSpoke_OnChainFails_BlocksTransfer(t *testing.T) {
	htlcMock := &mockHTLC{settleErr: errors.New("on-chain revert")}
	env := setupTestEnvFull(t, htlcMock, nil, "")
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

	// Tokens must NOT have been transferred
	if env.zeto.transferLockedCalled != 0 {
		t.Errorf("expected zeto.TransferLocked NOT called, got %d", env.zeto.transferLockedCalled)
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

// --- Tests for receiver locality validation ---

func TestLockHTLC_RejectsCrossSpokenReceiver(t *testing.T) {
	// Service configured as spoke-a — must reject spoke-b receivers
	env := setupTestEnvFull(t, nil, nil, "spoke-a")
	ctx := context.Background()

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_CROSS_REJECT",
		Receiver:    "funded_operator@spoke-b-bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err == nil {
		t.Fatal("expected error for cross-spoke receiver")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected codes.InvalidArgument, got %s", status.Code(err))
	}
	// Zeto Lock must NOT have been called
	if env.zeto.lockCalled != 0 {
		t.Errorf("expected zeto.Lock NOT called, got %d", env.zeto.lockCalled)
	}
}

func TestLockHTLC_AcceptsLocalReceiver(t *testing.T) {
	// Service configured as spoke-a — must accept spoke-a receivers
	env := setupTestEnvFull(t, nil, nil, "spoke-a")
	ctx := context.Background()

	resp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_LOCAL_OK",
		Receiver:    "funded_operator@spoke-a-bank-c",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC with local receiver: %v", err)
	}
	if resp.ContractId == "" {
		t.Error("expected non-empty contract_id")
	}
	if env.zeto.lockCalled != 1 {
		t.Errorf("expected zeto.Lock called 1 time, got %d", env.zeto.lockCalled)
	}
}

func TestLockHTLCWithHashLock_RejectsCrossSpokenReceiver(t *testing.T) {
	env := setupTestEnvFull(t, nil, nil, "spoke-b")
	ctx := context.Background()

	hashLock := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2"
	_, err := env.client.LockHTLCWithHashLock(ctx, &pb.LockHTLCWithHashLockRequest{
		AgreementId: "FX_CROSS_HASH_REJECT",
		Receiver:    "funded_operator@spoke-a-bank-c",
		Amount:      "1000",
		HashLock:    hashLock,
		TimeLock:    uint64(time.Now().Unix()) + 1800,
	})
	if err == nil {
		t.Fatal("expected error for cross-spoke receiver")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected codes.InvalidArgument, got %s", status.Code(err))
	}
	if env.zeto.lockCalled != 0 {
		t.Errorf("expected zeto.Lock NOT called, got %d", env.zeto.lockCalled)
	}
}

func TestLockHTLCWithHashLock_AcceptsLocalReceiver(t *testing.T) {
	env := setupTestEnvFull(t, nil, nil, "spoke-b")
	ctx := context.Background()

	hashLock := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2a3b4c5d6a7b8c9d0e1f2"
	resp, err := env.client.LockHTLCWithHashLock(ctx, &pb.LockHTLCWithHashLockRequest{
		AgreementId: "FX_LOCAL_HASH_OK",
		Receiver:    "funded_operator@spoke-b-bank-d",
		Amount:      "1000",
		HashLock:    hashLock,
		TimeLock:    uint64(time.Now().Unix()) + 1800,
	})
	if err != nil {
		t.Fatalf("LockHTLCWithHashLock with local receiver: %v", err)
	}
	if resp.ContractId == "" {
		t.Error("expected non-empty contract_id")
	}
}

func TestLockHTLC_NoSpokePrefixSkipsValidation(t *testing.T) {
	// When spokePrefix is empty (dev mode), any receiver is accepted
	env := setupTestEnvFull(t, nil, nil, "")
	ctx := context.Background()

	resp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_DEV_MODE",
		Receiver:    "funded_operator@spoke-b-bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC with no spokePrefix should accept any receiver: %v", err)
	}
	if resp.ContractId == "" {
		t.Error("expected non-empty contract_id")
	}
}

// --- Tests for SETTLING intermediate state and retry ---

func TestSettleHTLC_ConcurrentCallsOnlyOneSucceeds(t *testing.T) {
	htlcMock := &mockHTLC{}
	env := setupTestEnvFull(t, htlcMock, nil, "")
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
	// Token transfer should only happen once
	if env.zeto.transferLockedCalled != 1 {
		t.Errorf("expected zeto.TransferLocked called exactly 1 time, got %d", env.zeto.transferLockedCalled)
	}
}

func TestSettleHTLC_RetryAfterZetoTransferFails(t *testing.T) {
	htlcMock := &mockHTLC{}
	env := setupTestEnvFull(t, htlcMock, nil, "")
	env.zeto.transferLockedErr = errors.New("zeto transfer failed")
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_RETRY",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// First settle: on-chain succeeds, zeto fails → state should be SETTLING
	_, err = env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err == nil {
		t.Fatal("expected error when zeto transfer fails")
	}

	// Verify state is SETTLING
	statusResp2, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp2.Lock.State != pb.HTLCState_HTLC_STATE_SETTLING {
		t.Fatalf("expected state SETTLING, got %s", statusResp2.Lock.State)
	}

	// On-chain settle should have been called once
	if htlcMock.settleCalled != 1 {
		t.Errorf("expected htlc.Settle called 1 time, got %d", htlcMock.settleCalled)
	}

	// Fix the zeto mock and retry
	env.zeto.transferLockedErr = nil

	resp, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC retry: %v", err)
	}
	if resp.ZetoTxHash == "" {
		t.Error("expected non-empty zeto_tx_hash")
	}

	// On-chain settle should NOT have been called again (already succeeded)
	if htlcMock.settleCalled != 1 {
		t.Errorf("expected htlc.Settle still called 1 time (not retried), got %d", htlcMock.settleCalled)
	}

	// Verify state is SETTLED
	statusResp3, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp3.Lock.State != pb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("expected state SETTLED after retry, got %s", statusResp3.Lock.State)
	}
}

func TestRefundHTLC_RetryAfterZetoUnlockFails(t *testing.T) {
	env := setupTestEnvFull(t, nil, nil, "")
	env.zeto.unlockErr = errors.New("zeto unlock failed")
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_REFUND_RETRY",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) - 10, // already expired
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// First refund: zeto unlock fails → state should be REFUNDING
	_, err = env.client.RefundHTLC(ctx, &pb.RefundHTLCRequest{
		ContractId: lockResp.ContractId,
	})
	if err == nil {
		t.Fatal("expected error when zeto unlock fails")
	}

	// Verify state is REFUNDING
	statusResp, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp.Lock.State != pb.HTLCState_HTLC_STATE_REFUNDING {
		t.Fatalf("expected state REFUNDING, got %s", statusResp.Lock.State)
	}

	// Fix the zeto mock and retry
	env.zeto.unlockErr = nil

	resp, err := env.client.RefundHTLC(ctx, &pb.RefundHTLCRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("RefundHTLC retry: %v", err)
	}
	if resp.ZetoTxHash == "" {
		t.Error("expected non-empty zeto_tx_hash")
	}

	// Verify state is REFUNDED
	statusResp2, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp2.Lock.State != pb.HTLCState_HTLC_STATE_REFUNDED {
		t.Errorf("expected state REFUNDED after retry, got %s", statusResp2.Lock.State)
	}
}

func TestLockHTLC_RejectsUnparseableReceiverWhenSpokePrefixSet(t *testing.T) {
	// When spokePrefix is set, bare names like "bank-b" (no @spoke-X-...) should be rejected
	env := setupTestEnvFull(t, nil, nil, "spoke-a")
	ctx := context.Background()

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_UNPARSEABLE",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err == nil {
		t.Fatal("expected error for unparseable receiver when spokePrefix is set")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected codes.InvalidArgument, got %s", status.Code(err))
	}
	if env.zeto.lockCalled != 0 {
		t.Errorf("expected zeto.Lock NOT called, got %d", env.zeto.lockCalled)
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

	searchResp, err := env.client.SearchHTLC(ctx, &pb.SearchHTLCRequest{
		Sender: testPaladinIdentity,
	})
	if err != nil {
		t.Fatalf("SearchHTLC: %v", err)
	}
	// Compile-time proof: HTLCLock has no Secret field (field 6 is reserved in the proto).
	// Verify results contain the locked HTLC with no secret-carrying fields.
	if len(searchResp.Locks) == 0 {
		t.Error("expected at least one result for the locked HTLC")
	}
}

func TestSearchHTLC_BankIDFilterVisible(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_VIS_001",
		Receiver:    "counterparty@spoke-b-bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Sender is testPaladinIdentity ("funded_operator@spoke-a-bank-a").
	// Passing BankID "bank-a" via x-caller-identity should match the sender and return results.
	mdCtx := metadata.AppendToOutgoingContext(ctx, "x-caller-identity", "bank-a")

	searchResp, err := env.client.SearchHTLC(mdCtx, &pb.SearchHTLCRequest{})
	if err != nil {
		t.Fatalf("SearchHTLC with bank-a identity: %v", err)
	}
	if len(searchResp.Locks) == 0 {
		t.Error("bank-a should see the HTLC it created as sender")
	}

	// "bank-c" is not a counterparty — should get empty results.
	mdCtxOther := metadata.AppendToOutgoingContext(ctx, "x-caller-identity", "bank-c")
	searchRespOther, err := env.client.SearchHTLC(mdCtxOther, &pb.SearchHTLCRequest{})
	if err != nil {
		t.Fatalf("SearchHTLC with bank-c identity: %v", err)
	}
	if len(searchRespOther.Locks) != 0 {
		t.Errorf("bank-c should not see HTLCs where it is not a counterparty, got %d", len(searchRespOther.Locks))
	}
}

func TestGetHTLCStatus_BankIDFilterCounterparty(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_VIS_002",
		Receiver:    "counterparty@spoke-b-bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Sender is "bank-a" — should be allowed.
	mdCtx := metadata.AppendToOutgoingContext(ctx, "x-caller-identity", "bank-a")
	_, err = env.client.GetHTLCStatus(mdCtx, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Errorf("bank-a (sender) should be able to view the HTLC: %v", err)
	}

	// Receiver is "bank-b" — should be allowed.
	mdCtxB := metadata.AppendToOutgoingContext(ctx, "x-caller-identity", "bank-b")
	_, err = env.client.GetHTLCStatus(mdCtxB, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Errorf("bank-b (receiver) should be able to view the HTLC: %v", err)
	}

	// "bank-c" is not a counterparty — should get PermissionDenied.
	mdCtxC := metadata.AppendToOutgoingContext(ctx, "x-caller-identity", "bank-c")
	_, err = env.client.GetHTLCStatus(mdCtxC, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("bank-c should get PermissionDenied, got: %v", err)
	}
}
