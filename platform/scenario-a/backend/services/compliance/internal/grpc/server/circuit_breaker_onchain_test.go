// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/amm"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeChain models the on-chain AutomatedMarketMaker circuit-breaker state shared
// by all Central Banks. It faithfully reproduces the asymmetric contract semantics:
// pause is a 1-of-N flip that opens a fresh epoch, and resume needs signatures from
// two DISTINCT signers before the breaker clears. A signer may sign only once per
// epoch (mirrors AMM__AlreadySigned).
type fakeChain struct {
	paused bool
	signed map[string]bool
}

func newFakeChain() *fakeChain { return &fakeChain{signed: map[string]bool{}} }

// fakeBreaker is one Central Bank's view of the shared chain, bound to that bank's
// governance signer. Separate compliance instances hold separate fakeBreakers over
// the same fakeChain — exactly the multi-instance topology of production.
type fakeBreaker struct {
	signer string
	chain  *fakeChain
}

func (f *fakeBreaker) Pause(_ context.Context, _ string) (string, error) {
	if f.chain.paused {
		return "", errors.New("already paused")
	}
	f.chain.paused = true
	f.chain.signed = map[string]bool{} // new pause epoch → clear prior signatures
	return "0xpause", nil
}

func (f *fakeBreaker) ResumeVote(_ context.Context) (string, error) {
	if !f.chain.paused {
		return "", amm.ErrNotPaused
	}
	if f.chain.signed[f.signer] {
		return "", errors.New("already signed this resume proposal")
	}
	f.chain.signed[f.signer] = true
	if len(f.chain.signed) >= 2 { // RESUME_QUORUM
		f.chain.paused = false
	}
	return "0xvote", nil
}

func (f *fakeBreaker) IsPaused(_ context.Context) (bool, error) { return f.chain.paused, nil }

func newChainService(breaker amm.Breaker) *complianceService {
	return &complianceService{repo: repository.NewMemoryRepository(), breaker: breaker}
}

func actorCtx(subject string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-actor-subject", subject))
}

// A single Central Bank pauses on-chain (1-of-N) and the DB mirrors the chain.
func TestOnChainBreaker_Pause_OneOfN(t *testing.T) {
	t.Parallel()
	chain := newFakeChain()
	svc := newChainService(&fakeBreaker{signer: "cb-a", chain: chain})

	resp, err := svc.ToggleCircuitBreaker(actorCtx("cb-a-op"), &compliancv1.ToggleCircuitBreakerRequest{Pause: true, Reason: "liquidity anomaly"})
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !resp.IsPaused {
		t.Error("expected paused after 1-of-N pause")
	}
	if resp.TxHash == "" {
		t.Error("expected a tx hash for the on-chain pause")
	}
	if !chain.paused {
		t.Error("chain should be paused")
	}

	// Status reflects the on-chain truth.
	st, _ := svc.GetCircuitBreakerStatus(actorCtx("cb-a-op"), nil)
	if !st.IsPaused {
		t.Error("status must report the on-chain paused state")
	}
}

// A single actor can never resume: the first vote leaves the breaker engaged, and a
// second vote from the SAME signer is rejected (no self-quorum).
func TestOnChainBreaker_SingleActorCannotResume(t *testing.T) {
	t.Parallel()
	chain := newFakeChain()
	svc := newChainService(&fakeBreaker{signer: "cb-a", chain: chain})

	if _, err := svc.ToggleCircuitBreaker(actorCtx("cb-a-op"), &compliancv1.ToggleCircuitBreakerRequest{Pause: true, Reason: "halt"}); err != nil {
		t.Fatalf("pause: %v", err)
	}

	// First resume vote: recorded, but breaker stays engaged (quorum not reached).
	resp, err := svc.ToggleCircuitBreaker(actorCtx("cb-a-op"), &compliancv1.ToggleCircuitBreakerRequest{Pause: false, Reason: "resolved"})
	if err != nil {
		t.Fatalf("first resume vote: %v", err)
	}
	if !resp.IsPaused {
		t.Error("breaker must remain paused after a single resume vote")
	}

	// Second vote from the same actor is rejected — one key cannot form quorum.
	_, err = svc.ToggleCircuitBreaker(actorCtx("cb-a-op"), &compliancv1.ToggleCircuitBreakerRequest{Pause: false, Reason: "resolved again"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition on repeat self-vote, got %v", err)
	}
	if !chain.paused {
		t.Error("a single key must never resume the breaker")
	}
}

// Two DISTINCT Central Banks (separate instances, shared chain) reach the 2-of-N
// quorum and the breaker clears.
func TestOnChainBreaker_TwoDistinctActorsReachQuorum(t *testing.T) {
	t.Parallel()
	chain := newFakeChain()
	svcA := newChainService(&fakeBreaker{signer: "cb-a", chain: chain})
	svcB := newChainService(&fakeBreaker{signer: "cb-b", chain: chain})

	if _, err := svcA.ToggleCircuitBreaker(actorCtx("cb-a-op"), &compliancv1.ToggleCircuitBreakerRequest{Pause: true, Reason: "halt"}); err != nil {
		t.Fatalf("pause: %v", err)
	}

	// CB-A votes to resume → still paused.
	respA, err := svcA.ToggleCircuitBreaker(actorCtx("cb-a-op"), &compliancv1.ToggleCircuitBreakerRequest{Pause: false, Reason: "resolved"})
	if err != nil {
		t.Fatalf("cb-a vote: %v", err)
	}
	if !respA.IsPaused {
		t.Error("still paused after the first distinct vote")
	}

	// CB-B votes → quorum reached → resumed.
	respB, err := svcB.ToggleCircuitBreaker(actorCtx("cb-b-op"), &compliancv1.ToggleCircuitBreakerRequest{Pause: false, Reason: "resolved"})
	if err != nil {
		t.Fatalf("cb-b vote: %v", err)
	}
	if respB.IsPaused {
		t.Error("breaker must clear once two distinct governors vote to resume")
	}
	if chain.paused {
		t.Error("chain should be resumed at 2-of-N quorum")
	}
}

// Reason is still required before any chain interaction.
func TestOnChainBreaker_MissingReason(t *testing.T) {
	t.Parallel()
	chain := newFakeChain()
	svc := newChainService(&fakeBreaker{signer: "cb-a", chain: chain})
	_, err := svc.ToggleCircuitBreaker(context.Background(), &compliancv1.ToggleCircuitBreakerRequest{Pause: true})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if chain.paused {
		t.Error("no chain call should happen when validation fails")
	}
}
