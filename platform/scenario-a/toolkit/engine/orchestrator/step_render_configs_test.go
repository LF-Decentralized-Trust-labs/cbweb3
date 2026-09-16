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

// The log level must be an environment override with an info default — the whole
// point of the change that removed the hardcoded debug. Both directions are pinned
// because each fails independently: a default that silently reverts to debug
// reopens the 96 GB incident, and an override that no longer reaches the template
// takes debug away from whoever is mid-investigation.
func TestPaladinLogLevel_DefaultsToInfoAndHonoursTheOverride(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("PALADIN_LOG_LEVEL", "")
		if got := paladinLogLevel(); got != "info" {
			t.Errorf("paladinLogLevel() = %q with no override, want %q", got, "info")
		}
	})

	t.Run("override", func(t *testing.T) {
		t.Setenv("PALADIN_LOG_LEVEL", "debug")
		if got := paladinLogLevel(); got != "debug" {
			t.Errorf("paladinLogLevel() = %q with PALADIN_LOG_LEVEL=debug, want %q — debug must "+
				"stay reachable without editing a template", got, "debug")
		}
	})

	// Whitespace-only is not an override. Without the trim it would pass a blank
	// level straight into Paladin's config, which is a startup failure rather than
	// a default.
	t.Run("blank is not an override", func(t *testing.T) {
		t.Setenv("PALADIN_LOG_LEVEL", "   ")
		if got := paladinLogLevel(); got != "info" {
			t.Errorf("paladinLogLevel() = %q for a blank override, want %q", got, "info")
		}
	})
}
