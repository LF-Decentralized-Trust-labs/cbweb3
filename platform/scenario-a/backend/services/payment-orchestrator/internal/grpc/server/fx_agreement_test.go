// SPDX-License-Identifier: Apache-2.0

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

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// memFXRepo is an in-memory FXAgreementRepository for hermetic server tests.
type memFXRepo struct {
	mu         sync.Mutex
	agreements map[string]*domain.FXAgreementRecord
	events     map[string][]*domain.FXAgreementEvent
	listErr    error
}

func newMemFXRepo() *memFXRepo {
	return &memFXRepo{
		agreements: make(map[string]*domain.FXAgreementRecord),
		events:     make(map[string][]*domain.FXAgreementEvent),
	}
}

func (m *memFXRepo) CreateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.agreements[r.TradeID] = &cp
	return nil
}
func (m *memFXRepo) GetAgreement(_ context.Context, tradeID string) (*domain.FXAgreementRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.agreements[tradeID]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}
func (m *memFXRepo) UpdateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.agreements[r.TradeID] = &cp
	return nil
}
func (m *memFXRepo) ListAgreements(_ context.Context, f ports.FXAgreementFilter) ([]*domain.FXAgreementRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	var out []*domain.FXAgreementRecord
	for _, r := range m.agreements {
		if f.Counterparty != "" && r.CounterpartyB != f.Counterparty && r.Originator != f.Counterparty {
			continue
		}
		if f.State != "" && r.State != f.State {
			continue
		}
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}
func (m *memFXRepo) CreateAuditEvent(_ context.Context, e *domain.FXAgreementEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[e.TradeID] = append(m.events[e.TradeID], e)
	return nil
}
func (m *memFXRepo) ListAuditEvents(_ context.Context, tradeID string) ([]*domain.FXAgreementEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events[tradeID], nil
}
func (m *memFXRepo) ListExpiredNonTerminal(context.Context, int64) ([]*domain.FXAgreementRecord, error) {
	return nil, nil
}

type fxEnv struct {
	client pb.PaymentOrchestratorServiceClient
	fxRepo *memFXRepo
	zeto   *mockZeto
}

func setupFXEnv(t *testing.T, spokePrefix string) *fxEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := &mockZeto{}
	repo := newMemFXRepo()

	grpcServer, _, err := server.New(server.Config{
		Zeto:            mock,
		Relay:           noopRelay{},
		FXRepo:          repo,
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//nolint:staticcheck
	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cancel()
		grpcServer.Stop()
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		conn.Close()
		grpcServer.Stop()
	})
	return &fxEnv{client: pb.NewPaymentOrchestratorServiceClient(conn), fxRepo: repo, zeto: mock}
}

func proposeValid(t *testing.T, env *fxEnv, tradeID string) string {
	t.Helper()
	resp, err := env.client.ProposeFXAgreement(context.Background(), &pb.ProposeFXAgreementRequest{
		TradeId: tradeID, Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD",
		CounterCurrency: "BRL", Rate: "1.2", ExpiryDate: uint64(time.Now().Add(time.Hour).Unix()),
	})
	if err != nil {
		t.Fatalf("ProposeFXAgreement: %v", err)
	}
	return resp.TradeId
}

func TestFX_ProposeAcceptSettle_FullLifecycle(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()

	tid := proposeValid(t, env, "T-LIFE")

	// GetFXAgreement reflects PROPOSED.
	got, err := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tid})
	if err != nil || got.Agreement.State != pb.FXAgreementState_FX_STATE_PROPOSED {
		t.Fatalf("get proposed: %v state=%v", err, got.Agreement.State)
	}

	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if _, err := env.client.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Settle: %v", err)
	}
	got, _ = env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tid})
	if got.Agreement.State != pb.FXAgreementState_FX_STATE_SETTLED {
		t.Errorf("expected SETTLED, got %v", got.Agreement.State)
	}

	// Audit events recorded for each transition (proposed, accepted, settled).
	events, err := env.client.ListFXAgreementEvents(ctx, &pb.ListFXAgreementEventsRequest{TradeId: tid})
	if err != nil {
		t.Fatalf("ListFXAgreementEvents: %v", err)
	}
	if len(events.Events) != 3 {
		t.Errorf("expected 3 audit events, got %d", len(events.Events))
	}
}

func TestFX_Reject(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()
	tid := proposeValid(t, env, "T-REJ")
	if _, err := env.client.RejectFXAgreement(ctx, &pb.RejectFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	// Accepting a rejected agreement must fail.
	_, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

func TestFX_Cancel(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()
	tid := proposeValid(t, env, "T-CAN")
	if _, err := env.client.CancelFXAgreement(ctx, &pb.CancelFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	got, _ := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tid})
	if got.Agreement.State != pb.FXAgreementState_FX_STATE_CANCELLED {
		t.Errorf("expected CANCELLED, got %v", got.Agreement.State)
	}
	// Settling a cancelled agreement must fail.
	_, err := env.client.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{TradeId: tid})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition settling cancelled, got %v", err)
	}
}

func TestFX_Propose_Validation(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()

	cases := []struct {
		name string
		req  *pb.ProposeFXAgreementRequest
	}{
		{"missing fields", &pb.ProposeFXAgreementRequest{}},
		{"past expiry", &pb.ProposeFXAgreementRequest{
			CounterpartyB: "b", OriginAmount: "100", CounterAmount: "120",
			OriginCurrency: "USD", CounterCurrency: "BRL", Rate: "1.2",
			ExpiryDate: uint64(time.Now().Add(-time.Hour).Unix()),
		}},
		{"same currency", &pb.ProposeFXAgreementRequest{
			CounterpartyB: "b", OriginAmount: "100", CounterAmount: "120",
			OriginCurrency: "USD", CounterCurrency: "USD", Rate: "1.2",
			ExpiryDate: uint64(time.Now().Add(time.Hour).Unix()),
		}},
		{"bad origin amount", &pb.ProposeFXAgreementRequest{
			CounterpartyB: "b", OriginAmount: "abc", CounterAmount: "120",
			OriginCurrency: "USD", CounterCurrency: "BRL", Rate: "1.2",
			ExpiryDate: uint64(time.Now().Add(time.Hour).Unix()),
		}},
		{"rate inconsistent with amounts", &pb.ProposeFXAgreementRequest{
			CounterpartyB: "b", OriginAmount: "100", CounterAmount: "120",
			OriginCurrency: "USD", CounterCurrency: "BRL", Rate: "9.9",
			ExpiryDate: uint64(time.Now().Add(time.Hour).Unix()),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := env.client.ProposeFXAgreement(ctx, c.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestFX_NotFoundAndValidation(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()

	// Empty trade_id → InvalidArgument.
	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("accept empty: %v", err)
	}
	if _, err := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("get empty: %v", err)
	}
	// Unknown trade_id → NotFound.
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"accept", func() error { _, e := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: "ghost"}); return e }},
		{"reject", func() error { _, e := env.client.RejectFXAgreement(ctx, &pb.RejectFXAgreementRequest{TradeId: "ghost"}); return e }},
		{"cancel", func() error { _, e := env.client.CancelFXAgreement(ctx, &pb.CancelFXAgreementRequest{TradeId: "ghost"}); return e }},
		{"settle", func() error { _, e := env.client.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{TradeId: "ghost"}); return e }},
		{"get", func() error { _, e := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: "ghost"}); return e }},
	} {
		if status.Code(tc.call()) != codes.NotFound {
			t.Errorf("%s ghost: expected NotFound", tc.name)
		}
	}
}

func TestFX_List(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()
	proposeValid(t, env, "T-L1")
	t2 := proposeValid(t, env, "T-L2")
	_, _ = env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: t2})

	all, err := env.client.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{})
	if err != nil || len(all.Agreements) != 2 {
		t.Fatalf("list all: len=%d err=%v", len(all.Agreements), err)
	}
	accepted, _ := env.client.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{State: "FX_STATE_ACCEPTED"})
	if len(accepted.Agreements) != 1 || accepted.Agreements[0].TradeId != t2 {
		t.Errorf("filter accepted = %+v", accepted.Agreements)
	}
	byParty, _ := env.client.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{Counterparty: "bank-b"})
	if n := len(byParty.Agreements); n != 2 {
		t.Errorf("by party = %d", n)
	}
}

func TestFX_ListEvents_Validation(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()
	if _, err := env.client.ListFXAgreementEvents(ctx, &pb.ListFXAgreementEventsRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty trade_id: %v", err)
	}
}

func TestFX_ListEvents_NoRepo(t *testing.T) {
	// With no FXRepo, ListFXAgreementEvents must return FailedPrecondition.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	grpcServer, _, err := server.New(server.Config{
		Zeto: &mockZeto{}, Relay: noopRelay{}, PaladinIdentity: testPaladinIdentity, Logger: logger,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	lis, _ := net.Listen("tcp", "127.0.0.1:0")
	go func() { _ = grpcServer.Serve(lis) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//nolint:staticcheck
	conn, _ := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	t.Cleanup(func() { cancel(); conn.Close(); grpcServer.Stop() })
	client := pb.NewPaymentOrchestratorServiceClient(conn)

	if _, err := client.ListFXAgreementEvents(ctx, &pb.ListFXAgreementEventsRequest{TradeId: "x"}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition with no repo, got %v", err)
	}
}

// --- FX agreement gate on LockHTLC ---

func TestLockHTLC_FXAgreementGate(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()

	// Lock referencing an unaccepted (PROPOSED) agreement must be rejected.
	tid := proposeValid(t, env, "T-GATE")
	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: tid, Receiver: "bank-b", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition for unaccepted agreement, got %v", err)
	}

	// After acceptance, lock succeeds.
	_, _ = env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid})
	resp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: tid, Receiver: "bank-b", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC after accept: %v", err)
	}
	if resp.ContractId == "" {
		t.Error("expected contract id")
	}
}

func TestLockHTLC_FXAgreementExpired(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()

	// Create an accepted-but-expired agreement directly in the repo.
	now := time.Now()
	env.fxRepo.CreateAgreement(ctx, &domain.FXAgreementRecord{
		TradeID: "T-EXP", Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD",
		CounterCurrency: "BRL", Rate: "1.2",
		ExpiryDate: uint64(now.Add(-time.Minute).Unix()),
		State:      domain.FXStateAccepted, CreatedAt: now, UpdatedAt: now,
	})

	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "T-EXP", Receiver: "bank-b", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition for expired FX agreement, got %v", err)
	}
}

func TestFX_RepoListError(t *testing.T) {
	env := setupFXEnv(t, "")
	ctx := context.Background()
	env.fxRepo.listErr = errors.New("db down")
	_, err := env.client.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal on list error, got %v", err)
	}
}
