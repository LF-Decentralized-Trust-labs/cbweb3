// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type flowEnv struct {
	client     pb.PaymentOrchestratorServiceClient
	escrowRepo *fakeEscrowRepo
	fxRepo     *fakeFXRepo
	fiat       *mockFiat
	token      *mockToken
}

func setupFlowEnv(t *testing.T, cfg server.Config) *flowEnv {
	t.Helper()
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg.Relay = noopRelay{}

	grpcServer, newErr := server.New(cfg)
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
		cancel()
		conn.Close()
		grpcServer.Stop()
	})
	return &flowEnv{client: pb.NewPaymentOrchestratorServiceClient(conn)}
}

func newFlowEnv(t *testing.T) *flowEnv {
	t.Helper()
	escrowRepo := newFakeEscrowRepo()
	fxRepo := newFakeFXRepo()
	fiat := &mockFiat{balance: "1000"}
	token := &mockToken{balance: "1000"}
	env := setupFlowEnv(t, server.Config{
		Token:      token,
		Fiat:       fiat,
		EscrowRepo: escrowRepo,
		FXRepo:     fxRepo,
	})
	env.escrowRepo = escrowRepo
	env.fxRepo = fxRepo
	env.fiat = fiat
	env.token = token
	return env
}

// GetBalance with an explicit address reads via BalanceOf.
func TestGetBalance_WithAddress(t *testing.T) {
	env := newFlowEnv(t)
	resp, err := env.client.GetBalance(context.Background(), &pb.GetBalanceRequest{Address: "0xabc"})
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if resp.Balance != "1000" {
		t.Errorf("expected 1000, got %s", resp.Balance)
	}
}

// A repository error on lookup surfaces as Internal.
func TestApproveDeposit_RepoError(t *testing.T) {
	escrowRepo := newFakeEscrowRepo()
	escrowRepo.failGet = true
	env := setupFlowEnv(t, server.Config{Token: &mockToken{}, Fiat: &mockFiat{}, EscrowRepo: escrowRepo})
	_, err := env.client.ApproveDeposit(context.Background(), &pb.ApproveDepositRequest{DepositId: "x"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

// ApproveRedeem requires the tCeBM adapter; without it the call is Unavailable.
func TestApproveRedeem_NoTokenAdapter(t *testing.T) {
	escrowRepo := newFakeEscrowRepo()
	env := setupFlowEnv(t, server.Config{Fiat: &mockFiat{}, EscrowRepo: escrowRepo})
	ctx := context.Background()
	req, _ := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{RequesterBesuAddress: "0xb", Amount: "1"})
	_, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: req.RedeemId})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
}

// --- Deposit lifecycle ---

func TestDepositLifecycle_RegisterApproveMint(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()

	reg, err := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{
		RequesterBesuAddress: "0xbank", Amount: "500",
	})
	if err != nil {
		t.Fatalf("RegisterDeposit: %v", err)
	}
	if reg.DepositId == "" {
		t.Fatal("expected deposit id")
	}

	ap, err := env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: reg.DepositId})
	if err != nil {
		t.Fatalf("ApproveDeposit: %v", err)
	}
	if ap.FiatMintTxHash != "mock-fiat-mint-tx" {
		t.Errorf("expected mint tx hash, got %q", ap.FiatMintTxHash)
	}

	list, err := env.client.ListDeposits(ctx, &pb.ListDepositsRequest{})
	if err != nil {
		t.Fatalf("ListDeposits: %v", err)
	}
	if len(list.Deposits) != 1 || list.Deposits[0].Status != pb.DepositStatus_DEPOSIT_STATUS_APPROVED {
		t.Errorf("expected 1 approved deposit, got %+v", list.Deposits)
	}
}

func TestRegisterDeposit_Validation(t *testing.T) {
	env := newFlowEnv(t)
	_, err := env.client.RegisterDeposit(context.Background(), &pb.RegisterDepositRequest{Amount: "1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestApproveDeposit_NotFound(t *testing.T) {
	env := newFlowEnv(t)
	_, err := env.client.ApproveDeposit(context.Background(), &pb.ApproveDepositRequest{DepositId: "nope"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

// Mint failure during approval flips status to MINT_FAILED but the call returns OK
// with empty tx hash (the deposit is approved; mint can be retried).
func TestApproveDeposit_MintFailureMarksMintFailed(t *testing.T) {
	escrowRepo := newFakeEscrowRepo()
	var fiat = errFiat{err: errors.New("revert")}
	env := setupFlowEnv(t, server.Config{
		Token:      &mockToken{balance: "1"},
		Fiat:       fiat,
		EscrowRepo: escrowRepo,
	})
	env.escrowRepo = escrowRepo
	ctx := context.Background()

	reg, err := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{RequesterBesuAddress: "0xbank", Amount: "5"})
	if err != nil {
		t.Fatalf("RegisterDeposit: %v", err)
	}
	resp, err := env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: reg.DepositId})
	if err != nil {
		t.Fatalf("ApproveDeposit should not error on mint failure: %v", err)
	}
	if resp.FiatMintTxHash != "" {
		t.Errorf("expected empty tx hash on mint failure")
	}
	rec, _, _ := escrowRepo.GetDeposit(ctx, reg.DepositId)
	if rec.Status != "MINT_FAILED" {
		t.Errorf("expected MINT_FAILED, got %s", rec.Status)
	}
}

func TestRejectDeposit(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	reg, _ := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{RequesterBesuAddress: "0xbank", Amount: "5"})
	if _, err := env.client.RejectDeposit(ctx, &pb.RejectDepositRequest{DepositId: reg.DepositId, Reason: "kyc"}); err != nil {
		t.Fatalf("RejectDeposit: %v", err)
	}
	rec, _, _ := env.escrowRepo.GetDeposit(ctx, reg.DepositId)
	if rec.Status != "REJECTED" || rec.RejectionReason != "kyc" {
		t.Errorf("unexpected record: %+v", rec)
	}
	// Cannot approve a rejected deposit.
	_, err := env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: reg.DepositId})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

// RequestFiatExchange retries a mint for a MINT_FAILED deposit.
func TestRequestFiatExchange_RetriesMint(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	reg, _ := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{RequesterBesuAddress: "0xbank", Amount: "5"})
	rec, _, _ := env.escrowRepo.GetDeposit(ctx, reg.DepositId)
	rec.Status = "APPROVED"
	_ = env.escrowRepo.UpdateDeposit(ctx, rec)

	resp, err := env.client.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: reg.DepositId})
	if err != nil {
		t.Fatalf("RequestFiatExchange: %v", err)
	}
	if resp.MintTxHash == "" {
		t.Error("expected mint tx hash")
	}
	// Second call must fail with AlreadyExists.
	_, err = env.client.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: reg.DepositId})
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", err)
	}
}

// --- Redeem lifecycle ---

func TestRedeemLifecycle(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()

	req, err := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{RequesterBesuAddress: "0xbank", Amount: "200"})
	if err != nil {
		t.Fatalf("RequestRedeem: %v", err)
	}
	ap, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: req.RedeemId})
	if err != nil {
		t.Fatalf("ApproveRedeem: %v", err)
	}
	if ap.MintTxHash != "mock-mint-tx" {
		t.Errorf("expected mint tx, got %q", ap.MintTxHash)
	}
	list, _ := env.client.ListRedeems(ctx, &pb.ListRedeemsRequest{})
	if len(list.Redeems) != 1 || list.Redeems[0].Status != pb.RedeemStatus_REDEEM_STATUS_APPROVED {
		t.Errorf("unexpected redeems: %+v", list.Redeems)
	}
}

func TestRejectRedeem(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	req, _ := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{RequesterBesuAddress: "0xbank", Amount: "1"})
	if _, err := env.client.RejectRedeem(ctx, &pb.RejectRedeemRequest{RedeemId: req.RedeemId, Reason: "no"}); err != nil {
		t.Fatalf("RejectRedeem: %v", err)
	}
	_, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: req.RedeemId})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

// --- Escrow (tokenization) lifecycle: fCeBM → tCeBM, burn then mint ---

func TestEscrowLifecycle_BurnThenMint(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()

	req, err := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress: "0xbank", Amount: "300", DepositId: "dep-1",
	})
	if err != nil {
		t.Fatalf("RequestEscrow: %v", err)
	}
	ap, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: req.EscrowId})
	if err != nil {
		t.Fatalf("ApproveEscrow: %v", err)
	}
	// Both legs must complete: burn (fCeBM) AND mint (tCeBM). No partial settlement.
	if ap.BurnTxHash != "mock-fiat-burn-tx" || ap.MintTxHash != "mock-mint-tx" {
		t.Errorf("expected both burn and mint tx hashes, got burn=%q mint=%q", ap.BurnTxHash, ap.MintTxHash)
	}
	rec, _, _ := env.escrowRepo.GetEscrow(ctx, req.EscrowId)
	if rec.Status != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", rec.Status)
	}
}

func TestRequestEscrow_Validation(t *testing.T) {
	env := newFlowEnv(t)
	_, err := env.client.RequestEscrow(context.Background(), &pb.RequestEscrowRequest{Amount: "1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

// A burn failure aborts the escrow before mint — no tCeBM is created. Atomicity.
func TestApproveEscrow_BurnFailureNoMint(t *testing.T) {
	escrowRepo := newFakeEscrowRepo()
	var fiat = errFiat{err: errors.New("burn revert")}
	env := setupFlowEnv(t, server.Config{
		Token:      &mockToken{balance: "1"},
		Fiat:       fiat,
		EscrowRepo: escrowRepo,
	})
	env.escrowRepo = escrowRepo
	ctx := context.Background()
	req, _ := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress: "0xbank", Amount: "1", DepositId: "d",
	})
	_, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: req.EscrowId})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal on burn failure, got %v", err)
	}
	rec, _, _ := escrowRepo.GetEscrow(ctx, req.EscrowId)
	if rec.Status == "APPROVED" {
		t.Error("escrow must not be APPROVED when burn fails")
	}
}

func TestRejectEscrow(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	req, _ := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress: "0xbank", Amount: "1", DepositId: "d",
	})
	if _, err := env.client.RejectEscrow(ctx, &pb.RejectEscrowRequest{EscrowId: req.EscrowId, Reason: "x"}); err != nil {
		t.Fatalf("RejectEscrow: %v", err)
	}
	_, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: req.EscrowId})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

func TestListEscrows(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	_, _ = env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{RequesterBesuAddress: "0xb", Amount: "1", DepositId: "d"})
	list, err := env.client.ListEscrows(ctx, &pb.ListEscrowsRequest{})
	if err != nil {
		t.Fatalf("ListEscrows: %v", err)
	}
	if len(list.Escrows) != 1 {
		t.Errorf("expected 1 escrow, got %d", len(list.Escrows))
	}
}
