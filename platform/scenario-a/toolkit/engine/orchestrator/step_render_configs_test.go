// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderConfigsStep_Check_False_NoConfig(t *testing.T) {
	dir := t.TempDir()
	step := newRenderConfigsStep("spoke-test", dir, 8645, 8655, "/tmpl")
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when paladin/central-bank/config.yaml does not exist")
	}
}

func TestRenderConfigsStep_Check_True_ConfigExists(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "paladin", "central-bank")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("besu: {}"), 0o644)
	step := newRenderConfigsStep("spoke-test", dir, 8645, 8655, "/tmpl")
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when paladin/central-bank/config.yaml exists")
	}
}

func TestRenderConfigsStep_Run_RendersCBConfigOnly(t *testing.T) {
	dir := t.TempDir()

	// Create minimal template files.
	cbTmplDir := filepath.Join(dir, "tmpl", "central-bank")
	os.MkdirAll(cbTmplDir, 0o755)
	cbTmpl := `spokeid: {{.SpokeID}} rpc: {{.BesuRPCPort}}`
	os.WriteFile(filepath.Join(cbTmplDir, "config.yaml.tmpl"), []byte(cbTmpl), 0o644)

	// Write a minimal .deployed-addrs.env.
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(
		"REGISTRY_CONTRACT_ADDRESS=0xREG\nZETO_FACTORY_ADDRESS=0xZF\nPENTE_FACTORY_ADDRESS=0xPF\n",
	), 0o644)

	step := newRenderConfigsStep("spoke-test", dir, 8645, 8655, filepath.Join(dir, "tmpl"))
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// found is CB-only: central-bank config rendered; no bank-a/bank-c.
	cbPath := filepath.Join(dir, "paladin", "central-bank", "config.yaml")
	if _, err := os.Stat(cbPath); err != nil {
		t.Errorf("expected central-bank config at %s: %v", cbPath, err)
	}
	for _, node := range []string{"bank-a", "bank-c"} {
		if _, err := os.Stat(filepath.Join(dir, "paladin", node, "config.yaml")); err == nil {
			t.Errorf("found must NOT render config for %s (CB-only)", node)
		}
	}
}

func TestRenderConfigsStep_Run_ErrorsOnMissingTemplate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	step := newRenderConfigsStep("spoke-test", dir, 8645, 8655, filepath.Join(dir, "nonexistent-tmpl"))
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when template directory does not exist")
	}
}
