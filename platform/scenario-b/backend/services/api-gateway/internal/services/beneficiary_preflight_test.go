// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// The condition that rejects a delivery is known only to the beneficiary's central bank, and it
// used to be consulted only at the delivery — after bridge-in and after the AMM swap. The payer
// was debited and the swap executed before anyone found out. These tests pin the check that now
// runs before any of that, and the deliberate decision about what to do when it goes unanswered.

// preflightRelay answers a fixed pre-flight verdict and records whether the delivery was ever
// attempted, which is how "nothing moved" is expressed.
type preflightRelay struct {
	answer        BeneficiaryPreflight
	askedSpoke    string
	askedBank     string
	deliveryCalls int
}

func (p *preflightRelay) CheckBeneficiary(_ context.Context, spokeOut, bankID string) BeneficiaryPreflight {
	p.askedSpoke, p.askedBank = spokeOut, bankID
	return p.answer
}

func (p *preflightRelay) NotifyBridgeOut(_ context.Context, _ CactiCrossCurrencyBridgeOutRequest) (string, error) {
	p.deliveryCalls++
	return "echo", nil
}

func preflightOrchestrator(repo CrossCurrencySwapRepository, relay *preflightRelay) *CrossCurrencySwapOrchestrator {
	return NewCrossCurrencySwapOrchestrator(
		repo,
		nil,
		&stubLockMint{},
		stubBurnUnlock{},
		&recordingSwapService{},
		stubPoolActive{},
		stubCBOK{},
		nil, nil, nil,
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithHubSwapRelay(&stubHubSwapRelay{result: &SwapResult{
			TxHash: "0xswap", AmountIn: "800", HubSenderAddress: "0xCBHUB",
		}}).
		WithCactiRelay(relay)
}

// TestPreflight_RefusesBeforeAnyValueMoves is the whole point: a definite "no" stops the
// operation while the payer still has their money and the pool is untouched.
func TestPreflight_RefusesBeforeAnyValueMoves(t *testing.T) {
	relay := &preflightRelay{answer: BeneficiaryPreflight{
		Answered: true, Eligible: false, Code: EligibilityNotActive,
	}}
	repo := &capturingSwapRepo{}

	_, err := preflightOrchestrator(repo, relay).Execute(context.Background(), hubSwapRequest())

	if err == nil {
		t.Fatal("an ineligible beneficiary must stop the swap")
	}
	if !strings.Contains(err.Error(), "before any value moved") {
		t.Errorf("the refusal must say nothing moved, so an operator does not go looking for "+
			"stranded funds; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), EligibilityNotActive) {
		t.Errorf("the refusal must carry the reason code, got %q", err.Error())
	}
	if relay.deliveryCalls != 0 {
		t.Errorf("delivery attempted %d times after a refusal — the swap should never have started", relay.deliveryCalls)
	}
	// No delivery was attempted, so there is nothing to retry: marking it would put a row in
	// front of the sweeper that can never succeed.
	if repo.status == domain.BridgeOutDeliveryFailed {
		t.Error("a pre-flight refusal must NOT be marked as a retryable delivery — no delivery was made")
	}
}

// TestPreflight_AsksTheRightCentralBank pins the routing: the question goes to the spoke derived
// from the TARGET currency, naming the beneficiary. Asking the wrong CB would get a confident
// "unknown bank" and refuse a perfectly good payment.
func TestPreflight_AsksTheRightCentralBank(t *testing.T) {
	relay := &preflightRelay{answer: BeneficiaryPreflight{Answered: true, Eligible: true, Code: EligibilityOK}}

	preflightOrchestrator(&capturingSwapRepo{}, relay).Execute(context.Background(), hubSwapRequest())

	if relay.askedSpoke != "spoke-cop" {
		t.Errorf("asked spoke %q, want spoke-cop derived from the target currency", relay.askedSpoke)
	}
	if relay.askedBank == "" {
		t.Error("the beneficiary must be named in the question")
	}
}

// TestPreflight_ProceedsWhenUnanswered is the deliberate fail-open, asserted so it is a decision
// on the record rather than a gap someone later "fixes" without knowing why.
//
// Refusing here would let the beneficiary CB's availability decide whether THIS central bank can
// start a payment: one peer down closes the corridor. The guarantee against a stranded delivery
// is the bridge-out retry; this check is an optimisation, and an optimisation must not become an
// outage.
func TestPreflight_ProceedsWhenUnanswered(t *testing.T) {
	relay := &preflightRelay{answer: BeneficiaryPreflight{Answered: false}}

	_, err := preflightOrchestrator(&capturingSwapRepo{}, relay).Execute(context.Background(), hubSwapRequest())

	if err != nil && strings.Contains(err.Error(), "before any value moved") {
		t.Fatal("an unanswered pre-flight must not refuse the payment — a peer being unreachable " +
			"is not the same as a beneficiary being ineligible")
	}
	if relay.deliveryCalls == 0 {
		t.Error("the swap must proceed to the delivery when the question goes unanswered")
	}
}

// TestPreflight_EligibleProceeds guards the other direction: a healthy path must not be slowed
// or blocked by the check that was added to protect it.
func TestPreflight_EligibleProceeds(t *testing.T) {
	relay := &preflightRelay{answer: BeneficiaryPreflight{Answered: true, Eligible: true, Code: EligibilityOK}}

	_, err := preflightOrchestrator(&capturingSwapRepo{}, relay).Execute(context.Background(), hubSwapRequest())

	if err != nil {
		t.Fatalf("an eligible beneficiary must not be blocked: %v", err)
	}
	if relay.deliveryCalls != 1 {
		t.Errorf("delivery calls = %d, want 1", relay.deliveryCalls)
	}
}

// TestPreflight_SkippedWhenTheRelayCannotAnswer covers the optional capability: a relay that
// predates the check keeps working unchanged rather than failing to start.
func TestPreflight_SkippedWhenTheRelayCannotAnswer(t *testing.T) {
	// stubCactiRelay implements only NotifyBridgeOut — no CheckBeneficiary.
	relay := &stubCactiRelay{}

	_, err := NewCrossCurrencySwapOrchestrator(
		&capturingSwapRepo{}, nil, &stubLockMint{}, stubBurnUnlock{}, &recordingSwapService{},
		stubPoolActive{}, stubCBOK{}, nil, nil, nil,
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithHubSwapRelay(&stubHubSwapRelay{result: &SwapResult{TxHash: "0xswap", AmountIn: "800"}}).
		WithCactiRelay(relay).
		Execute(context.Background(), hubSwapRequest())

	if err != nil {
		t.Fatalf("a relay without the pre-flight capability must not break the swap: %v", err)
	}
	if !relay.called {
		t.Error("the delivery must still be attempted")
	}
}
