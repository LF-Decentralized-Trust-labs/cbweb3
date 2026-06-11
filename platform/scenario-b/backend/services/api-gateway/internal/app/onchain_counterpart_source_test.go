package app

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

type fakeCommitReader struct {
	pendingFor map[uint8][32]byte // side -> commitId
	commits    map[[32]byte]CommitView
}

func (f *fakeCommitReader) GetPendingCommit(_ context.Context, _ string, side uint8) ([32]byte, error) {
	return f.pendingFor[side], nil
}

func (f *fakeCommitReader) GetCommit(_ context.Context, id [32]byte) (CommitView, error) {
	return f.commits[id], nil
}

func TestCounterpartSource_QueriesOppositeSide(t *testing.T) {
	commitID := [32]byte{0xab}
	signer := common.HexToAddress("0xf17f52151EbEF6C7334FAD080c5704D77216b732")

	// This gateway is side A; the counterpart commit lives on side B.
	reader := &fakeCommitReader{
		pendingFor: map[uint8][32]byte{1: commitID},
		commits: map[[32]byte]CommitView{
			commitID: {Signer: signer, Amount: big.NewInt(100000), ExpiresAt: 1893456000, Status: commitStatusPending, Side: 1},
		},
	}
	src := newOnChainCounterpartSource(reader, "A")

	cp, err := src.CounterpartCommit(context.Background(), "W-BRL-ARS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cp == nil {
		t.Fatal("expected counterpart commit, got nil")
	}
	if cp.Side != "B" {
		t.Errorf("expected side B, got %q", cp.Side)
	}
	if cp.SignerAddress != signer.Hex() {
		t.Errorf("expected signer %s, got %s", signer.Hex(), cp.SignerAddress)
	}
	if cp.Amount != "100000" {
		t.Errorf("expected amount 100000, got %s", cp.Amount)
	}
	if cp.OnChainCommitID != CommitIDToHex(commitID) {
		t.Errorf("expected commitId %s, got %s", CommitIDToHex(commitID), cp.OnChainCommitID)
	}
}

func TestCounterpartSource_NoPendingCommit(t *testing.T) {
	src := newOnChainCounterpartSource(&fakeCommitReader{}, "B")
	cp, err := src.CounterpartCommit(context.Background(), "W-BRL-ARS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cp != nil {
		t.Errorf("expected nil counterpart, got %+v", cp)
	}
}

func TestCounterpartSource_NonPendingStatusIgnored(t *testing.T) {
	commitID := [32]byte{0x01}
	reader := &fakeCommitReader{
		pendingFor: map[uint8][32]byte{0: commitID},
		commits: map[[32]byte]CommitView{
			commitID: {Amount: big.NewInt(1), Status: 1 /* MATCHED */, Side: 0},
		},
	}
	// Gateway is side B, so it queries side A; commit there is MATCHED, not PENDING.
	src := newOnChainCounterpartSource(reader, "B")
	cp, err := src.CounterpartCommit(context.Background(), "W-BRL-ARS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cp != nil {
		t.Errorf("expected nil for non-PENDING commit, got %+v", cp)
	}
}
