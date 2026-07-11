package apply

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/orchestrator"
)

const fixtures = "../manifest/testdata"

// SC-004: dry-run produces a report of planned steps without external effects.
func TestApplyFoundHubDryRun(t *testing.T) {
	rep, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(fixtures, "found-hub.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("apply dry-run: %v", err)
	}
	if rep.Mode != "found-hub" || len(rep.Steps) == 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	for _, s := range rep.Steps {
		if s.Status != orchestrator.StatusPlanned && s.Status != orchestrator.StatusSkipped {
			t.Fatalf("dry-run step %s has status %s (want planned/skipped)", s.Name, s.Status)
		}
	}
}

// TK-B7: found-spoke dry-run plans the steps (hub bundle validated, no effects).
func TestApplyFoundSpokeDryRun(t *testing.T) {
	rep, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(fixtures, "found-spoke.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("apply found-spoke dry-run: %v", err)
	}
	if rep.Mode != "found-spoke" || len(rep.Steps) == 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	for _, s := range rep.Steps {
		if s.Status != orchestrator.StatusPlanned && s.Status != orchestrator.StatusSkipped {
			t.Fatalf("dry-run step %s status %s", s.Name, s.Status)
		}
	}
}

// SC-008: join mode is rejected with a clear error.
func TestApplyJoinUnsupported(t *testing.T) {
	_, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(fixtures, "join.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "not supported yet") {
		t.Fatalf("expected 'not supported yet', got %v", err)
	}
}

// SC-008: invalid/missing manifest is rejected before any effect.
func TestApplyMissingManifest(t *testing.T) {
	_, err := Apply(context.Background(), Options{
		ManifestPath: filepath.Join(t.TempDir(), "nope.yaml"),
		DataDir:      t.TempDir(),
		DryRun:       true,
	})
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestRenderFormats(t *testing.T) {
	r := orchestrator.Report{Mode: "found-hub", Steps: []orchestrator.StepResult{{Name: "s", Status: "planned"}}}
	j, err := Render(r, "json")
	if err != nil || !strings.Contains(string(j), "\"mode\": \"found-hub\"") {
		t.Fatalf("json render: %v\n%s", err, j)
	}
	y, err := Render(r, "yaml")
	if err != nil || !strings.Contains(string(y), "mode: found-hub") {
		t.Fatalf("yaml render: %v\n%s", err, y)
	}
	if _, err := Render(r, "xml"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
