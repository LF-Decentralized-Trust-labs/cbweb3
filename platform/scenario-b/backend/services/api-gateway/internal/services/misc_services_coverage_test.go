// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"

	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ---------------------------------------------------------------------------
// PoolStatusGate
// ---------------------------------------------------------------------------

type fakePoolReader struct {
	rA, rB string
	err    error
}

func (f fakePoolReader) GetPoolReserves(_ context.Context, _ string) (string, string, float64, error) {
	return f.rA, f.rB, 0, f.err
}
func (f fakePoolReader) GetFeeBps(_ context.Context) (uint64, error) { return 30, nil }

func TestPoolStatusGate_IsActive(t *testing.T) {
	g := NewPoolStatusGate(fakePoolReader{rA: "100", rB: "200"})
	active, err := g.IsActive(context.Background(), "P")
	if err != nil || !active {
		t.Fatalf("expected active, got %v err %v", active, err)
	}

	g = NewPoolStatusGate(fakePoolReader{rA: "0", rB: "200"})
	active, _ = g.IsActive(context.Background(), "P")
	if active {
		t.Fatal("expected inactive when reserve A is zero")
	}

	g = NewPoolStatusGate(fakePoolReader{err: errors.New("rpc")})
	if _, err := g.IsActive(context.Background(), "P"); err == nil {
		t.Fatal("expected propagated error")
	}
}

// ---------------------------------------------------------------------------
// CircuitBreakerGate (DB-backed)
// ---------------------------------------------------------------------------

func TestCircuitBreakerGate_IsHalted(t *testing.T) {
	db := newTestDB(t, &domain.ScenarioBRiskControlState{})
	g := NewCircuitBreakerGate(db)

	// No record → not halted (allow by default).
	halted, err := g.IsHalted(context.Background(), "P")
	if err != nil || halted {
		t.Fatalf("expected not halted with no record, got %v err %v", halted, err)
	}

	db.Create(&domain.ScenarioBRiskControlState{PoolPair: "P", CircuitBreakerState: domain.CircuitBreakerHalted})
	halted, err = g.IsHalted(context.Background(), "P")
	if err != nil || !halted {
		t.Fatalf("expected halted, got %v err %v", halted, err)
	}

	db.Create(&domain.ScenarioBRiskControlState{PoolPair: "Q", CircuitBreakerState: domain.CircuitBreakerLive})
	halted, _ = g.IsHalted(context.Background(), "Q")
	if halted {
		t.Fatal("expected not halted for LIVE pool")
	}
}

// ---------------------------------------------------------------------------
// CircuitBreakerService.GetStatus (DB-backed)
// ---------------------------------------------------------------------------

func TestCircuitBreakerService_GetStatus(t *testing.T) {
	db := newTestDB(t, &domain.ScenarioBRiskControlState{})
	svc := NewCircuitBreakerService(db, nil)

	// No record → defaults to LIVE.
	st, err := svc.GetStatus(context.Background(), "P")
	if err != nil || st.State != string(domain.CircuitBreakerLive) {
		t.Fatalf("expected default LIVE, got %+v err %v", st, err)
	}

	db.Create(&domain.ScenarioBRiskControlState{
		PoolPair:            "Q",
		CircuitBreakerState: domain.CircuitBreakerHalted,
		PauseInitiatorBankID: "bank-a",
		PauseReasonCode:     "FRAUD",
	})
	st, err = svc.GetStatus(context.Background(), "Q")
	if err != nil || st.State != string(domain.CircuitBreakerHalted) || st.PauseInitiator != "bank-a" {
		t.Fatalf("unexpected status: %+v err %v", st, err)
	}
}

// ---------------------------------------------------------------------------
// ParticipantResolver (DB-backed)
// ---------------------------------------------------------------------------

func TestParticipantResolver(t *testing.T) {
	db := newTestDB(t, &participantRow{})
	r := NewParticipantResolver(db)

	if _, err := r.ResolveWalletAddress(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty bank_code")
	}
	if _, err := r.ResolveWalletAddress(context.Background(), "missing"); err == nil {
		t.Fatal("expected not-found error")
	}

	db.Create(&participantRow{BankCode: "bank-a", WalletAddress: "0xabc", Status: "ACTIVE"})
	addr, err := r.ResolveWalletAddress(context.Background(), "bank-a")
	if err != nil || addr != "0xabc" {
		t.Fatalf("expected 0xabc, got %s err %v", addr, err)
	}

	db.Create(&participantRow{BankCode: "bank-b", WalletAddress: "0xdef", Status: "SUSPENDED"})
	if _, err := r.ResolveWalletAddress(context.Background(), "bank-b"); err == nil {
		t.Fatal("expected not-ACTIVE error")
	}

	db.Create(&participantRow{BankCode: "bank-c", WalletAddress: "", Status: "ACTIVE"})
	if _, err := r.ResolveWalletAddress(context.Background(), "bank-c"); err == nil {
		t.Fatal("expected empty-wallet error")
	}
}

// ---------------------------------------------------------------------------
// OversightService (DB-backed)
// ---------------------------------------------------------------------------

func TestOversightService(t *testing.T) {
	db := newTestDB(t, &domain.DisclosureRequest{}, &domain.DisclosureSignature{})
	svc := NewOversightService(db)

	if _, err := svc.OpenDisclosure(context.Background(), "", "req", "code"); err == nil {
		t.Fatal("expected validation error")
	}

	res, err := svc.OpenDisclosure(context.Background(), "tx-1", "bank-a", "AUDIT")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if res.State != string(domain.DisclosurePending) || res.QuorumRequired != 2 {
		t.Fatalf("unexpected disclosure: %+v", res)
	}

	// First signature → still pending.
	if err := svc.SignDisclosure(context.Background(), res.RequestID, "signer-1"); err != nil {
		t.Fatalf("sign 1 failed: %v", err)
	}
	// Duplicate signature rejected.
	if err := svc.SignDisclosure(context.Background(), res.RequestID, "signer-1"); err == nil {
		t.Fatal("expected duplicate-signature error")
	}
	// Second distinct signature → quorum reached.
	if err := svc.SignDisclosure(context.Background(), res.RequestID, "signer-2"); err != nil {
		t.Fatalf("sign 2 failed: %v", err)
	}

	status, err := svc.GetDisclosureStatus(context.Background(), res.RequestID)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status.State != string(domain.DisclosureQuorumReached) || status.QuorumReached < 2 {
		t.Fatalf("expected quorum reached, got %+v", status)
	}

	// Signing a non-pending request fails.
	if err := svc.SignDisclosure(context.Background(), res.RequestID, "signer-3"); err == nil {
		t.Fatal("expected not-PENDING error")
	}

	// Validation + not-found paths.
	if err := svc.SignDisclosure(context.Background(), "", ""); err == nil {
		t.Fatal("expected validation error")
	}
	if err := svc.SignDisclosure(context.Background(), "nope", "s"); err == nil {
		t.Fatal("expected not-found error")
	}
	if _, err := svc.GetDisclosureStatus(context.Background(), "nope"); err == nil {
		t.Fatal("expected not-found error")
	}
}

// ---------------------------------------------------------------------------
// PairRouter
// ---------------------------------------------------------------------------

type fakePairSource struct {
	pairs []domain.PairEntry
	err   error
}

func (f fakePairSource) GetAllActivePairs(_ context.Context) ([]domain.PairEntry, error) {
	return f.pairs, f.err
}

func TestPairRouter(t *testing.T) {
	factory := func(_ context.Context, ammAddress string) (AMMPoolReader, error) {
		if ammAddress == "0xbad" {
			return nil, errors.New("bad amm")
		}
		return fakePoolReader{rA: "1", rB: "1"}, nil
	}

	src := fakePairSource{pairs: []domain.PairEntry{
		{PairID: "BRL-USD", AMMAddress: "0xamm1"},
		{PairID: "ARS-USD", AMMAddress: "0xbad"}, // factory error → skipped
	}}
	r, err := NewPairRouter(context.Background(), src, factory)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := r.ClientFor("BRL-USD"); err != nil {
		t.Fatalf("expected client for BRL-USD, got %v", err)
	}
	if _, err := r.ClientFor("ARS-USD"); err == nil {
		t.Fatal("expected ARS-USD to be skipped (factory failed)")
	}
	if _, err := r.ClientFor("UNKNOWN"); err == nil {
		t.Fatal("expected not-found for unknown pair")
	}

	r.AddPair(context.Background(), "EUR-USD", "0xamm2")
	if _, err := r.ClientFor("EUR-USD"); err != nil {
		t.Fatalf("expected newly added pair, got %v", err)
	}
	// AddPair with failing factory → not added.
	r.AddPair(context.Background(), "JPY-USD", "0xbad")
	if _, err := r.ClientFor("JPY-USD"); err == nil {
		t.Fatal("expected JPY-USD not added (factory failed)")
	}

	ids := r.ListPairIDs()
	if len(ids) != 2 { // BRL-USD + EUR-USD
		t.Fatalf("expected 2 pair IDs, got %d: %v", len(ids), ids)
	}
}

func TestPairRouter_LoadError(t *testing.T) {
	factory := func(_ context.Context, _ string) (AMMPoolReader, error) { return nil, nil }
	if _, err := NewPairRouter(context.Background(), fakePairSource{err: errors.New("rpc")}, factory); err == nil {
		t.Fatal("expected load error")
	}
}

func TestPairRouter_WatchActivations(t *testing.T) {
	factory := func(_ context.Context, _ string) (AMMPoolReader, error) {
		return fakePoolReader{rA: "1", rB: "1"}, nil
	}
	r, _ := NewPairRouter(context.Background(), fakePairSource{}, factory)

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan PairActivatedEvent, 1)
	r.WatchActivations(ctx, ch)
	ch <- PairActivatedEvent{PairID: "NEW-PAIR", AMMAddress: "0xamm"}

	// Poll until the watcher goroutine processes the event.
	registered := false
	for i := 0; i < 200; i++ {
		if _, err := r.ClientFor("NEW-PAIR"); err == nil {
			registered = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !registered {
		t.Fatal("watch activation did not register pair in time")
	}
	cancel()
	close(ch)
}
