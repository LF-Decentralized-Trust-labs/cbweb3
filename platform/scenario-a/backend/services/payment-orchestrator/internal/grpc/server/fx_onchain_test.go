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

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// fakeFXClient is a test double for FXAgreementContractPort that records calls
// and can be made to fail to exercise the on-chain error paths.
type fakeFXClient struct {
	acceptCalled, rejectCalled, cancelCalled, settleCalled int
	acceptErr                                              error
	// penteTargetSeen records whether the last call carried a Pente FX target in ctx.
	penteTargetSeen bool
}

func (f *fakeFXClient) Propose(context.Context, ports.FXProposalParams) (string, error) {
	return "tx-propose", nil
}
func (f *fakeFXClient) ProposeOnBehalf(context.Context, ports.FXProposalParams) (string, error) {
	return "tx-propose-ob", nil
}
func (f *fakeFXClient) Accept(ctx context.Context, _ [32]byte) (string, error) {
	f.acceptCalled++
	if _, ok := ports.PenteFXTargetFromContext(ctx); ok {
		f.penteTargetSeen = true
	}
	return "tx-accept", f.acceptErr
}
func (f *fakeFXClient) AcceptOnBehalf(context.Context, [32]byte) (string, error) {
	f.acceptCalled++
	return "tx-accept-ob", f.acceptErr
}
func (f *fakeFXClient) Reject(context.Context, [32]byte) (string, error) {
	f.rejectCalled++
	return "tx-reject", nil
}
func (f *fakeFXClient) RejectOnBehalf(context.Context, [32]byte) (string, error) {
	f.rejectCalled++
	return "tx-reject-ob", nil
}
func (f *fakeFXClient) Cancel(context.Context, [32]byte) (string, error) {
	f.cancelCalled++
	return "tx-cancel", nil
}
func (f *fakeFXClient) Settle(context.Context, [32]byte) (string, error) {
	f.settleCalled++
	return "tx-settle", nil
}

// fakePente is a PenteClientPort double that returns a fixed context ref.
type fakePente struct {
	called int
	err    error
}

func (p *fakePente) EnsureFXContext(context.Context, ports.PenteContextRequest) (*ports.PenteContextResult, error) {
	p.called++
	if p.err != nil {
		return nil, p.err
	}
	return &ports.PenteContextResult{GroupID: "grp-1", ContractAddress: "0xcontract"}, nil
}

type fxChainEnv struct {
	client pb.PaymentOrchestratorServiceClient
	fxRepo *memFXRepo
	besu   *fakeFXClient
	pente  *fakePente
}

func setupFXChainEnv(t *testing.T, cfg server.Config) *fxChainEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg.Logger = logger
	cfg.Relay = noopRelay{}
	if cfg.Zeto == nil {
		cfg.Zeto = &mockZeto{}
	}
	if cfg.PaladinIdentity == "" {
		cfg.PaladinIdentity = testPaladinIdentity
	}

	grpcServer, _, err := server.New(cfg)
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
	t.Cleanup(func() { cancel(); conn.Close(); grpcServer.Stop() })

	return &fxChainEnv{client: pb.NewPaymentOrchestratorServiceClient(conn)}
}

func proposeChain(t *testing.T, c pb.PaymentOrchestratorServiceClient, tid string) string {
	t.Helper()
	resp, err := c.ProposeFXAgreement(context.Background(), &pb.ProposeFXAgreementRequest{
		TradeId: tid, Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD",
		CounterCurrency: "BRL", Rate: "1.2", ExpiryDate: uint64(time.Now().Add(time.Hour).Unix()),
	})
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	return resp.TradeId
}

// TestFX_OnChainBesuPath exercises Accept/Reject/Cancel/Settle with a Besu
// FXAgreement client wired, covering the on-chain branches and the client
// selection helpers.
func TestFX_OnChainBesuPath(t *testing.T) {
	repo := newMemFXRepo()
	besu := &fakeFXClient{}
	env := setupFXChainEnv(t, server.Config{FXRepo: repo, FXAgreementBesu: besu})
	env.fxRepo, env.besu = repo, besu
	ctx := context.Background()

	tid := proposeChain(t, env.client, "T-BESU")
	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if besu.acceptCalled != 1 {
		t.Errorf("on-chain Accept not called: %d", besu.acceptCalled)
	}
	if _, err := env.client.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if besu.settleCalled != 1 {
		t.Errorf("on-chain Settle not called: %d", besu.settleCalled)
	}

	// Reject path on a fresh proposed agreement.
	tid2 := proposeChain(t, env.client, "T-BESU-2")
	if _, err := env.client.RejectFXAgreement(ctx, &pb.RejectFXAgreementRequest{TradeId: tid2}); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if besu.rejectCalled != 1 {
		t.Errorf("on-chain Reject not called: %d", besu.rejectCalled)
	}

	// Cancel path.
	tid3 := proposeChain(t, env.client, "T-BESU-3")
	if _, err := env.client.CancelFXAgreement(ctx, &pb.CancelFXAgreementRequest{TradeId: tid3}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if besu.cancelCalled != 1 {
		t.Errorf("on-chain Cancel not called: %d", besu.cancelCalled)
	}
}

func TestFX_OnChainAcceptError(t *testing.T) {
	repo := newMemFXRepo()
	besu := &fakeFXClient{acceptErr: errors.New("revert")}
	env := setupFXChainEnv(t, server.Config{FXRepo: repo, FXAgreementBesu: besu})
	ctx := context.Background()
	tid := proposeChain(t, env.client, "T-ERR")
	_, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal on on-chain accept failure, got %v", err)
	}
}

func TestFX_OnBehalfPath(t *testing.T) {
	repo := newMemFXRepo()
	besu := &fakeFXClient{}
	env := setupFXChainEnv(t, server.Config{FXRepo: repo, FXAgreementBesu: besu})
	ctx := context.Background()
	// Propose on-behalf, then accept on-behalf.
	resp, err := env.client.ProposeFXAgreement(ctx, &pb.ProposeFXAgreementRequest{
		TradeId: "T-OB", Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD",
		CounterCurrency: "BRL", Rate: "1.2",
		ExpiryDate: uint64(time.Now().Add(time.Hour).Unix()), OnBehalf: true,
	})
	if err != nil {
		t.Fatalf("Propose on-behalf: %v", err)
	}
	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: resp.TradeId, OnBehalf: true}); err != nil {
		t.Fatalf("Accept on-behalf: %v", err)
	}
}

// TestFX_PentePath verifies that accepting with a Pente client ensures the
// bilateral context and routes the on-chain call through the Pente target.
func TestFX_PentePath(t *testing.T) {
	repo := newMemFXRepo()
	pente := &fakePente{}
	besu := &fakeFXClient{}
	env := setupFXChainEnv(t, server.Config{
		FXRepo: repo, Pente: pente, FXAgreementBesu: besu, FXAgreementPente: besu,
	})
	ctx := context.Background()
	tid := proposeChain(t, env.client, "T-PENTE")
	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if pente.called != 1 {
		t.Errorf("EnsureFXContext should be called once, got %d", pente.called)
	}
	if !besu.penteTargetSeen {
		t.Error("expected Pente FX target carried in context for the on-chain Accept call")
	}
	// The agreement should now carry the group/contract metadata.
	got, _ := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tid})
	if got.Agreement.GroupId != "grp-1" || got.Agreement.ContractAddress != "0xcontract" {
		t.Errorf("pente metadata not persisted: %+v", got.Agreement)
	}
}

func TestFX_PenteEnsureError(t *testing.T) {
	repo := newMemFXRepo()
	pente := &fakePente{err: errors.New("pente down")}
	env := setupFXChainEnv(t, server.Config{FXRepo: repo, Pente: pente, FXAgreementBesu: &fakeFXClient{}})
	ctx := context.Background()
	tid := proposeChain(t, env.client, "T-PENTE-ERR")
	_, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal on Pente ensure failure, got %v", err)
	}
}

// --- Strict HTLC mode: agreement commitment registration on accept ---

func TestFX_StrictMode_RegistersAgreementCommitment(t *testing.T) {
	repo := newMemFXRepo()
	htlcMock := &mockHTLC{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	grpcServer, _, err := server.New(server.Config{
		Zeto: &mockZeto{}, HTLC: htlcMock, Relay: noopRelay{}, FXRepo: repo,
		StrictHTLC: true, PaladinIdentity: testPaladinIdentity, Logger: logger,
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

	tid := proposeChain(t, client, "T-STRICT")
	if _, err := client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// In strict mode with no FX on-chain client, accept registers the
	// agreement commitment hash on the HTLC contract.
	if htlcMock.commitCalled != 1 {
		t.Errorf("expected RegisterAgreementCommitment called once, got %d", htlcMock.commitCalled)
	}
}

func TestLockHTLC_StrictMode_RequiresAgreement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	grpcServer, _, err := server.New(server.Config{
		Zeto: &mockZeto{}, Relay: noopRelay{}, FXRepo: newMemFXRepo(),
		StrictHTLC: true, PaladinIdentity: testPaladinIdentity, Logger: logger,
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

	// Strict mode: referencing an unknown agreement_id must fail closed.
	_, err = client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "unknown-agreement", Receiver: "bank-b", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition for unknown agreement in strict mode, got %v", err)
	}
}

// --- validateHTLCTermsAgainstAgreement mismatch paths ---

func TestLockHTLC_TermsMismatch(t *testing.T) {
	repo := newMemFXRepo()
	now := time.Now()
	repo.CreateAgreement(context.Background(), &domain.FXAgreementRecord{
		TradeID: "T-TERMS", Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD", CounterCurrency: "BRL",
		Rate: "1.2", SpokeAReceiver: "recv@spoke-a-bank-a",
		ExpiryDate: uint64(now.Add(time.Hour).Unix()), State: domain.FXStateAccepted,
		CreatedAt: now, UpdatedAt: now,
	})
	env := setupFXChainEnv(t, server.Config{
		FXRepo: repo, SpokePrefix: "spoke-a", PaladinIdentity: "funded_operator@spoke-a-bank-a",
	})
	ctx := context.Background()

	// Wrong receiver → FailedPrecondition.
	_, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "T-TERMS", Receiver: "wrong@spoke-a-bank-a", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition for receiver mismatch, got %v", err)
	}

	// Correct receiver but wrong amount → FailedPrecondition.
	_, err = env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "T-TERMS", Receiver: "recv@spoke-a-bank-a", Amount: "999",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition for amount mismatch, got %v", err)
	}

	// Correct terms → success.
	resp, err := env.client.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: "T-TERMS", Receiver: "recv@spoke-a-bank-a", Amount: "100",
		TimeLock: uint64(time.Now().Unix()) + 3600,
	})
	if err != nil {
		t.Fatalf("LockHTLC matching terms: %v", err)
	}
	if resp.ContractId == "" {
		t.Error("expected contract id")
	}
}
