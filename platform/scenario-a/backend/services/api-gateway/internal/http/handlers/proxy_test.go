// SPDX-License-Identifier: Apache-2.0

// This file covers the PaymentProxyHandler and OnboardingProxyHandler, which
// forward commercial-bank requests to the Central Bank over HTTP. The CB is
// faked with httptest.Server and the local Zeto/KMS dependencies with stubs and
// an in-process payment gRPC backend — fully hermetic.
package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/gofiber/fiber/v2"
)

var errTest = errors.New("test error")

func strReaderRaw(s string) io.Reader { return strings.NewReader(s) }

// ── PaymentProxyHandler ───────────────────────────────────────────────────────

func TestPaymentProxy_RegisterAndList(t *testing.T) {
	t.Parallel()

	const entityBesuAddr = "0xbesu"

	// Capture the query strings seen by the fake Central Bank for list operations.
	type listCall struct{ path, query string }
	var listCalls []listCall

	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			listCalls = append(listCalls, listCall{r.URL.Path, r.URL.RawQuery})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(cb.Close)

	ph := startFakePaymentBackend(t, &fakePaymentServer{zeto: &pb.InitiateZetoTransferResponse{TxHash: "ztx"}})
	h := NewPaymentProxyHandler(cb.URL, ph.payment, entityBesuAddr, "pal", "cbpal", "relay-secret")

	app := fiber.New()
	app.Post("/deposits", h.RegisterDeposit)
	app.Get("/deposits", h.ListDeposits)
	app.Post("/escrows", h.RequestEscrow)
	app.Get("/escrows", h.ListEscrows)
	app.Post("/redeems", h.RequestRedeem)
	app.Get("/redeems", h.ListRedeems)

	if resp := postJSON(t, app, "/deposits", map[string]any{"amount": "1"}); resp.StatusCode != http.StatusOK {
		t.Errorf("register deposit: want 200, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/escrows", map[string]any{"amount": "1"}); resp.StatusCode != http.StatusOK {
		t.Errorf("request escrow: want 200, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/redeems", map[string]any{"amount": "1"}); resp.StatusCode != http.StatusOK {
		t.Errorf("request redeem: want 200, got %d", resp.StatusCode)
	}

	// List endpoints must return 200 and must always forward the entity's own Besu address
	// as requester_id — ignoring any requester_id that a client may try to inject.
	for _, p := range []string{"/deposits", "/deposits?requester_id=other-bank", "/escrows", "/redeems"} {
		if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, p, nil)); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", p, resp.StatusCode)
		}
	}

	// Verify the CB always received requester_id=entityBesuAddr for every list call.
	for _, call := range listCalls {
		got := call.query
		want := "requester_id=" + entityBesuAddr
		if got != want {
			t.Errorf("CB received query %q for path %s; want %q — commercial bank isolation broken", got, call.path, want)
		}
	}
}

func TestPaymentProxy_Validation(t *testing.T) {
	t.Parallel()
	ph := startFakePaymentBackend(t, &fakePaymentServer{})
	h := NewPaymentProxyHandler("http://localhost:1", ph.payment, "0x", "pal", "cbpal", "")
	app := fiber.New()
	app.Post("/redeems", h.RequestRedeem)
	app.Post("/deposits", h.RegisterDeposit)

	// invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/redeems", strReaderRaw("{"))
	req.Header.Set("Content-Type", "application/json")
	if resp, _ := app.Test(req); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("redeem invalid json: want 400, got %d", resp.StatusCode)
	}
	// missing amount
	if resp := postJSON(t, app, "/redeems", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("redeem missing amount: want 400, got %d", resp.StatusCode)
	}
}

func TestPaymentProxy_CBUnreachable(t *testing.T) {
	t.Parallel()
	ph := startFakePaymentBackend(t, &fakePaymentServer{})
	// invalid base URL → proxy returns 502.
	h := NewPaymentProxyHandler("http://127.0.0.1:1", ph.payment, "0x", "pal", "cbpal", "")
	app := fiber.New()
	app.Post("/deposits", h.RegisterDeposit)
	if resp := postJSON(t, app, "/deposits", map[string]any{"amount": "1"}); resp.StatusCode != http.StatusBadGateway {
		t.Errorf("want 502, got %d", resp.StatusCode)
	}
}

func TestPaymentProxy_RedeemZetoError(t *testing.T) {
	t.Parallel()
	ph := startFakePaymentBackend(t, &fakePaymentServer{err: errTest})
	h := NewPaymentProxyHandler("http://localhost:1", ph.payment, "0x", "pal", "cbpal", "")
	app := fiber.New()
	app.Post("/redeems", h.RequestRedeem)
	if resp := postJSON(t, app, "/redeems", map[string]any{"amount": "1"}); resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("zeto error: want 500, got %d", resp.StatusCode)
	}
}

// ── OnboardingProxyHandler (dumb mode, no keyMgr) ─────────────────────────────

func TestOnboardingProxy_DumbMode(t *testing.T) {
	t.Parallel()
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(cb.Close)

	// No keyMgr, no pkiDir → not smart mode; plain pass-through proxy.
	h := NewOnboardingProxyHandler(cb.URL, "", "bank-a", nil)
	app := fiber.New()
	app.Post("/initiate", h.InitiateCredentialRequest)
	app.Get("/status/:requestId", h.GetOnboardingStatus)
	app.Post("/complete", h.CompleteOnboarding)
	app.Get("/my-status", h.GetMyOnboardingStatus)

	if resp := postJSON(t, app, "/initiate", map[string]any{"x": 1}); resp.StatusCode != http.StatusOK {
		t.Errorf("initiate: want 200, got %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/status/req-1", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}
	if resp := postJSON(t, app, "/complete", map[string]any{"x": 1}); resp.StatusCode != http.StatusOK {
		t.Errorf("complete: want 200, got %d", resp.StatusCode)
	}
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/my-status", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("my-status: want 200, got %d", resp.StatusCode)
	}
}

func TestOnboardingProxy_StatusMissingRequestID(t *testing.T) {
	t.Parallel()
	h := NewOnboardingProxyHandler("http://localhost:1", "", "bank-a", nil)
	app := fiber.New()
	app.Get("/status/", h.GetOnboardingStatus)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/status/", nil)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOnboardingProxy_MyStatusNoIdentity(t *testing.T) {
	t.Parallel()
	// bankCode empty and no claims → 400.
	h := NewOnboardingProxyHandler("http://localhost:1", "", "", nil)
	app := fiber.New()
	app.Get("/my-status", h.GetMyOnboardingStatus)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/my-status", nil)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOnboardingProxy_MyStatusFromClaims(t *testing.T) {
	t.Parallel()
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(cb.Close)
	h := NewOnboardingProxyHandler(cb.URL, "", "", nil)
	app := fiber.New()
	app.Get("/my-status", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{BankID: "bank-a"})
		return h.GetMyOnboardingStatus(c)
	})
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/my-status", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// keyMgrStub implements interfaces.OnboardingKeyManager.
type keyMgrStub struct {
	pubKey string
	addr   string
	sig    string
	err    error
}

func (s keyMgrStub) CreateOnboardingKey(context.Context, string) (string, string, error) {
	return s.pubKey, s.addr, s.err
}
func (s keyMgrStub) SignOnboardingPoP(context.Context, string, string) (string, string, error) {
	return s.sig, s.pubKey, s.err
}

// writeEC writes a P-256 private key PEM to pkiDir/<bank>.key for signNonceP256.
func writeEC(t *testing.T, pkiDir, bank string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(filepath.Join(pkiDir, bank+".key"), pemBytes, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestOnboardingProxy_SmartMode_InitiateAndComplete(t *testing.T) {
	t.Parallel()
	pkiDir := t.TempDir()
	bank := "bank-a"
	// CSR file required by InitiateCredentialRequest.
	if err := os.WriteFile(filepath.Join(pkiDir, bank+".csr"), []byte("CSRDATA"), 0600); err != nil {
		t.Fatal(err)
	}
	writeEC(t, pkiDir, bank)

	// Fake CB: serves onboarding status (with pop_nonce), complete, login, wallet/bind.
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/onboarding/status/req-1":
			_, _ = w.Write([]byte(`{"pop_nonce":"deadbeef","status":"APPROVED"}`))
		case r.URL.Path == "/api/v1/onboarding/credential-request":
			_, _ = w.Write([]byte(`{"request_id":"req-1"}`))
		case r.URL.Path == "/api/v1/onboarding/complete":
			_, _ = w.Write([]byte(`{"user_id":"u","cert_pem":"CERT","client_secret":"sec"}`))
		case r.URL.Path == "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"nonce":"cafe"}`))
		case r.URL.Path == "/api/v1/auth/wallet/bind":
			_, _ = w.Write([]byte(`{"accessToken":"jwt-token"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(cb.Close)

	km := keyMgrStub{pubKey: "04abcd", addr: "0xabc", sig: "ffee"}
	h := NewOnboardingProxyHandler(cb.URL, pkiDir, bank, km)
	app := fiber.New()
	app.Post("/initiate", h.InitiateCredentialRequest)
	app.Post("/complete", h.CompleteOnboarding)
	app.Post("/pki-login", h.PKILogin)

	if resp := postJSON(t, app, "/initiate", map[string]any{"institution_name": "I"}); resp.StatusCode != http.StatusOK {
		t.Errorf("smart initiate: want 200, got %d", resp.StatusCode)
	}
	// complete: status fetch → sign → forward → chained PKI login.
	complete := map[string]any{"request_id": "req-1", "user_id": "u"}
	resp := postJSON(t, app, "/complete", complete)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("smart complete: want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["access_token"] != "jwt-token" {
		t.Errorf("expected chained access_token, got %v", body["access_token"])
	}

	// pki-login: reads saved cert (written by complete) and authenticates.
	if resp := postJSON(t, app, "/pki-login", map[string]any{"user_id": "u", "client_secret": "sec"}); resp.StatusCode != http.StatusOK {
		t.Errorf("pki-login: want 200, got %d", resp.StatusCode)
	}
}

func TestOnboardingProxy_CompleteValidation(t *testing.T) {
	t.Parallel()
	pkiDir := t.TempDir()
	km := keyMgrStub{pubKey: "04ab"}
	h := NewOnboardingProxyHandler("http://localhost:1", pkiDir, "bank-a", km)
	app := fiber.New()
	app.Post("/complete", h.CompleteOnboarding)
	// smart mode but missing request_id/user_id → 400.
	if resp := postJSON(t, app, "/complete", map[string]any{}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOnboardingProxy_PKILoginNoCert(t *testing.T) {
	t.Parallel()
	pkiDir := t.TempDir() // no participant cert present
	km := keyMgrStub{pubKey: "04ab"}
	h := NewOnboardingProxyHandler("http://localhost:1", pkiDir, "bank-a", km)
	app := fiber.New()
	app.Post("/pki-login", h.PKILogin)
	// missing fields → 400
	if resp := postJSON(t, app, "/pki-login", map[string]any{"user_id": "u"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("validation: want 400, got %d", resp.StatusCode)
	}
	// cert not found → 412
	if resp := postJSON(t, app, "/pki-login", map[string]any{"user_id": "u", "client_secret": "s"}); resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("no cert: want 412, got %d", resp.StatusCode)
	}
}

func TestSignNonceP256(t *testing.T) {
	t.Parallel()
	pkiDir := t.TempDir()
	writeEC(t, pkiDir, "bank-a")
	h := NewOnboardingProxyHandler("http://x", pkiDir, "bank-a", nil)
	sig, err := h.signNonceP256(hex.EncodeToString([]byte("hello")))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if sig == "" {
		t.Fatal("empty signature")
	}
	// invalid hex nonce → error
	if _, err := h.signNonceP256("zz"); err == nil {
		t.Fatal("expected error for invalid hex")
	}
}
