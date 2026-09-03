// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Hermetic unit tests for the on-chain evidence plumbing: they stub the Besu RPC
// with httptest and need no live stack. Run with:
//
//	go test -tags integration -run TestEvidence ./...

func TestEvidenceHexToUint(t *testing.T) {
	cases := map[string]uint64{"0x0": 0, "0x1": 1, "0x10": 16, "0x1042": 4162, "": 0, "0x": 0, "nope": 0}
	for in, want := range cases {
		if got := hexToUint(in); got != want {
			t.Errorf("hexToUint(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestEvidenceReceiptResolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method != "eth_getTransactionReceipt" {
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"blockNumber":"0x412","gasUsed":"0x5208","status":"0x1"}}`))
	}))
	defer srv.Close()

	rec := newEvidenceRecorder(map[string]string{"hub": srv.URL})
	txs := rec.resolve([]txRef{{network: "hub", label: "amm_swap", hash: "0x" + strings.Repeat("d", 64)}})
	if len(txs) != 1 || !txs[0].Resolved {
		t.Fatalf("expected 1 resolved tx, got %+v", txs)
	}
	if txs[0].BlockNumber == nil || *txs[0].BlockNumber != 0x412 {
		t.Errorf("block number = %v, want 1042", txs[0].BlockNumber)
	}
	if txs[0].GasUsed == nil || *txs[0].GasUsed != 21000 {
		t.Errorf("gas used = %v, want 21000", txs[0].GasUsed)
	}
	if txs[0].Status != "success" {
		t.Errorf("status = %q, want success", txs[0].Status)
	}
}

func TestEvidenceReceiptNullIsUnresolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
	}))
	defer srv.Close()

	rec := newEvidenceRecorder(map[string]string{"spoke-b": srv.URL})
	txs := rec.resolve([]txRef{{network: "spoke-b", label: "zeto", hash: "0x" + strings.Repeat("e", 64)}})
	if len(txs) != 1 || txs[0].Resolved {
		t.Fatalf("expected 1 unresolved tx, got %+v", txs)
	}
	if txs[0].BlockNumber != nil || txs[0].GasUsed != nil || txs[0].Unresolved == "" {
		t.Errorf("unresolved tx must have nil block/gas + reason, got %+v", txs[0])
	}
}

func TestEvidenceRecordPopulatesTopLevel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"blockNumber":"0x64","gasUsed":"0x7530","status":"0x1"}}`))
	}))
	defer srv.Close()

	rec := newEvidenceRecorder(map[string]string{"spoke-b": srv.URL})
	rec.record("E2E-B-06", "phase_3_fiat_issuance", 201, 90*time.Millisecond, true, "corr-9",
		txRef{network: "spoke-b", label: "fiat_mint", hash: "0x" + strings.Repeat("f", 64)})

	if len(rec.steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(rec.steps))
	}
	s := rec.steps[0]
	if s.TxHash == nil || *s.TxHash != "0x"+strings.Repeat("f", 64) {
		t.Errorf("top-level tx_hash = %v, want all-f hash", s.TxHash)
	}
	if s.BlockNumber == nil || *s.BlockNumber != 100 {
		t.Errorf("top-level block_number = %v, want 100", s.BlockNumber)
	}
	if s.GasUsed == nil || *s.GasUsed != 30000 {
		t.Errorf("top-level gas_used = %v, want 30000", s.GasUsed)
	}
	if s.CorrelationID != "corr-9" {
		t.Errorf("correlation id = %q, want corr-9", s.CorrelationID)
	}
}
