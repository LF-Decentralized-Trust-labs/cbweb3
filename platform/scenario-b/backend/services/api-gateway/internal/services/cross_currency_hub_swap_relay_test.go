// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHubSwapRelay_ForwardsRequestAndReturnsRealizedOutcome(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("X-Relay-Auth")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"executed","swap_sender_address":"0xCBHUB","swap_tx_hash":"0xswap","amount_in":"800"}`))
	}))
	defer srv.Close()

	relay := NewCrossCurrencyHubSwapRelay(srv.URL, "shared-secret")
	res, err := relay.ExecuteHubSwap(context.Background(), CrossCurrencyHubSwapRequest{
		CorrelationID:      "corr-1",
		PayerBankID:        "bank-a",
		BridgeInPositionID: "pos-1",
		PoolPair:           "W-BRL-W-COP",
		TargetCurrency:     "COP",
		AmountOut:          "500",
		MaxAmountIn:        "1000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != HubSwapPath {
		t.Fatalf("expected POST to %s, got %s", HubSwapPath, gotPath)
	}
	if gotAuth != "shared-secret" {
		t.Fatalf("expected the relay secret to be sent, got %q", gotAuth)
	}
	// The payload carries no signer address and no token addresses: the CB is the signer and
	// resolves its own tokens. Sending them would invite the caller to steer the trade.
	if _, present := gotBody["swap_sender_address"]; present {
		t.Fatalf("request must not name a signer address: %v", gotBody)
	}
	if res.TxHash != "0xswap" || res.AmountIn != "800" {
		t.Fatalf("expected the realized outcome, got tx=%q amount_in=%q", res.TxHash, res.AmountIn)
	}
	if res.HubSenderAddress != "0xCBHUB" {
		t.Fatalf("expected the executing CB address to be returned, got %q", res.HubSenderAddress)
	}
}

func TestHubSwapRelay_DuplicateIsASuccessCarryingTheRecordedFacts(t *testing.T) {
	// A replay means the CB already executed this position's swap. Re-running is impossible
	// (the trade is not idempotent on-chain), so the recorded facts are the answer.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"duplicate","swap_sender_address":"0xCBHUB","swap_tx_hash":"0xfirst","amount_in":"777"}`))
	}))
	defer srv.Close()

	res, err := NewCrossCurrencyHubSwapRelay(srv.URL, "s").ExecuteHubSwap(context.Background(), CrossCurrencyHubSwapRequest{})
	if err != nil {
		t.Fatalf("a duplicate must not be an error: %v", err)
	}
	if res.TxHash != "0xfirst" || res.AmountIn != "777" {
		t.Fatalf("expected the recorded swap facts, got tx=%q amount_in=%q", res.TxHash, res.AmountIn)
	}
}

func TestHubSwapRelay_RejectsHollowSuccess(t *testing.T) {
	for name, body := range map[string]string{
		"no tx hash":        `{"status":"executed","swap_sender_address":"0xCBHUB","amount_in":"800"}`,
		"no amount_in":      `{"status":"executed","swap_sender_address":"0xCBHUB","swap_tx_hash":"0xswap"}`,
		"no sender address": `{"status":"executed","swap_tx_hash":"0xswap","amount_in":"800"}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			_, err := NewCrossCurrencyHubSwapRelay(srv.URL, "s").ExecuteHubSwap(context.Background(), CrossCurrencyHubSwapRequest{})
			if err == nil {
				t.Fatalf("expected an incomplete response to fail rather than settle on blanks")
			}
		})
	}
}

func TestHubSwapRelay_UnrecordedSwapStillSettles(t *testing.T) {
	// The CB traded but lost its record. Failing here would guarantee a half-settled payment
	// (input consumed, output undelivered); the condition travels back on the result instead.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"executed_unrecorded","swap_sender_address":"0xCBHUB","swap_tx_hash":"0xswap","amount_in":"800","warning":"reconciliation required"}`))
	}))
	defer srv.Close()

	res, err := NewCrossCurrencyHubSwapRelay(srv.URL, "s").ExecuteHubSwap(context.Background(), CrossCurrencyHubSwapRequest{})
	if err != nil {
		t.Fatalf("expected the settlement to continue, got %v", err)
	}
	if res.State != "executed_unrecorded" {
		t.Fatalf("expected the flagged state to reach the caller, got %q", res.State)
	}
}

func TestHubSwapRelay_SurfacesRejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"max_amount_in exceeds the amount bridged in for this position","code":"MAX_AMOUNT_IN_EXCEEDS_BRIDGED"}`))
	}))
	defer srv.Close()

	_, err := NewCrossCurrencyHubSwapRelay(srv.URL, "s").ExecuteHubSwap(context.Background(), CrossCurrencyHubSwapRequest{})
	if err == nil {
		t.Fatalf("expected a CB rejection to surface as an error")
	}
}
