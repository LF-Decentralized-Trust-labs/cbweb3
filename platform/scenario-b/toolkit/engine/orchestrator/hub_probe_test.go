package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// boolWord returns the 32-byte ABI word for a bool.
func boolWord(v bool) string {
	if v {
		return "0x" + strings.Repeat("0", 63) + "1"
	}
	return "0x" + strings.Repeat("0", 64)
}

// hubProbeServer mocks eth_call, answering isWhitelisted/isLiquidityProvider by
// the 4-byte selector prefix in the call data.
func hubProbeServer(t *testing.T, whitelisted, liquidity bool) *httptest.Server {
	t.Helper()
	selWhitelisted := "0x" + selectorHex("isWhitelisted(address)")
	selLiquidity := "0x" + selectorHex("isLiquidityProvider(address)")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params []json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var call struct {
			Data string `json:"data"`
		}
		_ = json.Unmarshal(req.Params[0], &call)
		res := boolWord(false)
		switch {
		case strings.HasPrefix(call.Data, selWhitelisted):
			res = boolWord(whitelisted)
		case strings.HasPrefix(call.Data, selLiquidity):
			res = boolWord(liquidity)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": res})
	}))
}

const cbAddr = "0x00000000000000000000000000000000000000CB"
const regAddr = "0x1000000000000000000000000000000000000001"

// SC-002: register-cb idempotency probe — true only when the CB is BOTH a
// verified participant AND a liquidity provider.
func TestHubCBRegistered(t *testing.T) {
	cases := []struct {
		name                   string
		whitelisted, liquidity bool
		want                   bool
	}{
		{"both", true, true, true},
		{"participant only", true, false, false},
		{"neither", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := hubProbeServer(t, tc.whitelisted, tc.liquidity)
			defer srv.Close()
			got, err := HubCBRegistered(context.Background(), srv.URL, regAddr, cbAddr)
			if err != nil {
				t.Fatalf("hubCBRegistered: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

// A JSON-RPC error surfaces as an error (never a silent false).
func TestHubCBRegisteredRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"error": map[string]any{"code": -32000, "message": "execution reverted"},
		})
	}))
	defer srv.Close()
	if _, err := HubCBRegistered(context.Background(), srv.URL, regAddr, cbAddr); err == nil {
		t.Fatal("expected error on RPC error response")
	}
}
