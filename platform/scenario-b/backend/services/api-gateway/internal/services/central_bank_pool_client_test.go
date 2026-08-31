// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeSovereignPoolPair(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"W-BRL-W-ARS", "W-BRL-ARS"},
		{"W-BRL-ARS", "W-BRL-ARS"},
		{"BRL-USD", "BRL-USD"},
	}
	for _, tc := range tests {
		if got := normalizeSovereignPoolPair(tc.in); got != tc.want {
			t.Errorf("normalizeSovereignPoolPair(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCentralBankPoolClient_GetPoolStatus(t *testing.T) {
	want := PoolStatusResponse{
		PoolPair:   "W-BRL-ARS",
		PoolStatus: "ACTIVE",
		ReserveA:   "100",
		ReserveB:   "200",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/amm/pool/W-BRL-ARS/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := NewCentralBankPoolClient(srv.URL, 0)
	got, err := client.GetPoolStatus(context.Background(), "W-BRL-W-ARS")
	if err != nil {
		t.Fatal(err)
	}
	if got.PoolStatus != "ACTIVE" || got.ReserveA != "100" {
		t.Fatalf("unexpected response: %+v", got)
	}

	active, err := client.IsActive(context.Background(), "W-BRL-ARS")
	if err != nil || !active {
		t.Fatalf("IsActive: active=%v err=%v", active, err)
	}
}

func TestCentralBankPoolClient_GetHubLiquidityConfig(t *testing.T) {
	want := HubLiquidityConfig{
		SovereignAMMAddress:       "0xecfcab0a285d3380e488a39b4bb21e777f8a4eac",
		SovereignHubTokenAAddress: "0x75c35c980c0d37ef46df04d31a140b65503c0eed",
		SovereignHubTokenBAddress: "0x82d50ad3c1091866e258fd0f1a7cc9674609d254",
		DefaultSovereignPoolPair:  "W-BRL-ARS",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/amm/hub-liquidity-config" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	client := NewCentralBankPoolClient(srv.URL, 0)
	got, err := client.GetHubLiquidityConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.SovereignAMMAddress != want.SovereignAMMAddress {
		t.Fatalf("got %q want %q", got.SovereignAMMAddress, want.SovereignAMMAddress)
	}
}
