// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// TestProofPossessionStep_CheckNoKey verifies Check returns (false, nil) when the
// bank's blockchain key has not been generated yet (not registered).
func TestProofPossessionStep_CheckNoKey(t *testing.T) {
	kp := keyprovider.NewLocalKeyProvider()
	step := newProofPossessionStep("bank-x", "0x1234", "http://localhost:1", kp, time.Second)

	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: unexpected error: %v", err)
	}
	if done {
		t.Error("Check should be false when no key exists / registry unreachable")
	}
}

// TestProofPossessionStep_CheckRegistryUnreachable verifies Check degrades to
// (false, nil) when a key exists but the registry cannot be dialed.
func TestProofPossessionStep_CheckRegistryUnreachable(t *testing.T) {
	kp := keyprovider.NewLocalKeyProvider()
	if _, err := kp.GenerateKey(context.Background(), "bank-x"); err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	step := newProofPossessionStep("bank-x", "0x1234", "http://127.0.0.1:1", kp, time.Second)

	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: unexpected error: %v", err)
	}
	if done {
		t.Error("Check should be false when registry is unreachable")
	}
}

func TestProofPossessionStep_Name(t *testing.T) {
	step := newProofPossessionStep("bank-x", "0x1", "http://x", keyprovider.NewLocalKeyProvider(), time.Second)
	if step.Name() != StepProofPossession {
		t.Errorf("Name() = %q, want %q", step.Name(), StepProofPossession)
	}
}
