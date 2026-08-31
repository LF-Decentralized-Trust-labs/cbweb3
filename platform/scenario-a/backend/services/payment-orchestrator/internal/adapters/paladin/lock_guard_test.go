// SPDX-License-Identifier: Apache-2.0

// End-to-end wiring test for the unsettleable-lock guard.
//
// The predicate itself is covered in locked_state_id_test.go. What this file pins is
// the part a unit test on the predicate cannot: that Lock() actually consults it and
// refuses, so no caller ever receives a ZetoLockResult carrying a state id that
// cannot be settled. That matters because the orchestrator creates the public,
// cross-spoke HTLC record immediately after Lock() returns — refusing here is what
// keeps a poisoned trade out of the relay, whose handler halts its event cursor on
// any error and would retry such a settle forever.
//
// The Paladin JSON-RPC is stubbed rather than mocked at the interface, so the guard
// is exercised through the real request/response path: resolveVerifier →
// ptx_sendTransaction → ptx_getStateReceipt.
package paladin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// stubPaladin answers the three calls Lock makes, returning the given state id from
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

// A lock whose state id begins with a zero byte must be refused, and the error must
// explain itself — this is the exact id that failed live.
func TestLock_RefusesUnsettleableStateID(t *testing.T) {
	t.Parallel()
	const bad = "0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6"
	srv := stubPaladin(t, bad)
	defer srv.Close()

	result, err := testClient(t, srv.URL).Lock(context.Background(), "1", "funded_operator@spoke-brl-bank-bradesco")
	if err == nil {
		t.Fatalf("Lock accepted an unsettleable state id; result=%+v", result)
	}
	if result != nil {
		t.Errorf("Lock returned a result alongside the error: %+v", result)
	}
	if !strings.Contains(err.Error(), bad) {
		t.Errorf("error does not name the offending id: %v", err)
	}
	if !strings.Contains(err.Error(), "PD210134") {
		t.Errorf("error does not cite the failure it prevents: %v", err)
	}
}

// The guard must not touch ordinary locks. 0x0a and 0x0e have a zero high nibble and
// settled fine in the live run, so they are the cases most at risk of a
// too-aggressive check.
func TestLock_AcceptsOrdinaryStateIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []string{
		"0x0a6166eca978d3a555321234567890abcdef1234567890abcdef1234567890ab",
		"0x0e69fa562c7230f1d7f01234567890abcdef1234567890abcdef1234567890ab",
		"0x2f69fa562c7230f1d7f01234567890abcdef1234567890abcdef1234567890ab",
	} {
		id := id
		t.Run(id[:8], func(t *testing.T) {
			t.Parallel()
			srv := stubPaladin(t, id)
			defer srv.Close()

			result, err := testClient(t, srv.URL).Lock(context.Background(), "1", "funded_operator@spoke-brl-bank-bradesco")
			if err != nil {
				t.Fatalf("Lock refused an ordinary state id %s: %v", id, err)
			}
			if len(result.LockedStateIDs) != 1 || result.LockedStateIDs[0] != id {
				t.Errorf("LockedStateIDs = %v, want [%s]", result.LockedStateIDs, id)
			}
		})
	}
}

// The refusal must be recognisable by the caller, not only readable by a human. The
// gRPC layer maps this sentinel to FailedPrecondition so a client can tell "retry, you
// will almost certainly get a usable id" from a genuine fault.
func TestLock_RefusalMatchesTheSentinel(t *testing.T) {
	t.Parallel()
	const bad = "0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6"
	srv := stubPaladin(t, bad)
	defer srv.Close()

	_, err := testClient(t, srv.URL).Lock(context.Background(), "1", "funded_operator@spoke-brl-bank-bradesco")
	if !errors.Is(err, ports.ErrUnsettleableLock) {
		t.Fatalf("refusal does not match ports.ErrUnsettleableLock: %v", err)
	}
}

// The tokens of a refused lock are locked on-chain and cannot be released, so this log
// line is the only trace that a specific sum became unrecoverable. Without the amount it
// cannot be reconciled against anything — the record has to say how much.
func TestLock_RefusalRecordsTheOrphanedAmount(t *testing.T) {
	t.Parallel()
	const bad = "0x00758ab137c47984ef49b1b7c3b073fb698f7366a8ad8ea98d8b915c4eb723b6"
	const amount = "1000"
	srv := stubPaladin(t, bad)
	defer srv.Close()

	var logged bytes.Buffer
	client := NewClient(ClientConfig{
		BaseURL:          srv.URL,
		Identity:         "funded_operator@spoke-brl-bank-itau",
		ZetoTokenAddress: "0x6213607c2fab2ddd572f7754a608565cd28ba7aa",
	}, slog.New(slog.NewTextHandler(&logged, nil)))

	if _, err := client.Lock(context.Background(), amount, "funded_operator@spoke-brl-bank-bradesco"); err == nil {
		t.Fatal("Lock accepted an unsettleable state id")
	}

	out := logged.String()
	for _, want := range []string{amount, bad, "cannot be released"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal record does not mention %q; got:\n%s", want, out)
		}
	}
}
