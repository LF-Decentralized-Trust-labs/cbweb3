// SPDX-License-Identifier: Apache-2.0

// This file exercises route-authorization on the mutating payment endpoints
// (R2-H-9 / R2-H-10): SettleHTLC, RefundHTLC, TransferToken, CancelFXAgreement
// and SettleFXAgreement must read the authenticated claims, reject unauthenticated
// callers (401), enforce counterparty ownership, and propagate x-caller-identity to
// the payment-orchestrator so it can enforce the same control server-side.
package handlers

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// recordingPaymentServer captures the x-caller-identity propagated by the gateway
// and returns configured responses/errors, so tests can assert both the
// authorization decision and the identity propagation.
type recordingPaymentServer struct {
	pb.UnimplementedPaymentOrchestratorServiceServer
	mu         sync.Mutex
	lastCaller string
	htlcStatus *pb.GetHTLCStatusResponse
	err        error // when set, every implemented method returns this error
}

func (s *recordingPaymentServer) record(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx)
	if vals := md.Get("x-caller-identity"); len(vals) > 0 {
		s.mu.Lock()
		s.lastCaller = vals[0]
		s.mu.Unlock()
	}
}

func (s *recordingPaymentServer) caller() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCaller
}

func (s *recordingPaymentServer) GetHTLCStatus(ctx context.Context, _ *pb.GetHTLCStatusRequest) (*pb.GetHTLCStatusResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return s.htlcStatus, nil
}

func (s *recordingPaymentServer) SettleHTLC(ctx context.Context, _ *pb.SettleHTLCRequest) (*pb.SettleHTLCResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.SettleHTLCResponse{HtlcTxHash: "tx"}, nil
}

func (s *recordingPaymentServer) RefundHTLC(ctx context.Context, _ *pb.RefundHTLCRequest) (*pb.RefundHTLCResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.RefundHTLCResponse{HtlcTxHash: "tx"}, nil
}

func (s *recordingPaymentServer) TransferToken(ctx context.Context, _ *pb.TransferTokenRequest) (*pb.TransferTokenResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.TransferTokenResponse{TxHash: "tx"}, nil
}

func (s *recordingPaymentServer) CancelFXAgreement(ctx context.Context, _ *pb.CancelFXAgreementRequest) (*pb.CancelFXAgreementResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.CancelFXAgreementResponse{TxHash: "tx"}, nil
}

func (s *recordingPaymentServer) SettleFXAgreement(ctx context.Context, _ *pb.SettleFXAgreementRequest) (*pb.SettleFXAgreementResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.SettleFXAgreementResponse{TxHash: "tx"}, nil
}

// startRecordingBackend spins up a loopback gRPC server serving fake and returns a
// PaymentHandler wired to a real gRPC adapter dialing it (bankCode = "bank-a").
func startRecordingBackend(t *testing.T, fake *recordingPaymentServer) *PaymentHandler {
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

// lockedHTLC is a helper fixture: an HTLC locked between bank-a (sender) and
// bank-b (receiver), so bank-a/bank-b are counterparties and bank-z is not.
func lockedHTLC() *pb.GetHTLCStatusResponse {
	return &pb.GetHTLCStatusResponse{Lock: &pb.HTLCLock{
		ContractId: "c1",
		Sender:     "op@spoke-a-bank-a",
		Receiver:   "op@spoke-b-bank-b",
		State:      pb.HTLCState_HTLC_STATE_LOCKED,
	}}
}

func TestSettleHTLC_Authorization(t *testing.T) {
	t.Parallel()
	fake := &recordingPaymentServer{htlcStatus: lockedHTLC()}
	h := startRecordingBackend(t, fake)

	// Counterparty (bank-a) → 200, and x-caller-identity is propagated.
	app := fiber.New()
	app.Post("/settle", authedClaims("bank-a"), h.SettleHTLC)
	if resp := postJSON(t, app, "/settle", map[string]any{"contract_id": "c1", "secret": "s"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("counterparty: want 200, got %d", resp.StatusCode)
	}
	if got := fake.caller(); got != "bank-a" {
		t.Errorf("x-caller-identity = %q, want bank-a", got)
	}

	// Non-counterparty (bank-z) → 403.
	app2 := fiber.New()
	app2.Post("/settle", authedClaims("bank-z"), h.SettleHTLC)
	if resp := postJSON(t, app2, "/settle", map[string]any{"contract_id": "c1", "secret": "s"}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-counterparty: want 403, got %d", resp.StatusCode)
	}

	// No claims → 401.
	app3 := fiber.New()
	app3.Post("/settle", h.SettleHTLC)
	if resp := postJSON(t, app3, "/settle", map[string]any{"contract_id": "c1", "secret": "s"}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no claims: want 401, got %d", resp.StatusCode)
	}
}

func TestRefundHTLC_Authorization(t *testing.T) {
	t.Parallel()
	fake := &recordingPaymentServer{htlcStatus: lockedHTLC()}
	h := startRecordingBackend(t, fake)

	// Counterparty (bank-b) → 200.
	app := fiber.New()
	app.Post("/refund", authedClaims("bank-b"), h.RefundHTLC)
	if resp := postJSON(t, app, "/refund", map[string]any{"contract_id": "c1"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("counterparty: want 200, got %d", resp.StatusCode)
	}
	if got := fake.caller(); got != "bank-b" {
		t.Errorf("x-caller-identity = %q, want bank-b", got)
	}

	// Non-counterparty (bank-z) → 403.
	app2 := fiber.New()
	app2.Post("/refund", authedClaims("bank-z"), h.RefundHTLC)
	if resp := postJSON(t, app2, "/refund", map[string]any{"contract_id": "c1"}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-counterparty: want 403, got %d", resp.StatusCode)
	}

	// No claims → 401.
	app3 := fiber.New()
	app3.Post("/refund", h.RefundHTLC)
	if resp := postJSON(t, app3, "/refund", map[string]any{"contract_id": "c1"}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no claims: want 401, got %d", resp.StatusCode)
	}
}

func TestTransferToken_Authorization(t *testing.T) {
	t.Parallel()
	fake := &recordingPaymentServer{}
	h := startRecordingBackend(t, fake)

	// Authenticated → 200, and x-caller-identity is propagated.
	app := fiber.New()
	app.Post("/transfer", authedClaims("bank-a"), h.TransferToken)
	if resp := postJSON(t, app, "/transfer", map[string]any{"to": "bank-b", "amount": "1"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated: want 200, got %d", resp.StatusCode)
	}
	if got := fake.caller(); got != "bank-a" {
		t.Errorf("x-caller-identity = %q, want bank-a", got)
	}

	// No claims → 401.
	app2 := fiber.New()
	app2.Post("/transfer", h.TransferToken)
	if resp := postJSON(t, app2, "/transfer", map[string]any{"to": "bank-b", "amount": "1"}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no claims: want 401, got %d", resp.StatusCode)
	}
}

func TestCancelFXAgreement_Authorization(t *testing.T) {
	t.Parallel()

	// Authenticated owner → 200, x-caller-identity propagated for server-side enforcement.
	okFake := &recordingPaymentServer{}
	hOK := startRecordingBackend(t, okFake)
	app := fiber.New()
	app.Post("/fx/:tradeId/cancel", authedClaims("bank-a"), hOK.CancelFXAgreement)
	if resp := postJSON(t, app, "/fx/t1/cancel", map[string]any{}); resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated: want 200, got %d", resp.StatusCode)
	}
	if got := okFake.caller(); got != "bank-a" {
		t.Errorf("x-caller-identity = %q, want bank-a", got)
	}

	// Orchestrator rejects a non-party caller (PermissionDenied) → 403.
	denyFake := &recordingPaymentServer{err: status.Error(codes.PermissionDenied, "not a party")}
	hDeny := startRecordingBackend(t, denyFake)
	app2 := fiber.New()
	app2.Post("/fx/:tradeId/cancel", authedClaims("bank-z"), hDeny.CancelFXAgreement)
	if resp := postJSON(t, app2, "/fx/t1/cancel", map[string]any{}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-party: want 403, got %d", resp.StatusCode)
	}

	// No claims → 401.
	app3 := fiber.New()
	app3.Post("/fx/:tradeId/cancel", hOK.CancelFXAgreement)
	if resp := postJSON(t, app3, "/fx/t1/cancel", map[string]any{}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no claims: want 401, got %d", resp.StatusCode)
	}
}

func TestSettleFXAgreement_Authorization(t *testing.T) {
	t.Parallel()

	// Authenticated owner → 200, x-caller-identity propagated.
	okFake := &recordingPaymentServer{}
	hOK := startRecordingBackend(t, okFake)
	app := fiber.New()
	app.Post("/fx/:tradeId/settle", authedClaims("bank-a"), hOK.SettleFXAgreement)
	if resp := postJSON(t, app, "/fx/t1/settle", map[string]any{}); resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated: want 200, got %d", resp.StatusCode)
	}
	if got := okFake.caller(); got != "bank-a" {
		t.Errorf("x-caller-identity = %q, want bank-a", got)
	}

	// Orchestrator rejects a non-party caller (PermissionDenied) → 403.
	denyFake := &recordingPaymentServer{err: status.Error(codes.PermissionDenied, "not a party")}
	hDeny := startRecordingBackend(t, denyFake)
	app2 := fiber.New()
	app2.Post("/fx/:tradeId/settle", authedClaims("bank-z"), hDeny.SettleFXAgreement)
	if resp := postJSON(t, app2, "/fx/t1/settle", map[string]any{}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-party: want 403, got %d", resp.StatusCode)
	}

	// No claims → 401.
	app3 := fiber.New()
	app3.Post("/fx/:tradeId/settle", hOK.SettleFXAgreement)
	if resp := postJSON(t, app3, "/fx/t1/settle", map[string]any{}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no claims: want 401, got %d", resp.StatusCode)
	}
}
