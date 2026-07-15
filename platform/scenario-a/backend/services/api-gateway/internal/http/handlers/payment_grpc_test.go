// SPDX-License-Identifier: Apache-2.0

// This file exercises the PaymentHandler endpoints against an in-process fake
// payment-orchestrator gRPC server (loopback TCP, no live backend). It covers
// happy paths, validation 4xx branches, and gRPC error→HTTP mapping.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	besuscanner "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/besu"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakePaymentServer is a configurable fake of the payment-orchestrator gRPC
// service. Each method returns its configured response/error.
type fakePaymentServer struct {
	pb.UnimplementedPaymentOrchestratorServiceServer
	err error // when set, every method returns this error

	htlcLock     *pb.LockHTLCResponse
	htlcSettle   *pb.SettleHTLCResponse
	htlcRefund   *pb.RefundHTLCResponse
	htlcStatus   *pb.GetHTLCStatusResponse
	htlcSearch   *pb.SearchHTLCResponse
	token        *pb.MintTokenResponse
	balance      *pb.GetBalanceResponse
	fiatBalance  *pb.GetFiatBalanceResponse
	deposit      *pb.RegisterDepositResponse
	fiatExchange *pb.RequestFiatExchangeResponse
	deposits     *pb.ListDepositsResponse
	escrow       *pb.RequestEscrowResponse
	approveEsc   *pb.ApproveEscrowResponse
	escrows      *pb.ListEscrowsResponse
	redeem       *pb.RequestRedeemResponse
	approveRed   *pb.ApproveRedeemResponse
	redeems      *pb.ListRedeemsResponse
	zeto         *pb.InitiateZetoTransferResponse
	fxPropose    *pb.ProposeFXAgreementResponse
	fxAccept     *pb.AcceptFXAgreementResponse
	fxReject     *pb.RejectFXAgreementResponse
	fxCancel     *pb.CancelFXAgreementResponse
	fxSettle     *pb.SettleFXAgreementResponse
	fxGet        *pb.GetFXAgreementResponse
	fxList       *pb.ListFXAgreementsResponse
	fxEvents     *pb.ListFXAgreementEventsResponse
}

func (s *fakePaymentServer) LockHTLC(context.Context, *pb.LockHTLCRequest) (*pb.LockHTLCResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.htlcLock, nil
}
func (s *fakePaymentServer) LockHTLCWithHashLock(context.Context, *pb.LockHTLCWithHashLockRequest) (*pb.LockHTLCWithHashLockResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.LockHTLCWithHashLockResponse{
		ContractId: s.htlcLock.GetContractId(),
		HashLock:   s.htlcLock.GetHashLock(),
	}, nil
}
func (s *fakePaymentServer) SettleHTLC(context.Context, *pb.SettleHTLCRequest) (*pb.SettleHTLCResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.htlcSettle, nil
}
func (s *fakePaymentServer) RefundHTLC(context.Context, *pb.RefundHTLCRequest) (*pb.RefundHTLCResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.htlcRefund, nil
}
func (s *fakePaymentServer) GetHTLCStatus(context.Context, *pb.GetHTLCStatusRequest) (*pb.GetHTLCStatusResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.htlcStatus, nil
}
func (s *fakePaymentServer) SearchHTLC(context.Context, *pb.SearchHTLCRequest) (*pb.SearchHTLCResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.htlcSearch, nil
}
func (s *fakePaymentServer) MintToken(context.Context, *pb.MintTokenRequest) (*pb.MintTokenResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.token, nil
}
func (s *fakePaymentServer) BurnToken(context.Context, *pb.BurnTokenRequest) (*pb.BurnTokenResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.BurnTokenResponse{TxHash: s.token.GetTxHash()}, nil
}
func (s *fakePaymentServer) TransferToken(context.Context, *pb.TransferTokenRequest) (*pb.TransferTokenResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.TransferTokenResponse{TxHash: s.token.GetTxHash()}, nil
}
func (s *fakePaymentServer) GetBalance(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.balance, nil
}
func (s *fakePaymentServer) GetFiatBalance(context.Context, *pb.GetFiatBalanceRequest) (*pb.GetFiatBalanceResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fiatBalance, nil
}
func (s *fakePaymentServer) RegisterDeposit(context.Context, *pb.RegisterDepositRequest) (*pb.RegisterDepositResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.deposit, nil
}
func (s *fakePaymentServer) ApproveDeposit(context.Context, *pb.ApproveDepositRequest) (*pb.ApproveDepositResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ApproveDepositResponse{}, nil
}
func (s *fakePaymentServer) RejectDeposit(context.Context, *pb.RejectDepositRequest) (*pb.RejectDepositResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.RejectDepositResponse{}, nil
}
func (s *fakePaymentServer) RequestFiatExchange(context.Context, *pb.RequestFiatExchangeRequest) (*pb.RequestFiatExchangeResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fiatExchange, nil
}
func (s *fakePaymentServer) ListDeposits(context.Context, *pb.ListDepositsRequest) (*pb.ListDepositsResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.deposits, nil
}
func (s *fakePaymentServer) RequestEscrow(context.Context, *pb.RequestEscrowRequest) (*pb.RequestEscrowResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.escrow, nil
}
func (s *fakePaymentServer) ApproveEscrow(context.Context, *pb.ApproveEscrowRequest) (*pb.ApproveEscrowResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.approveEsc, nil
}
func (s *fakePaymentServer) RejectEscrow(context.Context, *pb.RejectEscrowRequest) (*pb.RejectEscrowResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.RejectEscrowResponse{}, nil
}
func (s *fakePaymentServer) ListEscrows(context.Context, *pb.ListEscrowsRequest) (*pb.ListEscrowsResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.escrows, nil
}
func (s *fakePaymentServer) RequestRedeem(context.Context, *pb.RequestRedeemRequest) (*pb.RequestRedeemResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.redeem, nil
}
func (s *fakePaymentServer) ApproveRedeem(context.Context, *pb.ApproveRedeemRequest) (*pb.ApproveRedeemResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.approveRed, nil
}
func (s *fakePaymentServer) RejectRedeem(context.Context, *pb.RejectRedeemRequest) (*pb.RejectRedeemResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.RejectRedeemResponse{}, nil
}
func (s *fakePaymentServer) ListRedeems(context.Context, *pb.ListRedeemsRequest) (*pb.ListRedeemsResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.redeems, nil
}
func (s *fakePaymentServer) InitiateZetoTransfer(context.Context, *pb.InitiateZetoTransferRequest) (*pb.InitiateZetoTransferResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.zeto, nil
}
func (s *fakePaymentServer) ProposeFXAgreement(context.Context, *pb.ProposeFXAgreementRequest) (*pb.ProposeFXAgreementResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxPropose, nil
}
func (s *fakePaymentServer) AcceptFXAgreement(context.Context, *pb.AcceptFXAgreementRequest) (*pb.AcceptFXAgreementResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxAccept, nil
}
func (s *fakePaymentServer) RejectFXAgreement(context.Context, *pb.RejectFXAgreementRequest) (*pb.RejectFXAgreementResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxReject, nil
}
func (s *fakePaymentServer) CancelFXAgreement(context.Context, *pb.CancelFXAgreementRequest) (*pb.CancelFXAgreementResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxCancel, nil
}
func (s *fakePaymentServer) SettleFXAgreement(context.Context, *pb.SettleFXAgreementRequest) (*pb.SettleFXAgreementResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxSettle, nil
}
func (s *fakePaymentServer) GetFXAgreement(context.Context, *pb.GetFXAgreementRequest) (*pb.GetFXAgreementResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxGet, nil
}
func (s *fakePaymentServer) ListFXAgreements(context.Context, *pb.ListFXAgreementsRequest) (*pb.ListFXAgreementsResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxList, nil
}
func (s *fakePaymentServer) ListFXAgreementEvents(context.Context, *pb.ListFXAgreementEventsRequest) (*pb.ListFXAgreementEventsResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fxEvents, nil
}

// startFakePaymentBackend spins up a loopback gRPC server serving fake and
// returns a PaymentHandler wired to a real gRPC adapter dialing it.
func startFakePaymentBackend(t *testing.T, fake *fakePaymentServer) *PaymentHandler {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterPaymentOrchestratorServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })

	adapter, err := paymentadapter.NewGRPCAdapter(lis.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial adapter: %v", err)
	}
	t.Cleanup(func() { _ = adapter.Close() })
	return NewPaymentHandler(adapter, "bank-a")
}

// authedClaims injects valid token claims so authenticated handlers proceed.
func authedClaims(bankID string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "user-1", BankID: bankID})
		return c.Next()
	}
}

func postJSON(t *testing.T, app *fiber.App, path string, payload any) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	return resp
}

func TestPaymentHTLCEndpoints_HappyPaths(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		htlcLock:   &pb.LockHTLCResponse{ContractId: "c1", HashLock: "h1"},
		htlcSettle: &pb.SettleHTLCResponse{HtlcTxHash: "tx"},
		htlcRefund: &pb.RefundHTLCResponse{HtlcTxHash: "tx"},
	}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Post("/lock", h.LockHTLC)
	app.Post("/lock-hash", h.LockHTLCWithHashLock)
	app.Post("/settle", h.SettleHTLC)
	app.Post("/refund", h.RefundHTLC)

	cases := []struct {
		path string
		body map[string]any
		want int
	}{
		{"/lock", map[string]any{"receiver": "bank-b", "amount": "100"}, http.StatusCreated},
		{"/lock-hash", map[string]any{"receiver": "bank-b", "amount": "100", "hash_lock": "abc"}, http.StatusCreated},
		{"/settle", map[string]any{"contract_id": "c1", "secret": "s"}, http.StatusOK},
		{"/refund", map[string]any{"contract_id": "c1"}, http.StatusOK},
	}
	for _, tc := range cases {
		resp := postJSON(t, app, tc.path, tc.body)
		if resp.StatusCode != tc.want {
			t.Errorf("%s: want %d, got %d", tc.path, tc.want, resp.StatusCode)
		}
	}
}

func TestPaymentHTLCEndpoints_DownstreamError(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{err: status.Error(codes.Internal, "boom")}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Post("/lock", h.LockHTLC)
	app.Post("/settle", h.SettleHTLC)
	app.Post("/refund", h.RefundHTLC)

	for _, path := range []string{"/lock", "/settle", "/refund"} {
		body := map[string]any{"receiver": "bank-b", "amount": "1", "contract_id": "c1"}
		resp := postJSON(t, app, path, body)
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("%s: want 500, got %d", path, resp.StatusCode)
		}
	}
}

func TestPaymentHTLC_ValidationErrors(t *testing.T) {
	t.Parallel()
	h := NewPaymentHandler(nil, "bank-a")
	app := fiber.New()
	app.Post("/lock", h.LockHTLC)
	app.Post("/lock-hash", h.LockHTLCWithHashLock)

	// missing receiver/amount
	if resp := postJSON(t, app, "/lock", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("lock missing fields: want 400, got %d", resp.StatusCode)
	}
	// missing hash_lock
	if resp := postJSON(t, app, "/lock-hash", map[string]any{"receiver": "b", "amount": "1"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("lock-hash missing hash: want 400, got %d", resp.StatusCode)
	}
	// invalid JSON body
	req := httptest.NewRequest(http.MethodPost, "/lock", bytes.NewReader([]byte("{")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("lock invalid json: want 400, got %d", resp.StatusCode)
	}
}

func TestGetHTLCStatus(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		htlcStatus: &pb.GetHTLCStatusResponse{Lock: &pb.HTLCLock{
			ContractId: "c1",
			Sender:     "op@spoke-a-bank-a",
			Receiver:   "op@spoke-b-bank-b",
			State:      pb.HTLCState_HTLC_STATE_LOCKED,
		}},
	}
	h := startFakePaymentBackend(t, fake)

	// Authorized counterparty (bank-a) → 200.
	app := fiber.New()
	app.Get("/status/:contractId", authedClaims("bank-a"), h.GetHTLCStatus)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/status/c1", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorized: want 200, got %d", resp.StatusCode)
	}

	// Non-counterparty (bank-z) → 403.
	app2 := fiber.New()
	app2.Get("/status/:contractId", authedClaims("bank-z"), h.GetHTLCStatus)
	resp2, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/status/c1", nil))
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("non-counterparty: want 403, got %d", resp2.StatusCode)
	}

	// No claims → 401.
	app3 := fiber.New()
	app3.Get("/status/:contractId", h.GetHTLCStatus)
	resp3, _ := app3.Test(httptest.NewRequest(http.MethodGet, "/status/c1", nil))
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no claims: want 401, got %d", resp3.StatusCode)
	}
}

func TestGetHTLCStatus_MissingContractID(t *testing.T) {
	t.Parallel()
	h := NewPaymentHandler(nil, "bank-a")
	app := fiber.New()
	app.Get("/status/", authedClaims("bank-a"), h.GetHTLCStatus)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/status/", nil))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestGetHTLCStatus_PermissionDenied(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{err: status.Error(codes.PermissionDenied, "denied")}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Get("/status/:contractId", authedClaims("bank-a"), h.GetHTLCStatus)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/status/c1", nil))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestSearchHTLC(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		htlcSearch: &pb.SearchHTLCResponse{Locks: []*pb.HTLCLock{
			{ContractId: "c1", Sender: "op@spoke-a-bank-a", Receiver: "op@spoke-b-bank-b"},
			{ContractId: "c2", Sender: "op@spoke-c-bank-c", Receiver: "op@spoke-d-bank-d"},
		}},
	}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Get("/search", authedClaims("bank-a"), h.SearchHTLC)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/search", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Total int `json:"total"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Total != 1 {
		t.Fatalf("expected 1 filtered lock, got %d", body.Total)
	}

	// No claims → 401.
	app2 := fiber.New()
	app2.Get("/search", h.SearchHTLC)
	resp2, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/search", nil))
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no claims: want 401, got %d", resp2.StatusCode)
	}
}

func TestTokenEndpoints(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		token:       &pb.MintTokenResponse{TxHash: "tx"},
		balance:     &pb.GetBalanceResponse{Balance: "500"},
		fiatBalance: &pb.GetFiatBalanceResponse{Balance: "750"},
	}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Post("/mint", h.MintToken)
	app.Post("/burn", h.BurnToken)
	app.Post("/transfer", h.TransferToken)
	app.Get("/balance", h.GetBalance)
	app.Get("/fiat-balance", h.GetFiatBalance)

	if resp := postJSON(t, app, "/mint", map[string]any{"to": "x", "amount": "1"}); resp.StatusCode != http.StatusCreated {
		t.Errorf("mint: want 201, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/burn", map[string]any{"from": "x", "amount": "1"}); resp.StatusCode != http.StatusCreated {
		t.Errorf("burn: want 201, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/transfer", map[string]any{"to": "x", "amount": "1"}); resp.StatusCode != http.StatusOK {
		t.Errorf("transfer: want 200, got %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/balance", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("balance: want 200, got %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/fiat-balance", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("fiat-balance: want 200, got %d", resp.StatusCode)
	}
	// burn validation
	if resp := postJSON(t, app, "/burn", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("burn missing fields: want 400, got %d", resp.StatusCode)
	}
}

func TestGetFiatBalance_Unavailable(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{err: status.Error(codes.Unavailable, "down")}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Get("/fiat-balance", h.GetFiatBalance)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/fiat-balance", nil))
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
}

func TestEscrowDepositRedeemEndpoints(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		deposit:      &pb.RegisterDepositResponse{DepositId: "d1"},
		fiatExchange: &pb.RequestFiatExchangeResponse{MintTxHash: "tx"},
		deposits:     &pb.ListDepositsResponse{Deposits: []*pb.DepositRecord{{Id: "d1"}}},
		escrow:       &pb.RequestEscrowResponse{EscrowId: "e1"},
		approveEsc:   &pb.ApproveEscrowResponse{BurnTxHash: "b", MintTxHash: "m"},
		escrows:      &pb.ListEscrowsResponse{Escrows: []*pb.EscrowRecord{{Id: "e1"}}},
		redeem:       &pb.RequestRedeemResponse{RedeemId: "r1"},
		approveRed:   &pb.ApproveRedeemResponse{FiatMintTxHash: "f"},
		redeems:      &pb.ListRedeemsResponse{Redeems: []*pb.RedeemRecord{{Id: "r1"}}},
	}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Post("/deposits/register", h.RegisterDeposit)
	app.Post("/deposits/approve", h.ApproveDeposit)
	app.Post("/deposits/reject", h.RejectDeposit)
	app.Post("/deposits/fiat-exchange", h.RequestFiatExchange)
	app.Get("/deposits", h.ListDeposits)
	app.Post("/escrows/request", h.RequestEscrow)
	app.Post("/escrows/approve", h.ApproveEscrow)
	app.Post("/escrows/reject", h.RejectEscrow)
	app.Get("/escrows", h.ListEscrows)
	app.Post("/redeems/request", h.RequestRedeem)
	app.Post("/redeems/approve", h.ApproveRedeem)
	app.Post("/redeems/reject", h.RejectRedeem)
	app.Get("/redeems", h.ListRedeems)

	posts := []struct {
		path string
		body map[string]any
		want int
	}{
		{"/deposits/register", map[string]any{"amount": "1"}, http.StatusCreated},
		{"/deposits/approve", map[string]any{"deposit_id": "d1"}, http.StatusCreated},
		{"/deposits/reject", map[string]any{"deposit_id": "d1", "reason": "x"}, http.StatusOK},
		{"/deposits/fiat-exchange", map[string]any{"deposit_id": "d1"}, http.StatusCreated},
		{"/escrows/request", map[string]any{"amount": "1"}, http.StatusCreated},
		{"/escrows/approve", map[string]any{"escrow_id": "e1"}, http.StatusCreated},
		{"/escrows/reject", map[string]any{"escrow_id": "e1", "reason": "x"}, http.StatusOK},
		{"/redeems/request", map[string]any{"amount": "1"}, http.StatusCreated},
		{"/redeems/approve", map[string]any{"redeem_id": "r1"}, http.StatusCreated},
		{"/redeems/reject", map[string]any{"redeem_id": "r1", "reason": "x"}, http.StatusOK},
	}
	for _, tc := range posts {
		if resp := postJSON(t, app, tc.path, tc.body); resp.StatusCode != tc.want {
			t.Errorf("%s: want %d, got %d", tc.path, tc.want, resp.StatusCode)
		}
	}
	for _, p := range []string{"/deposits", "/escrows", "/redeems"} {
		if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, p, nil)); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", p, resp.StatusCode)
		}
	}
}

func TestFXAgreementEndpoints(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		fxPropose: &pb.ProposeFXAgreementResponse{TradeId: "t1", TxHash: "tx"},
		fxAccept:  &pb.AcceptFXAgreementResponse{TxHash: "tx"},
		fxReject:  &pb.RejectFXAgreementResponse{TxHash: "tx"},
		fxCancel:  &pb.CancelFXAgreementResponse{TxHash: "tx"},
		fxSettle:  &pb.SettleFXAgreementResponse{TxHash: "tx"},
		fxGet:     &pb.GetFXAgreementResponse{Agreement: &pb.FXAgreement{TradeId: "t1"}},
		fxList:    &pb.ListFXAgreementsResponse{Agreements: []*pb.FXAgreement{{TradeId: "t1"}}},
		fxEvents:  &pb.ListFXAgreementEventsResponse{Events: []*pb.FXAgreementEvent{{Id: 1, TradeId: "t1"}}},
	}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	// Accept and Reject require authenticated claims to enforce originator-cannot-self-accept.
	app.Use(authedClaims("bank-b"))
	app.Post("/fx", h.ProposeFXAgreement)
	app.Post("/fx/:tradeId/accept", h.AcceptFXAgreement)
	app.Post("/fx/:tradeId/reject", h.RejectFXAgreement)
	app.Post("/fx/:tradeId/cancel", h.CancelFXAgreement)
	app.Post("/fx/:tradeId/settle", h.SettleFXAgreement)
	app.Get("/fx/:tradeId", h.GetFXAgreement)
	app.Get("/fx/:tradeId/audit", h.ListFXAgreementEvents)
	app.Get("/fx", h.ListFXAgreements)

	propose := map[string]any{
		"counterparty_b": "bank-b", "origin_amount": "1", "counter_amount": "2",
		"origin_currency": "BRL", "counter_currency": "ARS", "rate": "2", "expiry_date": 99,
	}
	if resp := postJSON(t, app, "/fx", propose); resp.StatusCode != http.StatusCreated {
		t.Errorf("propose: want 201, got %d", resp.StatusCode)
	}
	// propose validation
	if resp := postJSON(t, app, "/fx", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("propose missing fields: want 400, got %d", resp.StatusCode)
	}
	for _, p := range []string{"/fx/t1/accept", "/fx/t1/reject", "/fx/t1/cancel", "/fx/t1/settle"} {
		if resp := postJSON(t, app, p, map[string]any{}); resp.StatusCode != http.StatusOK {
			t.Errorf("%s: want 200, got %d", p, resp.StatusCode)
		}
	}
	for _, p := range []string{"/fx/t1", "/fx/t1/audit", "/fx"} {
		if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, p, nil)); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", p, resp.StatusCode)
		}
	}

	// Accept/Reject without claims → 401.
	appNoClaims := fiber.New()
	appNoClaims.Post("/fx/:tradeId/accept", h.AcceptFXAgreement)
	appNoClaims.Post("/fx/:tradeId/reject", h.RejectFXAgreement)
	for _, p := range []string{"/fx/t1/accept", "/fx/t1/reject"} {
		if resp := postJSON(t, appNoClaims, p, map[string]any{}); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("no-claims %s: want 401, got %d", p, resp.StatusCode)
		}
	}
}

func TestProposeFXAgreement_RosterValidation(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{fxPropose: &pb.ProposeFXAgreementResponse{TradeId: "t1", TxHash: "tx"}}
	h := startFakePaymentBackend(t, fake).
		WithIdentityRoster(NewIdentityRoster([]string{"alice@spoke-a-bank-a", "bob@spoke-b-bank-b"}, nil))
	app := fiber.New()
	app.Use(authedClaims("bank-a"))
	app.Post("/fx", h.ProposeFXAgreement)

	terms := func(counterparty, beneficiary string) map[string]any {
		return map[string]any{
			"counterparty_b": counterparty, "beneficiary": beneficiary,
			"origin_amount": "1", "counter_amount": "2",
			"origin_currency": "BRL", "counter_currency": "ARS", "rate": "2", "expiry_date": 99,
		}
	}

	// Off-roster identity → 400 before the on-chain propose, listing the offender.
	resp := postJSON(t, app, "/fx", terms("carol@spoke-x-bank-c", "alice@spoke-a-bank-a"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("off-roster propose: want 400, got %d", resp.StatusCode)
	}
	var body struct {
		InvalidIdentities []string `json:"invalid_identities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	_ = resp.Body.Close()
	if len(body.InvalidIdentities) != 1 || body.InvalidIdentities[0] != "carol@spoke-x-bank-c" {
		t.Errorf("invalid_identities = %v, want [carol@spoke-x-bank-c]", body.InvalidIdentities)
	}

	// All parties on the roster → reaches the backend and returns 201.
	if resp := postJSON(t, app, "/fx", terms("alice@spoke-a-bank-a", "bob@spoke-b-bank-b")); resp.StatusCode != http.StatusCreated {
		t.Errorf("on-roster propose: want 201, got %d", resp.StatusCode)
	}
}

// flakyRosterProvider errors a fixed number of times before returning identities,
// to exercise the fail-closed retry-until-resolved path.
type flakyRosterProvider struct {
	failsLeft  int
	identities []string
}

func (p *flakyRosterProvider) ListParticipantIdentities(context.Context) ([]string, error) {
	if p.failsLeft > 0 {
		p.failsLeft--
		return nil, errors.New("orchestrator unavailable")
	}
	return p.identities, nil
}

func fxTerms(counterparty string) map[string]any {
	return map[string]any{
		"counterparty_b": counterparty,
		"origin_amount":  "1", "counter_amount": "2",
		"origin_currency": "BRL", "counter_currency": "ARS", "rate": "2", "expiry_date": 99,
	}
}

func TestProposeFXAgreement_FailsClosedWhenRosterUnavailable(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{fxPropose: &pb.ProposeFXAgreementResponse{TradeId: "t1", TxHash: "tx"}}
	h := startFakePaymentBackend(t, fake).
		WithIdentityRoster(NewIdentityRoster(nil, &fakeRosterProvider{err: errors.New("down")}))
	h.rosterRetryInitial = time.Millisecond
	h.rosterRetryMax = 30 * time.Millisecond // keep the test fast; still bounded fail-closed
	app := fiber.New()
	app.Use(authedClaims("bank-a"))
	app.Post("/fx", h.ProposeFXAgreement)

	// Roster never resolves → propose must be blocked (503), not proposed on-chain.
	if resp := postJSON(t, app, "/fx", fxTerms("alice@spoke-a-bank-a")); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unresolved roster: want 503, got %d", resp.StatusCode)
	}
}

func TestProposeFXAgreement_RetriesUntilRosterResolves(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{fxPropose: &pb.ProposeFXAgreementResponse{TradeId: "t1", TxHash: "tx"}}
	prov := &flakyRosterProvider{failsLeft: 2, identities: []string{"alice@spoke-a-bank-a"}}
	h := startFakePaymentBackend(t, fake).WithIdentityRoster(NewIdentityRoster(nil, prov))
	h.rosterRetryInitial = time.Millisecond
	app := fiber.New()
	app.Use(authedClaims("bank-a"))
	app.Post("/fx", h.ProposeFXAgreement)

	// Two transient failures, then the roster resolves and the (valid) party passes.
	if resp := postJSON(t, app, "/fx", fxTerms("alice@spoke-a-bank-a")); resp.StatusCode != http.StatusCreated {
		t.Fatalf("retry-then-resolve: want 201, got %d", resp.StatusCode)
	}
	if prov.failsLeft != 0 {
		t.Errorf("expected all transient failures consumed, %d left", prov.failsLeft)
	}
}

func TestFXAgreement_ErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code codes.Code
		want int
	}{
		{codes.NotFound, http.StatusNotFound},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.FailedPrecondition, http.StatusConflict},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.Internal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		fake := &fakePaymentServer{err: status.Error(tc.code, "x")}
		h := startFakePaymentBackend(t, fake)
		app := fiber.New()
		app.Get("/fx/:tradeId", h.GetFXAgreement)
		resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/fx/t1", nil))
		if resp.StatusCode != tc.want {
			t.Errorf("code %v: want %d, got %d", tc.code, tc.want, resp.StatusCode)
		}
	}
}

func TestFXAgreement_MissingTradeID(t *testing.T) {
	t.Parallel()
	h := NewPaymentHandler(nil, "bank-a")
	app := fiber.New()
	app.Get("/fx/", h.GetFXAgreement)
	app.Post("/fx//accept", h.AcceptFXAgreement)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/fx/", nil)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("get missing tradeId: want 400, got %d", resp.StatusCode)
	}
}

// fakeParticipantResolver is a configurable stand-in for the compliance adapter,
// used to test requester-name enrichment without a live compliance backend.
type fakeParticipantResolver struct {
	participants []complianceadapter.Participant
	err          error
}

func (f *fakeParticipantResolver) ListParticipants(_ context.Context, _, _ string) ([]complianceadapter.Participant, error) {
	return f.participants, f.err
}

// requesterNameOf pulls the requester_name of the first record out of a list
// response shaped as {"<key>": [ {..., "requester_name": "..."} ]}.
func requesterNameOf(t *testing.T, resp *http.Response, key string) string {
	t.Helper()
	var body map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s: %v", key, err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body[key], &rows); err != nil {
		t.Fatalf("decode %s rows: %v", key, err)
	}
	if len(rows) == 0 {
		t.Fatalf("%s: expected at least one row", key)
	}
	name, _ := rows[0]["requester_name"].(string)
	return name
}

func TestListRecords_RequesterNameEnrichment(t *testing.T) {
	t.Parallel()
	// The record's on-chain address is stored checksummed; the participant
	// wallet_address is lowercase. Enrichment must match case-insensitively.
	fake := &fakePaymentServer{
		deposits: &pb.ListDepositsResponse{Deposits: []*pb.DepositRecord{{Id: "d1", RequesterBesuAddress: "0xAbC123"}}},
		escrows:  &pb.ListEscrowsResponse{Escrows: []*pb.EscrowRecord{{Id: "e1", RequesterBesuAddress: "0xAbC123"}}},
		redeems:  &pb.ListRedeemsResponse{Redeems: []*pb.RedeemRecord{{Id: "r1", RequesterBesuAddress: "0xAbC123"}}},
	}
	h := startFakePaymentBackend(t, fake)
	h = h.WithParticipantResolver(&fakeParticipantResolver{
		participants: []complianceadapter.Participant{
			{WalletAddress: "0xabc123", InstitutionName: "Banco Alpha"},
			{WalletAddress: "0xdef456", InstitutionName: "Banco Beta"},
		},
	})

	app := fiber.New()
	app.Get("/deposits", h.ListDeposits)
	app.Get("/escrows", h.ListEscrows)
	app.Get("/redeems", h.ListRedeems)

	cases := []struct{ path, key string }{
		{"/deposits", "deposits"},
		{"/escrows", "escrows"},
		{"/redeems", "redeems"},
	}
	for _, tc := range cases {
		resp, _ := app.Test(httptest.NewRequest(http.MethodGet, tc.path, nil))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: want 200, got %d", tc.path, resp.StatusCode)
		}
		if got := requesterNameOf(t, resp, tc.key); got != "Banco Alpha" {
			t.Errorf("GET %s: requester_name = %q, want %q", tc.path, got, "Banco Alpha")
		}
	}
}

func TestListRecords_RequesterNameUnresolved(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		deposits: &pb.ListDepositsResponse{Deposits: []*pb.DepositRecord{{Id: "d1", RequesterBesuAddress: "0xNoMatch"}}},
	}
	h := startFakePaymentBackend(t, fake)
	h = h.WithParticipantResolver(&fakeParticipantResolver{
		participants: []complianceadapter.Participant{{WalletAddress: "0xabc123", InstitutionName: "Banco Alpha"}},
	})

	app := fiber.New()
	app.Get("/deposits", h.ListDeposits)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/deposits", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if got := requesterNameOf(t, resp, "deposits"); got != "" {
		t.Errorf("unmatched wallet: requester_name = %q, want empty", got)
	}
}

func TestListRecords_ResolverAbsentOrFailing(t *testing.T) {
	t.Parallel()
	fake := &fakePaymentServer{
		deposits: &pb.ListDepositsResponse{Deposits: []*pb.DepositRecord{{Id: "d1", RequesterBesuAddress: "0xabc123"}}},
	}

	// No resolver configured: listing still succeeds, name stays empty.
	hNoResolver := startFakePaymentBackend(t, fake)
	appNo := fiber.New()
	appNo.Get("/deposits", hNoResolver.ListDeposits)
	if resp, _ := appNo.Test(httptest.NewRequest(http.MethodGet, "/deposits", nil)); resp.StatusCode != http.StatusOK {
		t.Fatalf("no resolver: want 200, got %d", resp.StatusCode)
	}

	// Resolver error is non-fatal: listing still succeeds, name stays empty.
	hErr := startFakePaymentBackend(t, fake)
	hErr = hErr.WithParticipantResolver(&fakeParticipantResolver{err: status.Error(codes.Unavailable, "down")})
	appErr := fiber.New()
	appErr.Get("/deposits", hErr.ListDeposits)
	resp, _ := appErr.Test(httptest.NewRequest(http.MethodGet, "/deposits", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolver error: want 200, got %d", resp.StatusCode)
	}
	if got := requesterNameOf(t, resp, "deposits"); got != "" {
		t.Errorf("resolver error: requester_name = %q, want empty", got)
	}
}

func TestEnrichScanResultNames(t *testing.T) {
	t.Parallel()
	// Scan results carry checksummed EVM addresses; the name map is keyed lowercase.
	results := []besuscanner.HTLCScanResult{
		{ContractID: "c1", Sender: "0xAbC123", Receiver: "0xDeF456"},
		{ContractID: "c2", Sender: "0x999", Receiver: "0xAbC123"}, // sender unknown, receiver known
	}
	names := map[string]string{
		"0xabc123": "Banco Alpha",
		"0xdef456": "Banco Beta",
	}
	enrichScanResultNames(results, names)

	if results[0].SenderName != "Banco Alpha" || results[0].ReceiverName != "Banco Beta" {
		t.Errorf("row0: got sender=%q receiver=%q", results[0].SenderName, results[0].ReceiverName)
	}
	if results[1].SenderName != "" {
		t.Errorf("row1: unmatched sender should be empty, got %q", results[1].SenderName)
	}
	if results[1].ReceiverName != "Banco Alpha" {
		t.Errorf("row1: got receiver=%q, want %q", results[1].ReceiverName, "Banco Alpha")
	}

	// Nil/empty name map must be a no-op (no panic, names stay empty).
	clean := []besuscanner.HTLCScanResult{{Sender: "0xabc123"}}
	enrichScanResultNames(clean, nil)
	if clean[0].SenderName != "" {
		t.Errorf("nil map: want empty, got %q", clean[0].SenderName)
	}
}

// TestListSettledPvPCredits verifies the central-bank derivation of a receiving
// bank's incoming PvP legs from its aggregated SETTLED FX agreements: the origin
// leg credits SourceReceiver (OriginAmount), the counter leg credits DestReceiver
// (CounterAmount); non-settled agreements and non-receiver banks are excluded.
func TestListSettledPvPCredits(t *testing.T) {
	const settledUnix = int64(1752570000)
	fake := &fakePaymentServer{
		fxList: &pb.ListFXAgreementsResponse{Agreements: []*pb.FXAgreement{
			{
				TradeId:        "trade-1",
				State:          pb.FXAgreementState_FX_STATE_SETTLED,
				Originator:     "op@spoke-a-bank-a",
				Custodian:      "op@spoke-b-bank-d",
				SourceReceiver: "corr@spoke-a-bank-c",
				DestReceiver:   "ben@spoke-b-bank-b",
				OriginAmount:   "700",
				CounterAmount:  "300",
			},
			// Not settled → excluded even though bank-c is the source receiver.
			{
				TradeId:        "trade-2",
				State:          pb.FXAgreementState_FX_STATE_ACCEPTED,
				SourceReceiver: "corr@spoke-a-bank-c",
				OriginAmount:   "999",
			},
		}},
		fxEvents: &pb.ListFXAgreementEventsResponse{Events: []*pb.FXAgreementEvent{
			{Id: 1, TradeId: "trade-1", ToState: pb.FXAgreementState_FX_STATE_SETTLED, OccurredAtUnix: settledUnix},
		}},
	}
	h := startFakePaymentBackend(t, fake)
	app := fiber.New()
	app.Get("/pvp-credits", h.ListSettledPvPCredits)

	fetch := func(bankID string) []PvPCredit {
		t.Helper()
		url := "/pvp-credits"
		if bankID != "" {
			url += "?bank_id=" + bankID
		}
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, url, nil))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("bank %q: status = %d, want 200", bankID, resp.StatusCode)
		}
		var body struct {
			Credits []PvPCredit `json:"credits"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body.Credits
	}

	wantSettledAt := time.Unix(settledUnix, 0).UTC().Format(time.RFC3339)

	// bank-c is the origin-leg receiver → one credit of 700.
	if got := fetch("bank-c"); len(got) != 1 || got[0].Reference != "trade-1" || got[0].Amount != "700" || got[0].SettledAt != wantSettledAt {
		t.Errorf("bank-c credits = %+v, want one trade-1/700/%s", got, wantSettledAt)
	}
	// bank-b is the counter-leg receiver → one credit of 300.
	if got := fetch("bank-b"); len(got) != 1 || got[0].Reference != "trade-1" || got[0].Amount != "300" {
		t.Errorf("bank-b credits = %+v, want one trade-1/300", got)
	}
	// bank-a is only a sender → no credit.
	if got := fetch("bank-a"); len(got) != 0 {
		t.Errorf("bank-a credits = %+v, want none", got)
	}

	// Missing bank_id → 400.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/pvp-credits", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing bank_id: status = %d, want 400", resp.StatusCode)
	}
}
