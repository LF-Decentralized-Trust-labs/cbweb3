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

// These are hermetic unit tests for the on-chain evidence plumbing: they stub
// the Besu RPC with httptest and need no live stack. Run with:
//
//	go test -tags integration -run TestEvidence ./...

func TestEvidenceHexToUint(t *testing.T) {
	cases := map[string]uint64{"0x0": 0, "0x1": 1, "0x10": 16, "0x1042": 4162, "": 0, "0x": 0, "nonhex": 0}
	for in, want := range cases {
		if got := hexToUint(in); got != want {
			t.Errorf("hexToUint(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestEvidenceIsEVMTxHash(t *testing.T) {
	evm := "0x" + strings.Repeat("a", 64)
	cases := map[string]bool{
		evm:                                    true,
		"0x" + strings.Repeat("A", 64):         true,  // upper-case hex ok
		"88ddf35b-d345-4885-b3e7-a31bddf99e9a": false, // Paladin UUID
		"0xabc":                                false, // too short
		evm + "00":                             false, // too long
		"0x" + strings.Repeat("g", 64):         false, // non-hex
		"":                                     false,
	}
	for in, want := range cases {
		if got := isEVMTxHash(in); got != want {
			t.Errorf("isEVMTxHash(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestEvidencePrivacyLayerIDNoRPC(t *testing.T) {
	// A non-EVM hash (Paladin/Zeto id) must be marked privacy-layer WITHOUT an RPC
	// call — the stub fails the test if it is hit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("RPC must not be called for a privacy-layer id")
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	rec := newEvidenceRecorder(map[string]string{"spoke-a": srv.URL})
	txs := rec.resolve([]txRef{{network: "spoke-a", label: "zeto_mint", hash: "88ddf35b-d345-4885-b3e7-a31bddf99e9a"}})
	if len(txs) != 1 || txs[0].Resolved {
		t.Fatalf("expected 1 unresolved tx, got %+v", txs)
	}
	if txs[0].Unresolved != "privacy-layer id (Paladin/Zeto), not an EVM tx" {
		t.Errorf("unexpected reason: %q", txs[0].Unresolved)
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
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"blockNumber":"0x412","gasUsed":"0x5208","status":"0x1","blockHash":"0xabc"}}`))
	}))
	defer srv.Close()

	rec := newEvidenceRecorder(map[string]string{"spoke-a": srv.URL})
	txs := rec.resolve([]txRef{{network: "spoke-a", label: "mint", hash: "0x" + strings.Repeat("d", 64)}})
	if len(txs) != 1 {
		t.Fatalf("expected 1 resolved tx, got %d", len(txs))
	}
	got := txs[0]
	if !got.Resolved {
		t.Fatalf("expected resolved tx, got unresolved: %s", got.Unresolved)
	}
	if got.BlockNumber == nil || *got.BlockNumber != 0x412 {
		t.Errorf("block number = %v, want 1042", got.BlockNumber)
	}
	if got.GasUsed == nil || *got.GasUsed != 21000 {
		t.Errorf("gas used = %v, want 21000", got.GasUsed)
	}
	if got.Status != "success" {
		t.Errorf("status = %q, want success", got.Status)
	}
}

func TestEvidenceReceiptNullIsUnresolved(t *testing.T) {
	// A null result models a privacy-layer (Paladin/Zeto) id or a pending/unknown
	// tx: we record it as unresolved with a reason, never fabricated.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
	}))
	defer srv.Close()

	rec := newEvidenceRecorder(map[string]string{"spoke-b": srv.URL})
	txs := rec.resolve([]txRef{{network: "spoke-b", label: "htlc_lock", hash: "0x" + strings.Repeat("e", 64)}})
	if len(txs) != 1 || txs[0].Resolved {
		t.Fatalf("expected 1 unresolved tx, got %+v", txs)
	}
	if txs[0].BlockNumber != nil || txs[0].GasUsed != nil {
		t.Errorf("unresolved tx must have nil block/gas, got %+v", txs[0])
	}
	if txs[0].Unresolved == "" {
		t.Errorf("expected an unresolved reason")
	}
}

func TestEvidenceRecordPopulatesTopLevel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"blockNumber":"0x64","gasUsed":"0x7530","status":"0x1"}}`))
	}))
	defer srv.Close()

	rec := newEvidenceRecorder(map[string]string{"spoke-a": srv.URL})
	hash := "0x" + strings.Repeat("f", 64)
	rec.record("E2E-A-03", "phase_3_mint", 201, 150*time.Millisecond, true, "corr-123",
		txRef{network: "spoke-a", label: "mint", hash: hash})

	if len(rec.steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(rec.steps))
	}
	s := rec.steps[0]
	if s.TxHash == nil || *s.TxHash != hash {
		t.Errorf("top-level tx_hash = %v, want %s", s.TxHash, hash)
	}
	if s.BlockNumber == nil || *s.BlockNumber != 100 {
		t.Errorf("top-level block_number = %v, want 100", s.BlockNumber)
	}
	if s.GasUsed == nil || *s.GasUsed != 30000 {
		t.Errorf("top-level gas_used = %v, want 30000", s.GasUsed)
	}
	if s.CorrelationID != "corr-123" {
		t.Errorf("correlation id = %q, want corr-123", s.CorrelationID)
	}
}
