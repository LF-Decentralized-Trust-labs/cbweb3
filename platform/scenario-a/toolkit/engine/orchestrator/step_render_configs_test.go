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

func TestRenderConfigsStep_Run_RendersThreeConfigs(t *testing.T) {
	dir := t.TempDir()

	// Create minimal template files.
	cbTmplDir := filepath.Join(dir, "tmpl", "central-bank")
	bankTmplDir := filepath.Join(dir, "tmpl", "bank")
	os.MkdirAll(cbTmplDir, 0o755)
	os.MkdirAll(bankTmplDir, 0o755)
	cbTmpl := `spokeid: {{.SpokeID}} rpc: {{.BesuRPCPort}}`
	bankTmpl := `spokeid: {{.SpokeID}} node: {{.NodeName}}`
	os.WriteFile(filepath.Join(cbTmplDir, "config.yaml.tmpl"), []byte(cbTmpl), 0o644)
	os.WriteFile(filepath.Join(bankTmplDir, "config.yaml.tmpl"), []byte(bankTmpl), 0o644)

	// Write a minimal .deployed-addrs.env.
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(
		"REGISTRY_CONTRACT_ADDRESS=0xREG\nZETO_FACTORY_ADDRESS=0xZF\nPENTE_FACTORY_ADDRESS=0xPF\n",
	), 0o644)

	step := newRenderConfigsStep("spoke-test", dir, 8645, 8655, filepath.Join(dir, "tmpl"))
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, node := range []string{"central-bank", "bank-a", "bank-c"} {
		cfgPath := filepath.Join(dir, "paladin", node, "config.yaml")
		if _, err := os.Stat(cfgPath); err != nil {
			t.Errorf("expected config for node %s at %s: %v", node, cfgPath, err)
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
