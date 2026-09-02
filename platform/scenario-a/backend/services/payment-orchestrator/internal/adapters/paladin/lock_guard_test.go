// SPDX-License-Identifier: Apache-2.0

// Lock() must accept every well-formed locked state id, including one whose first
// byte is zero.
//
// This file used to pin the opposite. Paladin v0.15.0-rc.1 could not spend a
// zero-byte locked state via transferLocked — it failed permanently with
// PD210134 ("Failed to query states by IDs. Wanted: 1, Found: 0") — so the adapter
// refused such a lock rather than create an HTLC that could never settle.
//
// On the pinned v1.0.0 that call works. Verified on 2026-09-02 on a stack rebuilt
// from zero, by spending three harvested zero-byte states with a transferLocked
// built to match this adapter's own call, reading every receipt:
//
//	0x003b014366c41e58…d2ab389  success: true, block 1634
//	0x0099834d9bf6b953…15aedc   success: true, block 1651
//	0x000659303c8dc207…b85c39   success: true, block 1651
//
// Evidence and method are in docs/paladin-upgrade.md. The guard, its predicate and
// the ports.ErrUnsettleableLock sentinel are gone with it: a refusal that no longer
// corresponds to a real failure would strand funds it was written to protect, since
// ~4% of locks draw such an id (measured, 3 in 71 — not the ~1 in 256 the old
// comment assumed).
//
// The Paladin JSON-RPC is stubbed rather than mocked at the interface, so Lock is
// exercised through the real request/response path: resolveVerifier →
// ptx_sendTransaction → ptx_getTransactionFull → ptx_getStateReceipt.
package paladin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubPaladin answers the calls Lock makes, returning the given state id from
// ptx_getStateReceipt as a confirmed, locked state.
func stubPaladin(t *testing.T, stateID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "ptx_resolveVerifier":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":4,"result":"0x1111111111111111111111111111111111111111"}`))
		case "ptx_sendTransaction":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"11111111-2222-3333-4444-555555555555"}`))
		case "ptx_getTransactionFull":
			// sendTx does not return on the tx id alone — it polls for the receipt
			// (pollReceipt → ptx_getTransactionFull) and fails the call unless the
			// receipt reports success. A stub that omits this times out for 60s.
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"receipt":{"id":"11111111-2222-3333-4444-555555555555","success":true}}}`))
		case "ptx_getStateReceipt":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":5,"result":{"confirmed":[{"id":"` + stateID + `","data":{"locked":true}}]}}`))
		default:
			http.Error(w, "unexpected method "+req.Method, http.StatusInternalServerError)
		}
	}))
}

func testClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	return NewClient(ClientConfig{
		BaseURL:          baseURL,
		Identity:         "funded_operator@spoke-brl-bank-itau",
		ZetoTokenAddress: "0x6213607c2fab2ddd572f7754a608565cd28ba7aa",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// TestLock_AcceptsEveryWellFormedStateID covers the shapes that mattered while the
// guard existed, so a reintroduced check cannot pass unnoticed:
//
//   - the three zero-byte ids proven settleable on v1.0.0, one of them the id that
//     the old guard's own tests used as its canonical refusal case;
//   - 0x0a and 0x0e, a zero high nibble but a non-zero byte, which settled even on
//     the affected build and were the cases most at risk of a too-broad check;
//   - an ordinary id, as the baseline.
func TestLock_AcceptsEveryWellFormedStateID(t *testing.T) {
	t.Parallel()
	for _, id := range []string{
		"0x003b014366c41e58ab5a2a58beb384a81cc39a959c2b5d77fc41017c0d2ab389",
		"0x0099834d9bf6b953f2388ea5d204c1b5205544a7587a09e763b5015a8b15aedc",
		"0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6",
		"0x0a6166eca978d3a555321234567890abcdef1234567890abcdef1234567890ab",
		"0x0e69fa562c7230f1d7f01234567890abcdef1234567890abcdef1234567890ab",
		"0x2f69fa562c7230f1d7f01234567890abcdef1234567890abcdef1234567890ab",
	} {
		t.Run(id[:8], func(t *testing.T) {
			t.Parallel()
			srv := stubPaladin(t, id)
			defer srv.Close()

			result, err := testClient(t, srv.URL).Lock(context.Background(), "1", "funded_operator@spoke-brl-bank-bradesco")
			if err != nil {
				t.Fatalf("Lock refused state id %s: %v", id, err)
			}
			if len(result.LockedStateIDs) != 1 || result.LockedStateIDs[0] != id {
				t.Errorf("LockedStateIDs = %v, want [%s]", result.LockedStateIDs, id)
			}
		})
	}
}
