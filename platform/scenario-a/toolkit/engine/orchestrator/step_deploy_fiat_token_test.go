// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeployFiatTokenStep_Check_False_NoAddr(t *testing.T) {
	dir := t.TempDir()
	step := newDeployFiatTokenStep("spoke-test", dir, "http://localhost:8645", "BRL", nil, "/artifact.json", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should be false when FIAT_TOKEN_ADDRESS is absent")
	}
}

func TestDeployFiatTokenStep_Check_True_AddrPresent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("FIAT_TOKEN_ADDRESS=0xF1A7\n"), 0o644)
	step := newDeployFiatTokenStep("spoke-test", dir, "http://localhost:8645", "BRL", nil, "/artifact.json", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should be true when FIAT_TOKEN_ADDRESS is present")
	}
}

func TestDeployFiatTokenStep_Name(t *testing.T) {
	step := newDeployFiatTokenStep("spoke-test", t.TempDir(), "", "BRL", nil, "", 0)
	if step.Name() != StepDeployFiatToken {
		t.Errorf("Name() = %q, want %q", step.Name(), StepDeployFiatToken)
	}
}
