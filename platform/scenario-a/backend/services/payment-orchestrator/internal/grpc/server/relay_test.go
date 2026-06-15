// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
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
	"google.golang.org/grpc/credentials/insecure"
)

// dualRelay records the lock/settle handlers passed by startRelayWorkers so
// the test can invoke them directly, simulating cross-spoke relay events.
type dualRelay struct {
	mu            sync.Mutex
	lockHandler   func(ports.InteroperabilityProof) error
	settleHandler func(ports.InteroperabilityProof) error
}

func (r *dualRelay) SubscribeLockEvents(_ context.Context, h func(ports.InteroperabilityProof) error) error {
	r.mu.Lock()
	r.lockHandler = h
	r.mu.Unlock()
	return nil
}
func (r *dualRelay) SubscribeSettleEvents(_ context.Context, h func(ports.InteroperabilityProof) error) error {
	r.mu.Lock()
	r.settleHandler = h
	r.mu.Unlock()
	return nil
}
func (r *dualRelay) RelayProof(context.Context, ports.InteroperabilityProof) (string, error) {
	return "", nil
}
func (r *dualRelay) VerifyProof(context.Context, ports.InteroperabilityProof) (bool, error) {
	return true, nil
}

func (r *dualRelay) lock() func(ports.InteroperabilityProof) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lockHandler
}
func (r *dualRelay) settle() func(ports.InteroperabilityProof) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settleHandler
}

type relayEnv struct {
	client pb.PaymentOrchestratorServiceClient
	relay  *dualRelay
	fxRepo *memFXRepo
	zeto   *mockZeto
}

func setupRelayEnv(t *testing.T, spokePrefix string, fxRepo *memFXRepo) *relayEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := &mockZeto{}
	relay := &dualRelay{}

	grpcServer, startWorkers, err := server.New(server.Config{
		Zeto:            mock,
		Relay:           relay,
		FXRepo:          fxRepo,
		CrossSpokeMode:  true,
		SpokePrefix:     spokePrefix,
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

	// Run the relay workers so the handlers get registered on dualRelay.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go startWorkers(workerCtx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//nolint:staticcheck
	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cancel()
		workerCancel()
		grpcServer.Stop()
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		workerCancel()
		conn.Close()
		grpcServer.Stop()
	})

	// Wait until the lock handler has been registered.
	deadline := time.After(2 * time.Second)
	for relay.lock() == nil {
		select {
		case <-deadline:
			t.Fatal("relay handlers not registered")
		case <-time.After(2 * time.Millisecond):
		}
	}

	return &relayEnv{client: pb.NewPaymentOrchestratorServiceClient(conn), relay: relay, fxRepo: mustFX(fxRepo), zeto: mock}
}

func mustFX(r *memFXRepo) *memFXRepo {
	if r == nil {
		return newMemFXRepo()
	}
	return r
}

// TestRelayLock_CreatesLocalLeg verifies that a counterparty lock event with no
// pre-existing local HTLC drives the orchestrator to create the matching local
// leg via the FX agreement lookup.
func TestRelayLock_CreatesLocalLeg(t *testing.T) {
	repo := newMemFXRepo()
	now := time.Now()
	// This orchestrator is spoke-a. Counterparty receiver is the spoke-b receiver.
	repo.CreateAgreement(context.Background(), &domain.FXAgreementRecord{
		TradeID: "T-RELAY", Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD", CounterCurrency: "BRL",
		Rate: "1.2", SpokeAReceiver: "recv@spoke-a-bank-a", SpokeBReceiver: "recv@spoke-b-bank-b",
		ExpiryDate: uint64(now.Add(time.Hour).Unix()), State: domain.FXStateAccepted,
		CreatedAt: now, UpdatedAt: now,
	})
	env := setupRelayEnv(t, "spoke-a", repo)

	payload, _ := json.Marshal(map[string]string{"receiver": "recv@spoke-b-bank-b"})
	proof := ports.InteroperabilityProof{
		ContractID: "remote-cid", HashLock: "ffeeddccbbaa00112233445566778899aabbccddeeff00112233445566778899",
		TimeLock: uint64(now.Add(time.Hour).Unix()), ProofPayload: payload,
	}
	if err := env.relay.lock()(proof); err != nil {
		t.Fatalf("lock handler: %v", err)
	}

	// The local leg should now exist (searchable by agreement id).
	search, err := env.client.SearchHTLC(context.Background(), &pb.SearchHTLCRequest{AgreementId: "T-RELAY"})
	if err != nil {
		t.Fatalf("SearchHTLC: %v", err)
	}
	if len(search.Locks) != 1 {
		t.Fatalf("expected 1 local leg, got %d", len(search.Locks))
	}
	if search.Locks[0].Receiver != "recv@spoke-a-bank-a" {
		t.Errorf("local receiver = %q", search.Locks[0].Receiver)
	}
	if env.zeto.lockCalled != 1 {
		t.Errorf("expected zeto.Lock called once, got %d", env.zeto.lockCalled)
	}
}

// TestRelayLock_MarksCounterpartyLocked verifies that when a local HTLC already
// exists with the same hashLock (but a different contractId), the relay lock
// event flips CounterpartyLocked, unblocking settlement.
func TestRelayLock_MarksCounterpartyLocked(t *testing.T) {
	env := setupRelayEnv(t, "", newMemFXRepo())
	ctx := context.Background()

	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "T-CL", Receiver: "bank-b", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}
	status0, _ := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if status0.Lock.CounterpartyLocked {
		t.Fatal("CounterpartyLocked should start false")
	}

	// Relay observes the counterparty leg (same hashLock, different contractId).
	proof := ports.InteroperabilityProof{ContractID: "remote-other", HashLock: lockResp.HashLock}
	if err := env.relay.lock()(proof); err != nil {
		t.Fatalf("lock handler: %v", err)
	}
	status1, _ := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if !status1.Lock.CounterpartyLocked {
		t.Error("expected CounterpartyLocked true after counterparty leg observed")
	}

	// Own-lock echo (same contractId) is a no-op.
	echo := ports.InteroperabilityProof{ContractID: lockResp.ContractId, HashLock: lockResp.HashLock}
	if err := env.relay.lock()(echo); err != nil {
		t.Fatalf("echo handler: %v", err)
	}
}

func TestRelayLock_NoMatchingAgreement(t *testing.T) {
	env := setupRelayEnv(t, "spoke-a", newMemFXRepo())
	payload, _ := json.Marshal(map[string]string{"receiver": "unknown@spoke-b-bank-z"})
	proof := ports.InteroperabilityProof{
		ContractID: "remote-x", HashLock: "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
		ProofPayload: payload, TimeLock: uint64(time.Now().Add(time.Hour).Unix()),
	}
	// No matching agreement → handler logs and returns nil (no leg created).
	if err := env.relay.lock()(proof); err != nil {
		t.Fatalf("lock handler: %v", err)
	}
	if env.zeto.lockCalled != 0 {
		t.Errorf("no leg should be created, zeto.Lock called %d", env.zeto.lockCalled)
	}
}

func TestRelayLock_MissingReceiverPayload(t *testing.T) {
	env := setupRelayEnv(t, "spoke-a", newMemFXRepo())
	proof := ports.InteroperabilityProof{
		ContractID: "remote-y", HashLock: "aa00112233445566778899aabbccddeeff00112233445566778899aabbccddee",
		ProofPayload: []byte(`{}`),
	}
	if err := env.relay.lock()(proof); err != nil {
		t.Fatalf("lock handler: %v", err)
	}
}

// TestRelaySettle_SettlesLocalLeg drives a settle event from the counterparty
// (different contractId) that settles the local HTLC via hashLock match.
func TestRelaySettle_SettlesLocalLeg(t *testing.T) {
	env := setupRelayEnv(t, "", newMemFXRepo())
	ctx := context.Background()

	// Create a local leg via the relay lock path so it has no secret (responder leg).
	lockResp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "T-SETTLE", Receiver: "bank-b", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC: %v", err)
	}

	// Mark counterparty as locked so the settle guard passes (cross-spoke mode on).
	if err := env.relay.lock()(ports.InteroperabilityProof{ContractID: "remote-leg", HashLock: lockResp.HashLock}); err != nil {
		t.Fatalf("lock handler: %v", err)
	}

	// Settle event carries the secret and a foreign contractId.
	payload, _ := json.Marshal(map[string]string{"secret": lockResp.Secret})
	proof := ports.InteroperabilityProof{ContractID: "foreign-settle-cid", ProofPayload: payload}
	if err := env.relay.settle()(proof); err != nil {
		t.Fatalf("settle handler: %v", err)
	}

	st, _ := env.client.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{ContractId: lockResp.ContractId})
	if st.Lock.State != pb.HTLCState_HTLC_STATE_SETTLED {
		t.Errorf("expected SETTLED after relay settle, got %s", st.Lock.State)
	}
}

func TestRelaySettle_OwnEchoIgnored(t *testing.T) {
	env := setupRelayEnv(t, "", newMemFXRepo())
	ctx := context.Background()
	lockResp, _ := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "T-ECHO", Receiver: "bank-b", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	// Own-settle echo: contractId matches a local HTLC → ignored, no transfer.
	payload, _ := json.Marshal(map[string]string{"secret": lockResp.Secret})
	proof := ports.InteroperabilityProof{ContractID: lockResp.ContractId, ProofPayload: payload}
	if err := env.relay.settle()(proof); err != nil {
		t.Fatalf("settle echo: %v", err)
	}
	if env.zeto.transferLockedCalled != 0 {
		t.Errorf("own echo must not transfer, got %d", env.zeto.transferLockedCalled)
	}
}

func TestRelaySettle_MissingSecret(t *testing.T) {
	env := setupRelayEnv(t, "", newMemFXRepo())
	proof := ports.InteroperabilityProof{ContractID: "no-secret-cid", ProofPayload: []byte(`{}`)}
	if err := env.relay.settle()(proof); err == nil {
		t.Error("expected error for missing secret")
	}
}

func TestRelaySettle_NoMatchingLocalHTLC(t *testing.T) {
	env := setupRelayEnv(t, "", newMemFXRepo())
	// A secret that matches no local HTLC → NotFound is swallowed (returns nil).
	payload, _ := json.Marshal(map[string]string{"secret": "0011223344556677889900112233445566778899001122334455667788990011"})
	proof := ports.InteroperabilityProof{ContractID: "orphan-cid", ProofPayload: payload}
	if err := env.relay.settle()(proof); err != nil {
		t.Fatalf("expected nil for unmatched settle, got %v", err)
	}
}

// TestStartRelayWorkers_NilRelay verifies the early return when no relay is set.
func TestStartRelayWorkers_NilRelay(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	_, startWorkers, err := server.New(server.Config{
		Zeto: &mockZeto{}, Relay: nil, PaladinIdentity: testPaladinIdentity, Logger: logger,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	// Should return promptly without blocking.
	done := make(chan struct{})
	go func() { startWorkers(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startRelayWorkers with nil relay should return immediately")
	}
}
