// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

// White-box tests for the pure helper functions in the server package. These
// cannot be reached from the external server_test package because the symbols
// are unexported.

func TestNewUUID_Unique(t *testing.T) {
	a, b := newUUID(), newUUID()
	if a == "" || b == "" || a == b {
		t.Errorf("newUUID returned empty or duplicate values: %q %q", a, b)
	}
}

func TestContractIDBytes(t *testing.T) {
	// A 64-hex-char string decodes to 32 bytes unchanged.
	full := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	if got := contractIDBytes(full); len(got) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(got))
	}
	// An over-length input is truncated to 32 bytes.
	over := full + "ffff"
	if got := contractIDBytes(over); len(got) != 32 {
		t.Errorf("expected truncation to 32 bytes, got %d", len(got))
	}
	// Invalid hex yields an empty slice (decode error swallowed).
	if got := contractIDBytes("zzzz"); len(got) != 0 {
		t.Errorf("expected empty slice for invalid hex, got %d", len(got))
	}
}

func TestTradeIDBytes32(t *testing.T) {
	// A 32-byte hex string is used verbatim.
	full := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	raw := tradeIDBytes32(full)
	want, _ := hex.DecodeString(full)
	if string(raw) != string(want) {
		t.Error("expected verbatim hex bytes for 32-byte hex trade id")
	}
	// A non-hex / short string is hashed to 32 bytes.
	hashed := tradeIDBytes32("trade-xyz")
	if len(hashed) != 32 {
		t.Errorf("expected 32-byte hash, got %d", len(hashed))
	}
	// tradeIDBytes is an alias.
	if string(tradeIDBytes("trade-xyz")) != string(hashed) {
		t.Error("tradeIDBytes should match tradeIDBytes32")
	}
}

func TestAgreementCommitmentHash_Deterministic(t *testing.T) {
	rec := &domain.FXAgreementRecord{
		TradeID: "T1", OriginAmount: "100", CounterAmount: "120", Rate: "1.2",
	}
	s := &paymentOrchestratorService{}
	h1 := s.agreementCommitmentHash(rec)
	h2 := s.agreementCommitmentHash(rec)
	if h1 != h2 {
		t.Error("commitment hash must be deterministic")
	}
	rec2 := &domain.FXAgreementRecord{
		TradeID: "T2", OriginAmount: "100", CounterAmount: "120", Rate: "1.2",
	}
	if s.agreementCommitmentHash(rec2) == h1 {
		t.Error("different trade ids must produce different commitments")
	}
}

func TestBuildFXProposalParams(t *testing.T) {
	req := &pb.ProposeFXAgreementRequest{
		Originator: "0x1111111111111111111111111111111111111111",
		CounterpartyB: "0x2222222222222222222222222222222222222222",
		OriginAmount: "100", CounterAmount: "120", Rate: "1",
		OriginCurrency: "USD", CounterCurrency: "BRL", ExpiryDate: 9999,
	}
	params, err := buildFXProposalParams("T-PARAMS", req)
	if err != nil {
		t.Fatalf("buildFXProposalParams: %v", err)
	}
	if params.OriginAmount.String() != "100" || params.CounterAmount.String() != "120" {
		t.Errorf("amounts not parsed: %+v", params)
	}
	if params.ExpiryDate.Uint64() != 9999 {
		t.Errorf("expiry = %s", params.ExpiryDate)
	}

	// Invalid numeric fields surface an error.
	for _, bad := range []*pb.ProposeFXAgreementRequest{
		{OriginAmount: "x", CounterAmount: "1", Rate: "1"},
		{OriginAmount: "1", CounterAmount: "x", Rate: "1"},
		{OriginAmount: "1", CounterAmount: "1", Rate: "x"},
	} {
		if _, err := buildFXProposalParams("T", bad); err == nil {
			t.Errorf("expected error for invalid params %+v", bad)
		}
	}
}

func TestResolveFXPartyAddresses(t *testing.T) {
	ctx := context.Background()

	// nil zeto → pass-through unchanged.
	req := &pb.ProposeFXAgreementRequest{Originator: "alice@spoke-a-bank-a"}
	out, err := resolveFXPartyAddresses(ctx, req, nil)
	if err != nil || out.Originator != "alice@spoke-a-bank-a" {
		t.Fatalf("pass-through failed: out=%+v err=%v", out, err)
	}

	// With a zeto resolver, identity fields are resolved while 0x-addresses and
	// empties are left untouched.
	z := &resolveZeto{}
	req2 := &pb.ProposeFXAgreementRequest{
		Originator:      "alice@spoke-a-bank-a",
		CounterpartyB:   "0xabcabcabcabcabcabcabcabcabcabcabcabcabca",
		SettlementAgent: "",
		Custodian:       "carol@spoke-a-bank-c",
		Beneficiary:     "dave@spoke-a-bank-d",
	}
	out2, err := resolveFXPartyAddresses(ctx, req2, z)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if out2.Originator != "0x1111111111111111111111111111111111111111" {
		t.Errorf("originator not resolved: %q", out2.Originator)
	}
	if out2.CounterpartyB != req2.CounterpartyB {
		t.Errorf("0x address should be left unchanged, got %q", out2.CounterpartyB)
	}
	if out2.SettlementAgent != "" {
		t.Errorf("empty field should stay empty, got %q", out2.SettlementAgent)
	}
}

// resolveZeto is a minimal ZetoOperator for white-box helper tests that only
// need ResolveIdentity to return a deterministic fake address.
type resolveZeto struct{}

func (resolveZeto) Mint(context.Context, string, string) (string, error)     { return "tx", nil }
func (resolveZeto) Burn(context.Context, string, string) (string, error)     { return "tx", nil }
func (resolveZeto) Transfer(context.Context, string, string) (string, error) { return "tx", nil }
func (resolveZeto) Lock(context.Context, string, string) (*ports.ZetoLockResult, error) {
	return &ports.ZetoLockResult{}, nil
}
func (resolveZeto) Unlock(context.Context, string) (string, error) { return "tx", nil }
func (resolveZeto) TransferLocked(context.Context, string, string, string) (string, error) {
	return "tx", nil
}
func (resolveZeto) Balance(context.Context) (string, error) { return "0", nil }
func (resolveZeto) ResolveIdentity(_ context.Context, identity string) (string, error) {
	if identity == "" {
		return "", nil
	}
	return "0x1111111111111111111111111111111111111111", nil
}
