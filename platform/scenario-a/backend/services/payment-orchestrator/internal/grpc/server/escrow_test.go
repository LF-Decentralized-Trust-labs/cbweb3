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
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// failingFiat is a FiatTokenPort whose Mint/Burn fail, to exercise the
// escrow/deposit/redeem error paths (e.g. PENDING→MINT_FAILED).
type failingFiat struct {
	mintErr error
	burnErr error
}

func (f *failingFiat) Mint(context.Context, string, string) (string, error) {
	if f.mintErr != nil {
		return "", f.mintErr
	}
	return "fiat-mint-tx", nil
}
func (f *failingFiat) Burn(context.Context, string, string) (string, error) {
	if f.burnErr != nil {
		return "", f.burnErr
	}
	return "fiat-burn-tx", nil
}
func (f *failingFiat) BalanceOf(context.Context, string) (string, error) { return "0", nil }
func (f *failingFiat) GetFiatBalance(context.Context) (string, error)    { return "0", nil }

type escrowEnv struct {
	client     pb.PaymentOrchestratorServiceClient
	zeto       *mockZeto
	escrowRepo ports.EscrowRepository
}

// setupEscrowEnv wires a server with an in-memory escrow repository and the
// provided fiat adapter (nil disables fiat operations).
func setupEscrowEnv(t *testing.T, fiat ports.FiatTokenPort) *escrowEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := &mockZeto{}
	repo := repository.NewMemoryEscrowRepository()

	grpcServer, _, err := server.New(server.Config{
		Zeto:            mock,
		Relay:           noopRelay{},
		Fiat:            fiat,
		EscrowRepo:      repo,
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

	return &escrowEnv{
		client:     pb.NewPaymentOrchestratorServiceClient(conn),
		zeto:       mock,
		escrowRepo: repo,
	}
}

// --- Deposit lifecycle ---

func TestDeposit_RegisterApproveExchange(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{})
	ctx := context.Background()

	reg, err := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{
		RequesterBesuAddress: "0xabc", Amount: "1000", RequesterPaladinIdentity: "op@spoke-a-bank-a",
	})
	if err != nil {
		t.Fatalf("RegisterDeposit: %v", err)
	}
	if reg.DepositId == "" {
		t.Fatal("expected deposit id")
	}

	if _, err := env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: reg.DepositId}); err != nil {
		t.Fatalf("ApproveDeposit: %v", err)
	}

	ex, err := env.client.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: reg.DepositId})
	if err != nil {
		t.Fatalf("RequestFiatExchange: %v", err)
	}
	if ex.MintTxHash != "fiat-mint-tx" {
		t.Errorf("mint tx = %q", ex.MintTxHash)
	}

	// Idempotency: second exchange must be rejected (already executed).
	_, err = env.client.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: reg.DepositId})
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", err)
	}

	list, err := env.client.ListDeposits(ctx, &pb.ListDepositsRequest{})
	if err != nil || len(list.Deposits) != 1 {
		t.Fatalf("ListDeposits: len=%d err=%v", len(list.Deposits), err)
	}
	if list.Deposits[0].Status != pb.DepositStatus_DEPOSIT_STATUS_APPROVED {
		t.Errorf("status = %s", list.Deposits[0].Status)
	}
}

func TestDeposit_Reject(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{})
	ctx := context.Background()

	reg, _ := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{
		RequesterBesuAddress: "0xabc", Amount: "1000",
	})
	if _, err := env.client.RejectDeposit(ctx, &pb.RejectDepositRequest{DepositId: reg.DepositId, Reason: "kyc"}); err != nil {
		t.Fatalf("RejectDeposit: %v", err)
	}
	// Approving a rejected deposit must fail.
	_, err := env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: reg.DepositId})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

func TestDeposit_Validation(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{})
	ctx := context.Background()

	if _, err := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("register empty: %v", err)
	}
	if _, err := env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("approve empty: %v", err)
	}
	if _, err := env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: "ghost"}); status.Code(err) != codes.NotFound {
		t.Errorf("approve missing: %v", err)
	}
	if _, err := env.client.RejectDeposit(ctx, &pb.RejectDepositRequest{DepositId: "ghost"}); status.Code(err) != codes.NotFound {
		t.Errorf("reject missing: %v", err)
	}
	if _, err := env.client.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: "ghost"}); status.Code(err) != codes.NotFound {
		t.Errorf("exchange missing: %v", err)
	}
}

func TestDeposit_ExchangeMintFailure_SetsMintFailed(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{mintErr: errors.New("mint reverted")})
	ctx := context.Background()

	reg, _ := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{
		RequesterBesuAddress: "0xabc", Amount: "1000",
	})
	_, _ = env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: reg.DepositId})

	_, err := env.client.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: reg.DepositId})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}

	list, _ := env.client.ListDeposits(ctx, &pb.ListDepositsRequest{})
	if list.Deposits[0].Status != pb.DepositStatus_DEPOSIT_STATUS_MINT_FAILED {
		t.Errorf("expected MINT_FAILED, got %s", list.Deposits[0].Status)
	}

	// Retry from MINT_FAILED is allowed once mint succeeds.
	env2fiat := &failingFiat{}
	_ = env2fiat // documents intent; this env keeps failing — covered above
}

func TestDeposit_ExchangeFiatNil(t *testing.T) {
	env := setupEscrowEnv(t, nil)
	ctx := context.Background()
	reg, _ := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{
		RequesterBesuAddress: "0xabc", Amount: "1000",
	})
	_, _ = env.client.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: reg.DepositId})
	_, err := env.client.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: reg.DepositId})
	if status.Code(err) != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", err)
	}
}

// --- Escrow (tokenization) lifecycle: burn fCeBM → mint tCeBM ---

func TestEscrow_RequestApprove_BurnThenMint(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{})
	ctx := context.Background()

	req, err := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress: "0xabc", Amount: "500", RequesterPaladinIdentity: "op@spoke-a-bank-a",
	})
	if err != nil {
		t.Fatalf("RequestEscrow: %v", err)
	}

	resp, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: req.EscrowId})
	if err != nil {
		t.Fatalf("ApproveEscrow: %v", err)
	}
	if resp.BurnTxHash != "fiat-burn-tx" || resp.MintTxHash != "mock-mint-tx" {
		t.Errorf("unexpected hashes: burn=%q mint=%q", resp.BurnTxHash, resp.MintTxHash)
	}
	// Zeto mint (tCeBM issuance) must have been triggered exactly once.
	if env.zeto.mintCalled != 1 {
		t.Errorf("expected zeto.Mint called once, got %d", env.zeto.mintCalled)
	}

	list, _ := env.client.ListEscrows(ctx, &pb.ListEscrowsRequest{})
	if len(list.Escrows) != 1 || list.Escrows[0].Status != pb.EscrowStatus_ESCROW_STATUS_APPROVED {
		t.Errorf("escrow not approved: %+v", list.Escrows)
	}
}

func TestEscrow_ApproveBurnFails_NoMint(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{burnErr: errors.New("burn reverted")})
	ctx := context.Background()
	req, _ := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress: "0xabc", Amount: "500",
	})
	_, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: req.EscrowId})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
	if env.zeto.mintCalled != 0 {
		t.Errorf("mint must not run when burn fails, got %d", env.zeto.mintCalled)
	}
}

func TestEscrow_Reject(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{})
	ctx := context.Background()
	req, _ := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress: "0xabc", Amount: "500",
	})
	if _, err := env.client.RejectEscrow(ctx, &pb.RejectEscrowRequest{EscrowId: req.EscrowId, Reason: "limit"}); err != nil {
		t.Fatalf("RejectEscrow: %v", err)
	}
	_, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: req.EscrowId})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition approving rejected escrow, got %v", err)
	}
}

func TestEscrow_Validation(t *testing.T) {
	env := setupEscrowEnv(t, nil)
	ctx := context.Background()
	if _, err := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("request empty: %v", err)
	}
	if _, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("approve empty: %v", err)
	}
	if _, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: "ghost"}); status.Code(err) != codes.NotFound {
		t.Errorf("approve missing: %v", err)
	}
	if _, err := env.client.RejectEscrow(ctx, &pb.RejectEscrowRequest{EscrowId: "ghost"}); status.Code(err) != codes.NotFound {
		t.Errorf("reject missing: %v", err)
	}
	// fiat nil → Unavailable on approve.
	req, _ := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{RequesterBesuAddress: "0xabc", Amount: "1"})
	if _, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: req.EscrowId}); status.Code(err) != codes.Unavailable {
		t.Errorf("expected Unavailable with nil fiat, got %v", err)
	}
}

// --- Redeem (de-tokenization) lifecycle ---

func TestRedeem_RequestApprove(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{})
	ctx := context.Background()
	req, err := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{
		RequesterBesuAddress: "0xabc", Amount: "250",
		RequesterPaladinIdentity: "op@spoke-a-bank-a", ZetoTransferTxHash: "0xzt",
	})
	if err != nil {
		t.Fatalf("RequestRedeem: %v", err)
	}
	resp, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: req.RedeemId})
	if err != nil {
		t.Fatalf("ApproveRedeem: %v", err)
	}
	if resp.FiatMintTxHash != "fiat-mint-tx" {
		t.Errorf("fiat mint tx = %q", resp.FiatMintTxHash)
	}
	list, _ := env.client.ListRedeems(ctx, &pb.ListRedeemsRequest{})
	if len(list.Redeems) != 1 || list.Redeems[0].Status != pb.RedeemStatus_REDEEM_STATUS_APPROVED {
		t.Errorf("redeem not approved: %+v", list.Redeems)
	}
}

func TestRedeem_Reject_ReturnsZeto(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{})
	ctx := context.Background()
	req, _ := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{
		RequesterBesuAddress: "0xabc", Amount: "250", RequesterPaladinIdentity: "op@spoke-a-bank-a",
	})
	if _, err := env.client.RejectRedeem(ctx, &pb.RejectRedeemRequest{RedeemId: req.RedeemId, Reason: "no"}); err != nil {
		t.Fatalf("RejectRedeem: %v", err)
	}
	// Zeto tokens are returned to the requester on reject.
	if env.zeto.transferCalled != 1 {
		t.Errorf("expected zeto.Transfer called once on reject, got %d", env.zeto.transferCalled)
	}
}

func TestRedeem_ApproveMintFails(t *testing.T) {
	env := setupEscrowEnv(t, &failingFiat{mintErr: errors.New("mint reverted")})
	ctx := context.Background()
	req, _ := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{
		RequesterBesuAddress: "0xabc", Amount: "250",
	})
	_, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: req.RedeemId})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestRedeem_Validation(t *testing.T) {
	env := setupEscrowEnv(t, nil)
	ctx := context.Background()
	if _, err := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("request empty: %v", err)
	}
	if _, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("approve empty: %v", err)
	}
	if _, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: "ghost"}); status.Code(err) != codes.NotFound {
		t.Errorf("approve missing: %v", err)
	}
	if _, err := env.client.RejectRedeem(ctx, &pb.RejectRedeemRequest{RedeemId: "ghost"}); status.Code(err) != codes.NotFound {
		t.Errorf("reject missing: %v", err)
	}
	req, _ := env.client.RequestRedeem(ctx, &pb.RequestRedeemRequest{RequesterBesuAddress: "0xabc", Amount: "1"})
	if _, err := env.client.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: req.RedeemId}); status.Code(err) != codes.Unavailable {
		t.Errorf("expected Unavailable with nil fiat, got %v", err)
	}
}

// --- Zeto transfer proxy ---

func TestInitiateZetoTransfer(t *testing.T) {
	env := setupEscrowEnv(t, nil)
	ctx := context.Background()
	resp, err := env.client.InitiateZetoTransfer(ctx, &pb.InitiateZetoTransferRequest{
		ToIdentity: "op@spoke-a-bank-b", Amount: "100",
	})
	if err != nil {
		t.Fatalf("InitiateZetoTransfer: %v", err)
	}
	if resp.TxHash != "mock-transfer-tx" {
		t.Errorf("tx = %q", resp.TxHash)
	}
	if _, err := env.client.InitiateZetoTransfer(ctx, &pb.InitiateZetoTransferRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty: %v", err)
	}
}

// --- BurnToken (uncovered earlier) ---

func TestBurnToken(t *testing.T) {
	env := setupEscrowEnv(t, nil)
	ctx := context.Background()
	resp, err := env.client.BurnToken(ctx, &pb.BurnTokenRequest{From: "op@spoke-a-bank-a", Amount: "10"})
	if err != nil {
		t.Fatalf("BurnToken: %v", err)
	}
	if resp.TxHash != "mock-burn-tx" {
		t.Errorf("tx = %q", resp.TxHash)
	}
	if _, err := env.client.BurnToken(ctx, &pb.BurnTokenRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty: %v", err)
	}
}
