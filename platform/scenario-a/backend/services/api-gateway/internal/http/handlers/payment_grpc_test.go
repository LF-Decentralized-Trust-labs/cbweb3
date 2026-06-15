// SPDX-License-Identifier: Apache-2.0

// This file exercises the PaymentHandler endpoints against an in-process fake
// payment-orchestrator gRPC server (loopback TCP, no live backend). It covers
// happy paths, validation 4xx branches, and gRPC error→HTTP mapping.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
