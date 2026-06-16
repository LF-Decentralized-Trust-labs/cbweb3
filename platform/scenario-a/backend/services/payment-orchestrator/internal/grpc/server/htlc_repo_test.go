// Package server_test — HTLC repository integration tests.
//
// These tests verify:
//  1. loadHTLCsFromDB correctly pre-populates the in-memory HTLC cache from the
//     HTLCRepository at startup.
//  2. persistHTLC calls HTLCRepository.UpdateHTLC with the current in-memory
//     state snapshot.
//  3. When HTLCRepo is nil (the baseline used by the existing server tests),
//     the server operates identically to before — no panics, no regressions.
//  4. When HTLCRepo is wired, LockHTLC calls CreateHTLC and SettleHTLC calls
//     UpdateHTLC with the SETTLED state.
//
// A hand-written mockHTLCRepository is used; no external mock library is required.
package server_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ─── mockHTLCRepository ───────────────────────────────────────────────────────
// A hand-written in-memory HTLCRepository test double that records every call
// and allows tests to inject errors for specific operations.

type mockHTLCRepository struct {
	mu sync.RWMutex

	records map[string]*domain.HTLCRecord // keyed by ContractID
	byHash  map[string]*domain.HTLCRecord // keyed by HashLock

	createCalls int
	updateCalls int
	getCalls    int
	listCalls   int

	createErr    error
	updateErr    error
	getErr       error
	listNonTermErr error

	// captures the most recent UpdateHTLC call argument (shallow copy)
	lastUpdate *domain.HTLCRecord
}

func newMockHTLCRepository() *mockHTLCRepository {
	return &mockHTLCRepository{
		records: make(map[string]*domain.HTLCRecord),
		byHash:  make(map[string]*domain.HTLCRecord),
	}
}

func (m *mockHTLCRepository) CreateHTLC(_ context.Context, record *domain.HTLCRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	if m.createErr != nil {
		return m.createErr
	}
	cp := *record
	m.records[record.ContractID] = &cp
	m.byHash[record.HashLock] = &cp
	return nil
}

func (m *mockHTLCRepository) GetHTLC(_ context.Context, contractID string) (*domain.HTLCRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.getCalls++
	if m.getErr != nil {
		return nil, m.getErr
	}
	r, ok := m.records[contractID]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

func (m *mockHTLCRepository) GetHTLCByHashLock(_ context.Context, hashLock string) (*domain.HTLCRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.byHash[hashLock]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

func (m *mockHTLCRepository) UpdateHTLC(_ context.Context, record *domain.HTLCRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls++
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.records[record.ContractID]; !ok {
		return fmt.Errorf("htlc %s not found", record.ContractID)
	}
	cp := *record
	m.records[record.ContractID] = &cp
	m.byHash[record.HashLock] = &cp
	m.lastUpdate = &cp
	return nil
}

func (m *mockHTLCRepository) ListHTLCs(_ context.Context, filter ports.HTLCFilter) ([]*domain.HTLCRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.listCalls++
	var out []*domain.HTLCRecord
	for _, r := range m.records {
		if filter.AgreementID != "" && r.AgreementID != filter.AgreementID {
			continue
		}
		if filter.Sender != "" && r.Sender != filter.Sender {
			continue
		}
		if filter.Receiver != "" && r.Receiver != filter.Receiver {
			continue
		}
		if filter.State != "" && string(r.State) != filter.State {
			continue
		}
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func (m *mockHTLCRepository) ListNonTerminal(_ context.Context) ([]*domain.HTLCRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listNonTermErr != nil {
		return nil, m.listNonTermErr
	}
	terminal := map[domain.HTLCState]bool{
		domain.HTLCStateSettled:  true,
		domain.HTLCStateRefunded: true,
		domain.HTLCStateInvalid:  true,
	}
	var out []*domain.HTLCRecord
	for _, r := range m.records {
		if !terminal[r.State] {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ─── helper ───────────────────────────────────────────────────────────────────

// setupTestEnvWithHTLCRepo starts a gRPC server with a real mockHTLCRepository
// wired as HTLCRepo, an optional on-chain HTLCContractPort mock, and returns
// the gRPC client together with both mocks for assertion.
func setupTestEnvWithHTLCRepo(
	t *testing.T,
	htlcRepo *mockHTLCRepository,
	onChainHTLC *mockHTLC,
) (pb.PaymentOrchestratorServiceClient, *mockZeto, func()) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	zetoMock := &mockZeto{}
	var htlcPort ports.HTLCContractPort
	if onChainHTLC != nil {
		htlcPort = onChainHTLC
	}

	grpcServer, _, err := server.New(server.Config{
		Zeto:            zetoMock,
		HTLC:            htlcPort,
		Relay:           noopRelay{},
		HTLCRepo:        htlcRepo,
		PaladinIdentity: testPaladinIdentity,
		Logger:          logger,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = grpcServer.Serve(lis) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//nolint:staticcheck
	conn, dialErr := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if dialErr != nil {
		cancel()
		grpcServer.Stop()
		t.Fatalf("dial: %v", dialErr)
	}

	cleanup := func() {
		conn.Close()
		grpcServer.Stop()
		cancel()
	}
	t.Cleanup(cleanup)
	return pb.NewPaymentOrchestratorServiceClient(conn), zetoMock, cleanup
}

// ─── loadHTLCsFromDB ──────────────────────────────────────────────────────────

// TestLoadHTLCsFromDB_PopulatesInMemoryCache verifies that when HTLCRepo contains
// non-terminal records, they are loaded into the server's htlcs map on startup
// and immediately available for GetHTLCStatus.
func TestLoadHTLCsFromDB_PopulatesInMemoryCache(t *testing.T) {
	repo := newMockHTLCRepository()

	// Seed the repository with two non-terminal records and one terminal record.
	lockedRec := &domain.HTLCRecord{
		ContractID:  "preload-locked",
		AgreementID: "FX_PRELOAD",
		Sender:      testPaladinIdentity,
		Receiver:    "bank-b",
		Amount:      "1000",
		HashLock:    "aabbccdd00112233aabbccdd00112233aabbccdd00112233aabbccdd00112233",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
		State:       domain.HTLCStateLocked,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	settlingRec := &domain.HTLCRecord{
		ContractID:  "preload-settling",
		AgreementID: "FX_PRELOAD",
		Sender:      testPaladinIdentity,
		Receiver:    "bank-b",
		Amount:      "500",
		HashLock:    "1122334455667788112233445566778811223344556677881122334455667788",
		TimeLock:    uint64(time.Now().Unix()) + 1800,
		State:       domain.HTLCStateSettling,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	settledRec := &domain.HTLCRecord{
		ContractID:  "preload-settled",
		AgreementID: "FX_PRELOAD",
		Sender:      testPaladinIdentity,
		Receiver:    "bank-b",
		Amount:      "200",
		HashLock:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		TimeLock:    uint64(time.Now().Unix()) + 900,
		State:       domain.HTLCStateSettled, // terminal — should NOT be loaded
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	repo.records[lockedRec.ContractID] = lockedRec
	repo.byHash[lockedRec.HashLock] = lockedRec
	repo.records[settlingRec.ContractID] = settlingRec
	repo.byHash[settlingRec.HashLock] = settlingRec
	repo.records[settledRec.ContractID] = settledRec
	repo.byHash[settledRec.HashLock] = settledRec

	client, _, _ := setupTestEnvWithHTLCRepo(t, repo, nil)
	ctx := context.Background()

	// Locked record should be accessible immediately after startup.
	resp, err := client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: "preload-locked"})
	if err != nil {
		t.Fatalf("GetHTLCStatus(preload-locked): %v", err)
	}
	if resp.Lock.State != pb.HTLCState_HTLC_STATE_LOCKED {
		t.Errorf("expected LOCKED, got %s", resp.Lock.State)
	}

	// Settling record should also be accessible.
	resp2, err := client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: "preload-settling"})
	if err != nil {
		t.Fatalf("GetHTLCStatus(preload-settling): %v", err)
	}
	if resp2.Lock.State != pb.HTLCState_HTLC_STATE_SETTLING {
		t.Errorf("expected SETTLING, got %s", resp2.Lock.State)
	}

	// Terminal (SETTLED) record is not pre-loaded into the in-memory cache by
	// loadHTLCsFromDB, but GetHTLCStatus falls back to the DB — so it IS
	// accessible, and its state should be SETTLED.
	resp3, err := client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: "preload-settled"})
	if err != nil {
		t.Fatalf("GetHTLCStatus(preload-settled): %v", err)
	}
	if resp3.Lock.State != pb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("expected SETTLED for terminal record, got %s", resp3.Lock.State)
	}
}

// TestLoadHTLCsFromDB_NilRepo verifies that server.New succeeds and
// GetHTLCStatus works normally when HTLCRepo is nil (no DB configured).
func TestLoadHTLCsFromDB_NilRepo(t *testing.T) {
	// setupTestEnv() uses nil HTLCRepo — re-use for confidence.
	env := setupTestEnv(t)
	ctx := context.Background()

	// Lock an HTLC — should work with in-memory only.
	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_NIL_REPO",
		Receiver:    "bank-b",
		Amount:      "100",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}
	if lockResp.ContractId == "" {
		t.Error("expected non-empty contract_id")
	}
}

// TestLoadHTLCsFromDB_ReturnsErrorOnRepoFailure verifies that server.New
// propagates an error when ListNonTerminal fails.
func TestLoadHTLCsFromDB_ReturnsErrorOnRepoFailure(t *testing.T) {
	repo := newMockHTLCRepository()
	repo.listNonTermErr = errors.New("database unavailable")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	_, _, err := server.New(server.Config{
		Zeto:            &mockZeto{},
		Relay:           noopRelay{},
		HTLCRepo:        repo,
		PaladinIdentity: testPaladinIdentity,
		Logger:          logger,
	})
	if err == nil {
		t.Fatal("expected error from server.New when ListNonTerminal fails")
	}
}

// ─── persistHTLC / UpdateHTLC ─────────────────────────────────────────────────

// TestPersistHTLC_CalledOnLockAndSettle verifies that:
//   - CreateHTLC is called once when LockHTLC succeeds.
//   - UpdateHTLC is called (at least once, as SETTLING then SETTLED) after SettleHTLC.
//   - The final stored state is SETTLED.
func TestPersistHTLC_CalledOnLockAndSettle(t *testing.T) {
	repo := newMockHTLCRepository()
	client, _, _ := setupTestEnvWithHTLCRepo(t, repo, nil)
	ctx := context.Background()

	lockResp, err := client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_PERSIST_001",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	repo.mu.RLock()
	creates := repo.createCalls
	repo.mu.RUnlock()

	if creates != 1 {
		t.Errorf("expected CreateHTLC called once after LockHTLC, got %d", creates)
	}

	// Settle — should call UpdateHTLC at least twice (SETTLING, then SETTLED).
	_, err = client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC: %v", err)
	}

	repo.mu.RLock()
	updates := repo.updateCalls
	lastUpdate := repo.lastUpdate
	repo.mu.RUnlock()

	if updates < 1 {
		t.Errorf("expected at least 1 UpdateHTLC call after SettleHTLC, got %d", updates)
	}
	if lastUpdate == nil {
		t.Fatal("lastUpdate is nil — UpdateHTLC was never called")
	}
	if lastUpdate.State != domain.HTLCStateSettled {
		t.Errorf("expected final persisted state SETTLED, got %s", lastUpdate.State)
	}
}

// TestPersistHTLC_UpdateHTLCErrorIsLogged verifies that a failure in
// UpdateHTLC does NOT surface as an error to the caller — persistence is
// best-effort and the in-memory state is authoritative.
func TestPersistHTLC_UpdateHTLCErrorIsLogged(t *testing.T) {
	repo := newMockHTLCRepository()
	// Let CreateHTLC succeed so LockHTLC succeeds, then break updates.
	client, _, _ := setupTestEnvWithHTLCRepo(t, repo, nil)
	ctx := context.Background()

	lockResp, err := client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_PERSIST_ERR",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Now inject an update error.
	repo.mu.Lock()
	repo.updateErr = errors.New("DB write timeout")
	repo.mu.Unlock()

	// SettleHTLC should still succeed — persist errors are non-fatal.
	_, err = client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC should succeed even when persist fails: %v", err)
	}
}

// ─── CreateHTLC failure during LockHTLC ───────────────────────────────────────

// TestLockHTLC_CreateHTLCError_ReturnsInternalError verifies that when
// HTLCRepository.CreateHTLC fails, LockHTLC returns codes.Internal.
func TestLockHTLC_CreateHTLCError_ReturnsInternalError(t *testing.T) {
	repo := newMockHTLCRepository()
	repo.createErr = errors.New("unique constraint violation")

	client, _, _ := setupTestEnvWithHTLCRepo(t, repo, nil)
	ctx := context.Background()

	_, err := client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_CREATE_ERR",
		Receiver:    "bank-b",
		Amount:      "500",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err == nil {
		t.Fatal("expected error when CreateHTLC fails")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected codes.Internal, got %s", status.Code(err))
	}
}

// ─── HTLCRepo nil — backward-compat regression suite ─────────────────────────
// The following tests duplicate a representative subset of the existing test
// suite with explicit assertion that HTLCRepo being nil does not change
// any observable behaviour.

// TestHTLCRepoNil_LockSettleRefundCycle exercises the full HTLC lifecycle
// with HTLCRepo == nil and confirms the server behaves as before.
func TestHTLCRepoNil_LockSettleRefundCycle(t *testing.T) {
	env := setupTestEnv(t) // HTLCRepo is nil
	ctx := context.Background()

	// Lock
	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_NIL_CYCLE",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}
	if lockResp.ContractId == "" || lockResp.HashLock == "" {
		t.Error("expected non-empty contract_id and hash_lock")
	}

	// Status — should be LOCKED
	statusResp, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Fatalf("GetHTLCStatus: %v", err)
	}
	if statusResp.Lock.State != pb.HTLCState_HTLC_STATE_LOCKED {
		t.Errorf("expected LOCKED, got %s", statusResp.Lock.State)
	}

	// Settle
	settleResp, err := env.client.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: lockResp.ContractId,
		Secret:     lockResp.Secret,
	})
	if err != nil {
		t.Fatalf("SettleHTLC: %v", err)
	}
	if settleResp.ZetoTxHash == "" {
		t.Error("expected non-empty zeto_tx_hash after settle")
	}

	// Status after settle — should be SETTLED
	statusResp2, err := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if err != nil {
		t.Fatalf("GetHTLCStatus after settle: %v", err)
	}
	if statusResp2.Lock.State != pb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("expected SETTLED, got %s", statusResp2.Lock.State)
	}
}

// TestHTLCRepoNil_RefundHTLC exercises the refund path with nil HTLCRepo.
func TestHTLCRepoNil_RefundHTLC(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "FX_NIL_REFUND",
		Receiver:    "bank-b",
		Amount:      "200",
		TimeLock:    uint64(time.Now().Unix()) - 60, // already expired
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	refundResp, err := env.client.RefundHTLC(ctx, &pb.RefundHTLCRequest{
		ContractId: lockResp.ContractId,
	})
	if err != nil {
		t.Fatalf("RefundHTLC: %v", err)
	}
	if refundResp.ZetoTxHash == "" {
		t.Error("expected non-empty zeto_tx_hash after refund")
	}
}

// ─── mockHTLCRepository interface contract ────────────────────────────────────

// TestMockHTLCRepository_CreateAndGetHTLC verifies the in-process test double
// itself is correct — this is analogous to testing the MemoryEscrowRepository.
func TestMockHTLCRepository_CreateAndGetHTLC(t *testing.T) {
	repo := newMockHTLCRepository()
	ctx := context.Background()

	rec := &domain.HTLCRecord{
		ContractID:  "htlc-001",
		AgreementID: "FX_001",
		Sender:      "alice@spoke-a-bank-a",
		Receiver:    "bob@spoke-a-bank-b",
		Amount:      "1000",
		HashLock:    "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
		State:       domain.HTLCStateLocked,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := repo.CreateHTLC(ctx, rec); err != nil {
		t.Fatalf("CreateHTLC: %v", err)
	}

	got, err := repo.GetHTLC(ctx, "htlc-001")
	if err != nil {
		t.Fatalf("GetHTLC: %v", err)
	}
	if got == nil {
		t.Fatal("expected record, got nil")
	}
	if got.ContractID != rec.ContractID {
		t.Errorf("expected ContractID %q, got %q", rec.ContractID, got.ContractID)
	}
	if got.State != domain.HTLCStateLocked {
		t.Errorf("expected LOCKED, got %s", got.State)
	}
}

func TestMockHTLCRepository_GetHTLC_NotFound(t *testing.T) {
	repo := newMockHTLCRepository()
	ctx := context.Background()

	got, err := repo.GetHTLC(ctx, "nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent record")
	}
}

func TestMockHTLCRepository_GetHTLCByHashLock(t *testing.T) {
	repo := newMockHTLCRepository()
	ctx := context.Background()

	hashLock := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	rec := &domain.HTLCRecord{
		ContractID: "htlc-hash-001",
		HashLock:   hashLock,
		State:      domain.HTLCStateLocked,
		Sender:     "alice",
		Receiver:   "bob",
		Amount:     "100",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := repo.CreateHTLC(ctx, rec); err != nil {
		t.Fatalf("CreateHTLC: %v", err)
	}

	got, err := repo.GetHTLCByHashLock(ctx, hashLock)
	if err != nil {
		t.Fatalf("GetHTLCByHashLock: %v", err)
	}
	if got == nil || got.ContractID != rec.ContractID {
		t.Errorf("expected ContractID %q, got %v", rec.ContractID, got)
	}

	notFound, err := repo.GetHTLCByHashLock(ctx, "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for unknown hash_lock")
	}
}

func TestMockHTLCRepository_UpdateHTLC(t *testing.T) {
	repo := newMockHTLCRepository()
	ctx := context.Background()

	rec := &domain.HTLCRecord{
		ContractID: "htlc-upd",
		HashLock:   "1111111111111111111111111111111111111111111111111111111111111111",
		State:      domain.HTLCStateLocked,
		Sender:     "alice",
		Receiver:   "bob",
		Amount:     "100",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := repo.CreateHTLC(ctx, rec); err != nil {
		t.Fatalf("CreateHTLC: %v", err)
	}

	rec.State = domain.HTLCStateSettled
	rec.ZetoTxHash = "0xZETO_SETTLE"
	if err := repo.UpdateHTLC(ctx, rec); err != nil {
		t.Fatalf("UpdateHTLC: %v", err)
	}

	got, _ := repo.GetHTLC(ctx, "htlc-upd")
	if got.State != domain.HTLCStateSettled {
		t.Errorf("expected SETTLED, got %s", got.State)
	}
	if got.ZetoTxHash != "0xZETO_SETTLE" {
		t.Errorf("expected ZetoTxHash, got %q", got.ZetoTxHash)
	}
}

func TestMockHTLCRepository_UpdateHTLC_NotFound(t *testing.T) {
	repo := newMockHTLCRepository()
	ctx := context.Background()

	rec := &domain.HTLCRecord{
		ContractID: "htlc-missing",
		HashLock:   "2222222222222222222222222222222222222222222222222222222222222222",
		State:      domain.HTLCStateLocked,
	}
	if err := repo.UpdateHTLC(ctx, rec); err == nil {
		t.Error("expected error when updating non-existent HTLC")
	}
}

func TestMockHTLCRepository_ListHTLCs_Filters(t *testing.T) {
	repo := newMockHTLCRepository()
	ctx := context.Background()

	for i, id := range []string{"h1", "h2", "h3"} {
		hashLock := fmt.Sprintf("%064d", i)
		r := &domain.HTLCRecord{
			ContractID:  id,
			AgreementID: "FX_LIST",
			Sender:      "alice",
			Receiver:    "bob",
			Amount:      "100",
			HashLock:    hashLock,
			State:       domain.HTLCStateLocked,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if i == 2 {
			r.AgreementID = "FX_OTHER"
			r.State = domain.HTLCStateSettled
		}
		if err := repo.CreateHTLC(ctx, r); err != nil {
			t.Fatalf("CreateHTLC %s: %v", id, err)
		}
	}

	all, err := repo.ListHTLCs(ctx, ports.HTLCFilter{})
	if err != nil || len(all) != 3 {
		t.Errorf("expected 3 records, got %d (err=%v)", len(all), err)
	}

	byAgreement, err := repo.ListHTLCs(ctx, ports.HTLCFilter{AgreementID: "FX_LIST"})
	if err != nil || len(byAgreement) != 2 {
		t.Errorf("expected 2 for FX_LIST, got %d (err=%v)", len(byAgreement), err)
	}

	byState, err := repo.ListHTLCs(ctx, ports.HTLCFilter{State: string(domain.HTLCStateSettled)})
	if err != nil || len(byState) != 1 {
		t.Errorf("expected 1 SETTLED, got %d (err=%v)", len(byState), err)
	}
}

func TestMockHTLCRepository_ListNonTerminal(t *testing.T) {
	repo := newMockHTLCRepository()
	ctx := context.Background()

	states := []domain.HTLCState{
		domain.HTLCStateLocked,
		domain.HTLCStateSettling,
		domain.HTLCStateRefunding,
		domain.HTLCStateSettled,  // terminal
		domain.HTLCStateRefunded, // terminal
		domain.HTLCStateInvalid,  // terminal
	}

	for i, st := range states {
		hashLock := fmt.Sprintf("%064d", i)
		r := &domain.HTLCRecord{
			ContractID: fmt.Sprintf("nt-%d", i),
			HashLock:   hashLock,
			State:      st,
			Sender:     "a",
			Receiver:   "b",
			Amount:     "1",
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		repo.records[r.ContractID] = r
	}

	nonTerminal, err := repo.ListNonTerminal(ctx)
	if err != nil {
		t.Fatalf("ListNonTerminal: %v", err)
	}
	if len(nonTerminal) != 3 {
		t.Errorf("expected 3 non-terminal records (LOCKED, SETTLING, REFUNDING), got %d", len(nonTerminal))
	}
}

// ─── SearchHTLC DB fallback ───────────────────────────────────────────────────

// TestSearchHTLC_DBFallback verifies that SearchHTLC queries the HTLCRepository
// when the in-memory map returns no results for the given filter.
//
// Setup:
//  1. Start server with an EMPTY repo so loadHTLCsFromDB pre-populates nothing.
//  2. Inject one record directly into the repo's maps after server creation —
//     it lives only in the DB, NOT in the server's in-memory htlcs map.
//  3. Call SearchHTLC with AgreementId="agree-1".
//  4. Expect exactly one lock back with ContractId="db-htlc-1".
func TestSearchHTLC_DBFallback(t *testing.T) {
	repo := newMockHTLCRepository()

	// Create server with empty repo — loadHTLCsFromDB finds nothing to pre-load.
	client, _, _ := setupTestEnvWithHTLCRepo(t, repo, nil)

	// Inject a record directly into the mock's internal maps *after* server.New
	// so it is NOT in the server's in-memory htlcs map.
	dbRec := &domain.HTLCRecord{
		ContractID:  "db-htlc-1",
		AgreementID: "agree-1",
		Sender:      "sender@spoke-a",
		Receiver:    "receiver@spoke-a",
		Amount:      "500",
		HashLock:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
		State:       domain.HTLCStateLocked,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	repo.mu.Lock()
	repo.records[dbRec.ContractID] = dbRec
	repo.byHash[dbRec.HashLock] = dbRec
	repo.mu.Unlock()

	ctx := context.Background()
	resp, err := client.SearchHTLC(ctx, &pb.SearchHTLCRequest{AgreementId: "agree-1"})
	if err != nil {
		t.Fatalf("SearchHTLC: %v", err)
	}
	if len(resp.Locks) != 1 {
		t.Fatalf("expected 1 lock from DB fallback, got %d", len(resp.Locks))
	}
	if resp.Locks[0].ContractId != "db-htlc-1" {
		t.Errorf("expected ContractId %q, got %q", "db-htlc-1", resp.Locks[0].ContractId)
	}
}

// ─── handleRelayLockEvent — CounterpartyLocked persist ───────────────────────

// captureRelay is a relay test double that records the callback registered by
// SubscribeLockEvents so tests can fire events synchronously.
type captureRelay struct {
	mu           sync.Mutex
	lockHandler  func(ports.InteroperabilityProof) error
}

func (r *captureRelay) SubscribeLockEvents(_ context.Context, handler func(ports.InteroperabilityProof) error) error {
	r.mu.Lock()
	r.lockHandler = handler
	r.mu.Unlock()
	return nil
}
func (r *captureRelay) SubscribeSettleEvents(_ context.Context, _ func(ports.InteroperabilityProof) error) error {
	return nil
}
func (r *captureRelay) RelayProof(_ context.Context, _ ports.InteroperabilityProof) (string, error) {
	return "", nil
}
func (r *captureRelay) VerifyProof(_ context.Context, _ ports.InteroperabilityProof) (bool, error) {
	return true, nil
}

// fireLockEvent calls the registered lock callback synchronously and returns any error.
func (r *captureRelay) fireLockEvent(proof ports.InteroperabilityProof) error {
	r.mu.Lock()
	h := r.lockHandler
	r.mu.Unlock()
	if h == nil {
		return fmt.Errorf("captureRelay: no lock handler registered yet")
	}
	return h(proof)
}

// TestHandleRelayLockEvent_PersistsCounterpartyLocked verifies that when the relay
// delivers a LogHTLCLocked event from the counterparty (same HashLock, different
// ContractID), handleRelayLockEvent sets CounterpartyLocked=true on the local
// record and calls HTLCRepository.UpdateHTLC with that snapshot.
func TestHandleRelayLockEvent_PersistsCounterpartyLocked(t *testing.T) {
	repo := newMockHTLCRepository()
	relay := &captureRelay{}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	zetoMock := &mockZeto{}

	grpcServer, startRelayWorkers, err := server.New(server.Config{
		Zeto:            zetoMock,
		Relay:           relay,
		HTLCRepo:        repo,
		PaladinIdentity: testPaladinIdentity,
		Logger:          logger,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	lis, lisErr := net.Listen("tcp", "127.0.0.1:0")
	if lisErr != nil {
		t.Fatalf("listen: %v", lisErr)
	}
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(grpcServer.Stop)

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(dialCancel)
	//nolint:staticcheck
	conn, dialErr := grpc.DialContext(dialCtx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if dialErr != nil {
		t.Fatalf("dial: %v", dialErr)
	}
	t.Cleanup(func() { conn.Close() })
	client := pb.NewPaymentOrchestratorServiceClient(conn)

	// Start relay workers so SubscribeLockEvents is called and the handler captured.
	relayCtx, relayCancel := context.WithCancel(context.Background())
	t.Cleanup(relayCancel)
	go startRelayWorkers(relayCtx)

	// Give the relay goroutine a moment to call SubscribeLockEvents.
	time.Sleep(10 * time.Millisecond)

	// Lock an HTLC so there is a real record in both the in-memory map and the DB.
	ctx := context.Background()
	lockResp, err := client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "agree-relay-1",
		Receiver:    "bank-b",
		Amount:      "1000",
		TimeLock:    uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}
	hashLock := lockResp.HashLock

	// Record the UpdateHTLC call count before the relay event.
	repo.mu.RLock()
	updatesBefore := repo.updateCalls
	repo.mu.RUnlock()

	// Simulate the relay delivering a LogHTLCLocked event from the counterparty:
	// same HashLock but a DIFFERENT ContractID.
	proof := ports.InteroperabilityProof{
		HashLock:   hashLock,
		ContractID: "remote-contract-from-other-spoke",
		EventName:  "LogHTLCLocked",
	}
	if fireErr := relay.fireLockEvent(proof); fireErr != nil {
		t.Fatalf("fireLockEvent: %v", fireErr)
	}

	// Assert UpdateHTLC was called at least once more after the relay event.
	repo.mu.RLock()
	updatesAfter := repo.updateCalls
	lastUpdate := repo.lastUpdate
	repo.mu.RUnlock()

	if updatesAfter <= updatesBefore {
		t.Errorf("expected UpdateHTLC to be called after relay lock event, but updateCalls did not increase (before=%d after=%d)", updatesBefore, updatesAfter)
	}
	if lastUpdate == nil {
		t.Fatal("lastUpdate is nil — UpdateHTLC was never called")
	}
	if !lastUpdate.CounterpartyLocked {
		t.Errorf("expected CounterpartyLocked=true in persisted snapshot, got false")
	}
}
