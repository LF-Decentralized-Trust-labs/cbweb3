// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/hex"
	"testing"

	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

// tradeIDBytes32 returns the raw 32 bytes when given a valid hex-encoded 32-byte
// string, and otherwise hashes the input to 32 bytes.
func TestTradeIDBytes32(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	hexID := hex.EncodeToString(raw)
	got := tradeIDBytes32(hexID)
	if hex.EncodeToString(got) != hexID {
		t.Errorf("valid 32-byte hex should pass through unchanged")
	}
	if len(got) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(got))
	}

	// Non-hex / wrong-length input falls back to sha256 (still 32 bytes, deterministic).
	a := tradeIDBytes32("trade-xyz")
	b := tradeIDBytes32("trade-xyz")
	if len(a) != 32 {
		t.Errorf("expected 32 bytes for hashed id, got %d", len(a))
	}
	if hex.EncodeToString(a) != hex.EncodeToString(b) {
		t.Errorf("hashing must be deterministic")
	}
}

func TestBuildFXProposalParams(t *testing.T) {
	req := &pb.ProposeFXAgreementRequest{
		Originator:      "0x0000000000000000000000000000000000000001",
		CounterpartyB:   "0x0000000000000000000000000000000000000002",
		OriginAmount:    "100",
		CounterAmount:   "500",
		Rate:            "5",
		OriginCurrency:  "BRL",
		CounterCurrency: "USD",
		ExpiryDate:      1893456000,
	}
	params, err := buildFXProposalParams("deadbeef", req)
	if err != nil {
		t.Fatalf("buildFXProposalParams: %v", err)
	}
	if params.OriginAmount.String() != "100" || params.CounterAmount.String() != "500" {
		t.Errorf("amounts not parsed: %+v", params)
	}
	if params.Rate.String() != "5" {
		t.Errorf("rate not parsed: %s", params.Rate)
	}
	if params.ExpiryDate.Uint64() != 1893456000 {
		t.Errorf("expiry not parsed: %s", params.ExpiryDate)
	}
}

func TestBuildFXProposalParams_InvalidAmounts(t *testing.T) {
	tests := []struct {
		name string
		req  *pb.ProposeFXAgreementRequest
	}{
		{"bad origin", &pb.ProposeFXAgreementRequest{OriginAmount: "x", CounterAmount: "5", Rate: "1"}},
		{"bad counter", &pb.ProposeFXAgreementRequest{OriginAmount: "5", CounterAmount: "x", Rate: "1"}},
		{"bad rate", &pb.ProposeFXAgreementRequest{OriginAmount: "5", CounterAmount: "5", Rate: "x"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildFXProposalParams("id", tc.req); err == nil {
				t.Error("expected error for invalid amount")
			}
		})
	}
}
