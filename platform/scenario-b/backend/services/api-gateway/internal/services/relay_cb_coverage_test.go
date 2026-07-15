// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ---------------------------------------------------------------------------
// CrossCurrencyBridgeInRelay (httptest)
// ---------------------------------------------------------------------------

func TestBridgeInRelay_NotifyBridgeIn_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/amm/cross-currency-bridge-in" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Relay-Auth") != "secret" {
			t.Errorf("missing relay auth header")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "position_id": "pos-99", "bridge_state": "ACTIVE"})
	}))
	defer srv.Close()

	relay := NewCrossCurrencyBridgeInRelay(srv.URL, "secret")
	pos, err := relay.NotifyBridgeIn(context.Background(), CrossCurrencyBridgeInRequest{
		CorrelationID: "corr-1", PayerBankID: "bank-a", SourceCurrency: "BRL", Amount: "1000", SpokeIn: "spoke-a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos != "pos-99" {
		t.Fatalf("expected pos-99, got %s", pos)
	}
}

func TestBridgeInRelay_NotifyBridgeIn_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	relay := NewCrossCurrencyBridgeInRelay(srv.URL, "secret")
	if _, err := relay.NotifyBridgeIn(context.Background(), CrossCurrencyBridgeInRequest{}); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestBridgeInRelay_NotifyBridgeIn_ConnError(t *testing.T) {
	relay := NewCrossCurrencyBridgeInRelay("http://127.0.0.1:1", "secret")
	if _, err := relay.NotifyBridgeIn(context.Background(), CrossCurrencyBridgeInRequest{}); err == nil {
		t.Fatal("expected connection error")
	}
}

// ---------------------------------------------------------------------------
// CactiCrossCurrencyRelay (httptest)
// ---------------------------------------------------------------------------

func TestCactiRelay_NotifyBridgeOut_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cross-currency/bridge-out" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "correlation_id": "corr-echo"})
	}))
	defer srv.Close()

	relay := NewCactiCrossCurrencyRelay(srv.URL, "secret")
	echo, err := relay.NotifyBridgeOut(context.Background(), CactiCrossCurrencyBridgeOutRequest{CorrelationID: "corr-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if echo != "corr-echo" {
		t.Fatalf("expected corr-echo, got %s", echo)
	}
}

func TestCactiRelay_NotifyBridgeOut_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	relay := NewCactiCrossCurrencyRelay(srv.URL, "secret")
	if _, err := relay.NotifyBridgeOut(context.Background(), CactiCrossCurrencyBridgeOutRequest{}); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestCactiRelay_NotifyBridgeOut_ConnError(t *testing.T) {
	relay := NewCactiCrossCurrencyRelay("http://127.0.0.1:1", "secret")
	if _, err := relay.NotifyBridgeOut(context.Background(), CactiCrossCurrencyBridgeOutRequest{}); err == nil {
		t.Fatal("expected connection error")
	}
}

// ---------------------------------------------------------------------------
// CircuitBreakerService Pause / Resume lifecycle (sqlite + fake AMM caller)
// ---------------------------------------------------------------------------

type fakeCBCaller struct {
	pauseTx      string
	pauseErr     error
	proposeReqID string
	proposeErr   error
	signErr      error
	executeErr   error
}

func (f *fakeCBCaller) PauseCircuitBreaker(_ context.Context, _ string, _ []byte) (string, error) {
	return f.pauseTx, f.pauseErr
}
func (f *fakeCBCaller) ProposeResume(_ context.Context, _ string, _ []byte) (string, error) {
	return f.proposeReqID, f.proposeErr
}
func (f *fakeCBCaller) SignResume(_ context.Context, _, _ string, _ []byte) error { return f.signErr }
func (f *fakeCBCaller) ExecuteResume(_ context.Context, _, _ string) error        { return f.executeErr }

func TestCircuitBreakerService_Lifecycle(t *testing.T) {
	db := newTestDB(t, &domain.ScenarioBRiskControlState{}, &domain.CircuitBreakerSignature{})
	// Seed a risk control row so Updates() has a target.
	db.Create(&domain.ScenarioBRiskControlState{PoolPair: "W-BRL-ARS", CircuitBreakerState: domain.CircuitBreakerLive})

	caller := &fakeCBCaller{pauseTx: "0xpause", proposeReqID: "req-1"}
	svc := NewCircuitBreakerService(db, caller)
	sig := []byte{0x01, 0x02}

	// Pause.
	if err := svc.Pause(context.Background(), "W-BRL-ARS", "bank-a", "FRAUD", sig); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	st, _ := svc.GetStatus(context.Background(), "W-BRL-ARS")
	if st.State != string(domain.CircuitBreakerHalted) {
		t.Fatalf("expected HALTED after pause, got %s", st.State)
	}

	// ProposeResume.
	reqID, err := svc.ProposeResume(context.Background(), "W-BRL-ARS", "bank-a", sig)
	if err != nil || reqID != "req-1" {
		t.Fatalf("propose resume failed: req=%s err=%v", reqID, err)
	}

	// SignResume.
	if err := svc.SignResume(context.Background(), "W-BRL-ARS", "req-1", "bank-b", sig); err != nil {
		t.Fatalf("sign resume failed: %v", err)
	}

	// ExecuteResume.
	if err := svc.ExecuteResume(context.Background(), "W-BRL-ARS", "req-1"); err != nil {
		t.Fatalf("execute resume failed: %v", err)
	}
	st, _ = svc.GetStatus(context.Background(), "W-BRL-ARS")
	if st.State != string(domain.CircuitBreakerLive) {
		t.Fatalf("expected LIVE after execute resume, got %s", st.State)
	}
}

func TestCircuitBreakerService_OnChainErrors(t *testing.T) {
	db := newTestDB(t, &domain.ScenarioBRiskControlState{}, &domain.CircuitBreakerSignature{})
	ctx := context.Background()

	if err := NewCircuitBreakerService(db, &fakeCBCaller{pauseErr: errPause}).Pause(ctx, "P", "b", "r", nil); err == nil {
		t.Fatal("expected pause on-chain error")
	}
	if _, err := NewCircuitBreakerService(db, &fakeCBCaller{proposeErr: errPause}).ProposeResume(ctx, "P", "b", nil); err == nil {
		t.Fatal("expected propose on-chain error")
	}
	if err := NewCircuitBreakerService(db, &fakeCBCaller{signErr: errPause}).SignResume(ctx, "P", "r", "b", nil); err == nil {
		t.Fatal("expected sign on-chain error")
	}
	if err := NewCircuitBreakerService(db, &fakeCBCaller{executeErr: errPause}).ExecuteResume(ctx, "P", "r"); err == nil {
		t.Fatal("expected execute on-chain error")
	}
}

var errPause = errInline("on-chain failure")

type errInline string

func (e errInline) Error() string { return string(e) }
