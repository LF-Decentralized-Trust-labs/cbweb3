// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderConfigsStep_Check_False_NoConfig(t *testing.T) {
	requireDocker(t)
	dir := t.TempDir()
	step := newRenderConfigsStep("spoke-test-renderconfigs-nocfg", dir, 8645, 8655, "/tmpl").(*renderConfigsStep)
	cleanupVolume(t, step.paladinConfigVolume())

	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when config.yaml does not exist in the volume")
	}
}

func TestRenderConfigsStep_Check_True_ConfigExists(t *testing.T) {
	requireDocker(t)
	dir := t.TempDir()
	step := newRenderConfigsStep("spoke-test-renderconfigs-exists", dir, 8645, 8655, "/tmpl").(*renderConfigsStep)
	cleanupVolume(t, step.paladinConfigVolume())

	if err := writeVolumeFile(context.Background(), step.paladinConfigVolume(), "config.yaml", []byte("besu: {}"), "0644"); err != nil {
		t.Fatal(err)
	}
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when config.yaml exists in the volume")
	}
}

func TestRenderConfigsStep_Run_RendersCBConfigOnly(t *testing.T) {
	requireDocker(t)
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

	step := newRenderConfigsStep("spoke-test-renderconfigs-run", dir, 8645, 8655, filepath.Join(dir, "tmpl")).(*renderConfigsStep)
	cleanupVolume(t, step.paladinConfigVolume())

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// found is CB-only: central-bank config rendered into the volume.
	data, err := readVolumeFile(context.Background(), step.paladinConfigVolume(), "config.yaml")
	if err != nil {
		t.Fatalf("expected central-bank config in volume %s: %v", step.paladinConfigVolume(), err)
	}
	if len(data) == 0 {
		t.Error("rendered config.yaml is empty")
	}
}

func TestRenderConfigsStep_Run_ErrorsOnMissingTemplate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	step := newRenderConfigsStep("spoke-test-renderconfigs-missingtmpl", dir, 8645, 8655, filepath.Join(dir, "nonexistent-tmpl"))
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when template directory does not exist")
	}
}
