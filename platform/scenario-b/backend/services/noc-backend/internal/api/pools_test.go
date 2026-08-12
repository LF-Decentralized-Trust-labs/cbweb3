// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/config"
)

// poolsResponse mirrors the JSON the portal consumes from GET /pools.
type poolsResponse struct {
	Data     []NocPoolStatus    `json:"data"`
	Failures []PoolFetchFailure `json:"failures"`
}

// callPools mounts the handler against the given gateway and returns the decoded body.
func callPools(t *testing.T, gatewayURL string, pairs []string) poolsResponse {
	t.Helper()
	app := fiber.New()
	NewPoolsHandler(&config.Config{AMMGatewayURL: gatewayURL, AMMPairs: pairs}).Register(app)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/pools", nil), 10_000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var decoded poolsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return decoded
}

func TestGetPoolsDiscoversPairsWhenNoneArePinned(t *testing.T) {
	var listedPairs int
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/amm/pairs":
			listedPairs++
			_, _ = w.Write([]byte(`{"pairs":[{"pair_id":"W-BRL-ARS","status":"ACTIVE"},{"pair_id":"W-BRL-COP","status":"ACTIVE"}]}`))
		case "/api/v2/amm/pool/W-BRL-ARS/status":
			_, _ = w.Write([]byte(`{"pool_pair":"W-BRL-ARS","reserve_a":"500","reserve_b":"500"}`))
		case "/api/v2/amm/pool/W-BRL-COP/status":
			_, _ = w.Write([]byte(`{"pool_pair":"W-BRL-COP","reserve_a":"500","reserve_b":"500"}`))
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer gateway.Close()

	got := callPools(t, gateway.URL, nil)

	if listedPairs != 1 {
		t.Errorf("pairs listed %d times, want 1", listedPairs)
	}
	if len(got.Data) != 2 {
		t.Fatalf("data has %d pools, want the 2 discovered ones: %+v", len(got.Data), got.Data)
	}
	if len(got.Failures) != 0 {
		t.Errorf("failures = %v, want none", got.Failures)
	}
}

func TestGetPoolsPinnedPairsSkipDiscovery(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/amm/pairs" {
			t.Errorf("discovery called even though AMM_PAIRS pins the pairs")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pool_pair":"PINNED","reserve_a":"500","reserve_b":"500"}`))
	}))
	defer gateway.Close()

	got := callPools(t, gateway.URL, []string{"PINNED"})

	if len(got.Data) != 1 || got.Data[0].Pair != "PINNED" {
		t.Errorf("data = %+v, want the pinned pair", got.Data)
	}
}

func TestGetPoolsReportsADiscoveryFailure(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gateway down", http.StatusBadGateway)
	}))
	defer gateway.Close()

	got := callPools(t, gateway.URL, nil)

	if len(got.Failures) != 1 || got.Failures[0].Pair != discoveryPseudoPair {
		t.Fatalf("failures = %+v, want one discovery failure", got.Failures)
	}
	if len(got.Data) != 0 {
		t.Errorf("data = %+v, want empty", got.Data)
	}
}

func TestGetPoolsReturnsPoolStatus(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v2/amm/pool/W-BRL-ARS/status"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pool_pair":"W-BRL-ARS","pool_status":"ACTIVE","reserve_a":"600","reserve_b":"400"}`))
	}))
	defer gateway.Close()

	got := callPools(t, gateway.URL, []string{"W-BRL-ARS"})

	if len(got.Data) != 1 {
		t.Fatalf("data has %d pools, want 1", len(got.Data))
	}
	if len(got.Failures) != 0 {
		t.Errorf("failures = %v, want none", got.Failures)
	}
	if got.Data[0].Pair != "W-BRL-ARS" || got.Data[0].ReserveA != "600" {
		t.Errorf("unexpected pool %+v", got.Data[0])
	}
	// 60/40 stays inside the 70/30 band.
	if got.Data[0].Breached7030 {
		t.Errorf("60/40 split reported as breached: %+v", got.Data[0])
	}
}

func TestGetPoolsReportsUnreachableGatewayInsteadOfLookingEmpty(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"not found among active pairs"}`, http.StatusNotFound)
	}))
	defer gateway.Close()

	got := callPools(t, gateway.URL, []string{"W-BRL-ARS"})

	if len(got.Data) != 0 {
		t.Errorf("data = %v, want empty", got.Data)
	}
	if len(got.Failures) != 1 {
		t.Fatalf("failures has %d entries, want 1 — a failed fetch must not be silent", len(got.Failures))
	}
	if got.Failures[0].Pair != "W-BRL-ARS" || got.Failures[0].Reason == "" {
		t.Errorf("unexpected failure %+v", got.Failures[0])
	}
}

func TestGetPoolsKeepsPartialResults(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/amm/pool/GOOD-PAIR/status" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"pool_pair":"GOOD-PAIR","reserve_a":"800","reserve_b":"200"}`))
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer gateway.Close()

	got := callPools(t, gateway.URL, []string{"GOOD-PAIR", "BAD-PAIR"})

	if len(got.Data) != 1 || got.Data[0].Pair != "GOOD-PAIR" {
		t.Errorf("data = %+v, want just GOOD-PAIR", got.Data)
	}
	if len(got.Failures) != 1 || got.Failures[0].Pair != "BAD-PAIR" {
		t.Errorf("failures = %+v, want just BAD-PAIR", got.Failures)
	}
	// 80/20 breaches the 70/30 band.
	if !got.Data[0].Breached7030 {
		t.Errorf("80/20 split not reported as breached: %+v", got.Data[0])
	}
}
