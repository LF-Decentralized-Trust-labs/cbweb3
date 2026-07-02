// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"os"
	"path/filepath"
	"testing"
)

// makeScenarioTree creates a minimal Scenario A root (with the marker file) and
// returns its path.
func makeScenarioTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	marker := filepath.Join(root, scenarioMarker)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestScenarioRoot_FindsMarkerFromNestedDir(t *testing.T) {
	root := makeScenarioTree(t)
	nested := filepath.Join(root, "toolkit", "cmd", "cbweb3")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := scenarioRoot(nested); got != root {
		t.Errorf("scenarioRoot(%q) = %q; want %q", nested, got, root)
	}
}

func TestScenarioRoot_FindsMarkerFromSiblingDir(t *testing.T) {
	// Binary placed under samples/ (a sibling of toolkit/) must still resolve.
	root := makeScenarioTree(t)
	samples := filepath.Join(root, "samples")
	if err := os.MkdirAll(samples, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := scenarioRoot(samples); got != root {
		t.Errorf("scenarioRoot(%q) = %q; want %q", samples, got, root)
	}
}

func TestScenarioRoot_ReturnsEmptyWhenNoMarker(t *testing.T) {
	dir := t.TempDir()
	if got := scenarioRoot(dir); got != "" {
		t.Errorf("scenarioRoot(%q) = %q; want empty", dir, got)
	}
}

func TestScenarioRoot_TriesAllStartDirsInOrder(t *testing.T) {
	root := makeScenarioTree(t)
	noMarker := t.TempDir()
	// First start dir has no marker; second one resolves.
	if got := scenarioRoot(noMarker, filepath.Join(root, "toolkit")); got != root {
		t.Errorf("scenarioRoot should fall through to the second start dir; got %q want %q", got, root)
	}
}

func TestLocalProfileFromExDir_CBWEB3HomeTakesPrecedence(t *testing.T) {
	home := makeScenarioTree(t)
	t.Setenv("CBWEB3_HOME", home)
	// A bogus exDir must be ignored when CBWEB3_HOME is set.
	p := LocalProfileFromExDir("/nonexistent/bin", "/opt/cbweb3/data/spoke-x", 8645)

	wantCB := filepath.Join(home, "provisioning", "templates", "central-bank", "docker-compose.yaml")
	if p.CentralBankComposePath != wantCB {
		t.Errorf("CentralBankComposePath = %q; want %q", p.CentralBankComposePath, wantCB)
	}
	wantScripts := filepath.Join(home, "deploy", "local", "paladin", "scripts")
	if p.ScriptsDir != wantScripts {
		t.Errorf("ScriptsDir = %q; want %q", p.ScriptsDir, wantScripts)
	}
}

func TestLocalProfileFromExDir_PerPathEnvOverridesRoot(t *testing.T) {
	home := makeScenarioTree(t)
	t.Setenv("CBWEB3_HOME", home)
	t.Setenv("CBWEB3_CENTRAL_BANK_COMPOSE", "/custom/cb-compose.yaml")
	p := LocalProfileFromExDir("/nonexistent/bin", "/opt/cbweb3/data/spoke-x", 8645)
	if p.CentralBankComposePath != "/custom/cb-compose.yaml" {
		t.Errorf("per-path override ignored: CentralBankComposePath = %q", p.CentralBankComposePath)
	}
}
