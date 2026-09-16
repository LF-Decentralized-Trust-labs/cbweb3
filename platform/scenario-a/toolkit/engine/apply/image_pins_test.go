// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// The Besu and Paladin pins used to be written out as literals here as well as in
// the orchestrator, so raising a version meant finding every copy. That is exactly
// how the Besu pin missed the old deploy/local path: one tree was updated, the other
// was not, and nothing failed. These two tests make the second copy impossible.

// TestDefaultImagePinsComeFromTheOrchestratorConstants pins the behaviour: with no
// environment override, the profile hands the engine the same images the orchestrator
// would have defaulted to on its own.
func TestDefaultImagePinsComeFromTheOrchestratorConstants(t *testing.T) {
	t.Setenv("CBWEB3_BESU_IMAGE", "")
	t.Setenv("CBWEB3_PALADIN_IMAGE", "")

	p := LocalProfileFromExDir(t.TempDir(), t.TempDir(), 8545)

	if p.BesuImage != orchestrator.DefaultBesuImage {
		t.Errorf("profile Besu image = %q, want the orchestrator constant %q",
			p.BesuImage, orchestrator.DefaultBesuImage)
	}
	if p.PaladinImage != orchestrator.DefaultPaladinImage {
		t.Errorf("profile Paladin image = %q, want the orchestrator constant %q",
			p.PaladinImage, orchestrator.DefaultPaladinImage)
	}
}

// TestNoImagePinLiteralsInThisPackage is the drift guard the behaviour test cannot
// give: a second literal that happens to hold the same value today would pass the
// test above and still have to be found by hand at the next upgrade.
//
// Only the image *reference* is searched for, not the bare version, because a
// comment naming a version in prose is documentation rather than a pin.
func TestNoImagePinLiteralsInThisPackage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	needles := []string{"hyperledger/besu:", "lfdecentralizedtrust/paladin:"}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, needle := range needles {
			if strings.Contains(string(body), needle) {
				t.Errorf("%s pins an image inline (%q); reference the orchestrator constant instead "+
					"so a version bump has exactly one place to change", name, needle)
			}
		}
	}
}
