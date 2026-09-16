// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"strings"
	"testing"

	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

// TestLockHTLC_AcceptsAReceiverOnAHyphenatedSpoke covers the WIRING of the local-spoke
// gate, not just the primitive underneath it. Nothing here existed before, and its
// absence is why a regression in this exact line survived: with SpokePrefix on both
// sides the comparison was two identical wrong guesses, so every unit fixture
// ("spoke-a-bank-a") agreed and passed. Reading the local side from SPOKE_ID made it
// correct and the two stopped agreeing — a bank could no longer lock to its own
// neighbour on spoke-costa-rica, which is a live LNET spoke.
func TestLockHTLC_AcceptsAReceiverOnAHyphenatedSpoke(t *testing.T) {
	const spoke = "spoke-costa-rica"
	env := setupTestEnvFull(t, &mockHTLC{}, nil, spoke)
	defer env.cancel()

	_, err := env.client.LockHTLC(context.Background(), &pb.LockHTLCRequest{
		Receiver: "funded_operator@" + spoke + "-cb2",
		Amount:   "5000",
	})
	if err != nil && strings.Contains(err.Error(), "does not belong to the local spoke") {
		t.Fatalf("a receiver on the local spoke was rejected as foreign: %v", err)
	}
}

// TestLockHTLC_RejectsAReceiverOnAnotherSpoke is the other half: the gate still has to
// do its job. A fix that simply accepted everything would pass the test above.
func TestLockHTLC_RejectsAReceiverOnAnotherSpoke(t *testing.T) {
	env := setupTestEnvFull(t, &mockHTLC{}, nil, "spoke-costa-rica")
	defer env.cancel()

	for _, receiver := range []string{
		"funded_operator@spoke-brl-bank-itau",
		"funded_operator@spoke-peru-cb5",
		"funded_operator@spoke-chile-cb",
	} {
		_, err := env.client.LockHTLC(context.Background(), &pb.LockHTLCRequest{
			Receiver: receiver, Amount: "5000",
		})
		if err == nil || !strings.Contains(err.Error(), "does not belong to the local spoke") {
			t.Errorf("receiver %q on another spoke must be rejected, got err=%v", receiver, err)
		}
	}
}

// TestLockHTLC_TwoSegmentSpokeStillWorks guards the conventions that were already
// fine, so the fix cannot trade one shape for the other.
func TestLockHTLC_TwoSegmentSpokeStillWorks(t *testing.T) {
	env := setupTestEnvFull(t, &mockHTLC{}, nil, "spoke-brl")
	defer env.cancel()

	_, err := env.client.LockHTLC(context.Background(), &pb.LockHTLCRequest{
		Receiver: "funded_operator@spoke-brl-bank-bradesco",
		Amount:   "1000",
	})
	if err != nil && strings.Contains(err.Error(), "does not belong to the local spoke") {
		t.Fatalf("a receiver on spoke-brl was rejected as foreign: %v", err)
	}
}
