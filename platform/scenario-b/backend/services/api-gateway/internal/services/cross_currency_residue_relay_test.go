// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
)

// ---------------------------------------------------------------------------
// CrossCurrencyResidueRelay (httptest)
//
// Step 4 of a cross-currency swap: the initiating gateway asks the issuing CB of the
// source currency to give the unspent slippage buffer back to the payer. Only the CB
// holds CENTRAL_BANK_ROLE on the wrapped source token, so the leg is delegated over
// the same direct channel bridge-in uses.
// ---------------------------------------------------------------------------

func TestResidueRelay_NotifyResidueReturn_Success(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ResidueReturnPath {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Relay-Auth") != "secret" {
			t.Errorf("missing relay auth header")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected content type: %s", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "position_id": "pos-residue-1"})
	}))
	defer srv.Close()

	relay := NewCrossCurrencyResidueRelay(srv.URL, "secret")
	pos, err := relay.NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{
		CorrelationID:      "corr-1",
		SwapTxHash:         "0xswap",
		PoolPair:           "W-BRL-ARS",
		BridgeInPositionID: "pos-in-1",
		PayerBankID:        "bank-a",
		SpokeIn:            "spoke-brl",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos != "pos-residue-1" {
		t.Fatalf("expected pos-residue-1, got %s", pos)
	}

	// The amount is derived by the CB from its own bridge-in position and the on-chain
	// LogSwap. A caller that could name the amount could inflate its own refund, so the
	// payload must not carry one — this asserts the wire contract, not just the struct.
	for _, forbidden := range []string{"amount", "residue_amount", "max_amount_in"} {
		if _, present := gotBody[forbidden]; present {
			t.Errorf("residue-return payload must not carry %q", forbidden)
		}
	}
	if gotBody["bridge_in_position_id"] != "pos-in-1" {
		t.Errorf("bridge_in_position_id not forwarded: %v", gotBody["bridge_in_position_id"])
	}
	if gotBody["swap_tx_hash"] != "0xswap" {
		t.Errorf("swap_tx_hash not forwarded: %v", gotBody["swap_tx_hash"])
	}
}

// The CB enqueues rather than waiting for the release, so 202 is a success too.
func TestResidueRelay_NotifyResidueReturn_Accepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "position_id": "pos-residue-2"})
	}))
	defer srv.Close()

	pos, err := NewCrossCurrencyResidueRelay(srv.URL+"/", "secret").
		NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{CorrelationID: "corr-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos != "pos-residue-2" {
		t.Fatalf("expected pos-residue-2, got %s", pos)
	}
}

func TestResidueRelay_NotifyResidueReturn_Signed(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	registry := relayauth.NewRegistry()
	registry.Add("central-bank-a", &key.PublicKey)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := registry.VerifyRequest(
			r.Header.Get(relayauth.HeaderKeyID),
			r.Header.Get(relayauth.HeaderTimestamp),
			r.Header.Get(relayauth.HeaderSignature),
			http.MethodPost, ResidueReturnPath, body, time.Now(),
		); err != nil {
			t.Errorf("signature did not verify: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"position_id": "pos-signed"})
	}))
	defer srv.Close()

	relay := NewCrossCurrencyResidueRelay(srv.URL, "secret").
		WithSigner(relayauth.NewSigner("central-bank-a", key))
	pos, err := relay.NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{CorrelationID: "corr-3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos != "pos-signed" {
		t.Fatalf("expected pos-signed, got %s", pos)
	}
}

// A signer without a usable key must fail before the request goes out: an unsigned
// residue return would be rejected by the CB middleware anyway.
func TestResidueRelay_NotifyResidueReturn_SignError(t *testing.T) {
	sent := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sent = true
		_ = json.NewEncoder(w).Encode(map[string]string{"position_id": "should-not-happen"})
	}))
	defer srv.Close()

	relay := NewCrossCurrencyResidueRelay(srv.URL, "secret").WithSigner(relayauth.NewSigner("central-bank-a", nil))
	if _, err := relay.NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{}); err == nil {
		t.Fatal("expected signing error")
	}
	if sent {
		t.Fatal("request must not be sent when signing fails")
	}
}

func TestResidueRelay_NotifyResidueReturn_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("POSITION_OWNER_MISMATCH"))
	}))
	defer srv.Close()

	_, err := NewCrossCurrencyResidueRelay(srv.URL, "secret").
		NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{})
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	// The CB's rejection reason must survive into the error: this leg is not retried
	// automatically, so the log line is the only diagnostic.
	if !strings.Contains(err.Error(), "POSITION_OWNER_MISMATCH") {
		t.Fatalf("error must carry the CB response body, got: %v", err)
	}
}

func TestResidueRelay_NotifyResidueReturn_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	if _, err := NewCrossCurrencyResidueRelay(srv.URL, "secret").
		NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{}); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestResidueRelay_NotifyResidueReturn_ConnError(t *testing.T) {
	if _, err := NewCrossCurrencyResidueRelay("http://127.0.0.1:1", "secret").
		NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{}); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestResidueRelay_NotifyResidueReturn_BadURL(t *testing.T) {
	if _, err := NewCrossCurrencyResidueRelay("http://\x7f", "secret").
		NotifyResidueReturn(context.Background(), CrossCurrencyResidueReturnRequest{}); err == nil {
		t.Fatal("expected request construction error")
	}
}

// CrossCurrencyResidueRelay must satisfy the interface the orchestrator depends on.
func TestResidueRelay_ImplementsIface(t *testing.T) {
	var _ CrossCurrencyResidueRelayIface = NewCrossCurrencyResidueRelay("http://cb", "secret")
}

// ---------------------------------------------------------------------------
// BridgePositionReader.GetPosition
//
// The residue handler authorizes a return against the bridge-in position the CB itself
// created — the bridged amount is read from here, never from the request body.
// ---------------------------------------------------------------------------

func TestBridgePositionReader_GetPosition(t *testing.T) {
	db := newTestDB(t, &domain.BridgedAssetPosition{})
	db.Create(&domain.BridgedAssetPosition{
		PositionID:           "pos-in-1",
		OwnerBankID:          "bank-a",
		SpokeNetwork:         "spoke-brl",
		NativeAsset:          "tCeBM-BRL",
		MirroredAsset:        "W-BRL",
		MirroredAmount:       "15382248884786955855",
		BridgeState:          domain.BridgeStateActive,
		MintToHubAddress:     "0xhub",
		BurnFromSpokeAddress: "0xspoke",
		Leg:                  domain.BridgeLegSettlement,
	})

	reader := NewBridgePositionReader(db)

	detail, err := reader.GetPosition(context.Background(), "pos-in-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.MirroredAmount != "15382248884786955855" {
		t.Errorf("mirrored_amount not read from the CB's own position: %s", detail.MirroredAmount)
	}
	if detail.OwnerBankID != "bank-a" || detail.MintToHubAddress != "0xhub" ||
		detail.BurnFromSpokeAddress != "0xspoke" || detail.Leg != string(domain.BridgeLegSettlement) {
		t.Errorf("unexpected detail: %+v", detail)
	}

	if _, err := reader.GetPosition(context.Background(), ""); err == nil {
		t.Error("expected error for empty position_id")
	}
	if _, err := reader.GetPosition(context.Background(), "pos-missing"); err == nil {
		t.Error("expected error for unknown position_id")
	}
}
