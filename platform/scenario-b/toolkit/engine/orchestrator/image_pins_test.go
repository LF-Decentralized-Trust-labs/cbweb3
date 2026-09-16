// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ported from Scenario A (engine/apply/image_pins_test.go). The guard audit in
// docs/guard-parity.md framed the work as porting B's guards to A and found the drift runs
// both ways: this one existed only in A, while B held the same Besu version string in four
// steps — step_found_hub, step_found_spoke, step_join and step_genesis — with nothing
// tying them together.
//
// Scope is narrower than A's on purpose: A guards two pins (Besu and Paladin) because A
// runs Paladin. B does not — its entity-besu template declares a paladin service the
// toolkit never starts (see docs/scenario-drift.md). Adding a Paladin needle here would
// guard nothing and read as if B ran it.

// imagePinDeclarationFile is the one file allowed to hold an image reference as a literal.
const imagePinDeclarationFile = "image_pins.go"

// TestDefaultBesuImageIsThePinnedVersion pins the value itself, so a bump is a deliberate
// edit to a test rather than a silent one. docs/TOOLCHAIN.md is the authority; this is the
// code's copy of that decision, in exactly one place.
func TestDefaultBesuImageIsThePinnedVersion(t *testing.T) {
	const want = "hyperledger/besu:25.8.0"
	if DefaultBesuImage != want {
		t.Errorf("DefaultBesuImage = %q, want %q — if this is an intended upgrade, change "+
			"docs/TOOLCHAIN.md and this test together", DefaultBesuImage, want)
	}
}

// TestNoImagePinLiteralsInThisPackage is the drift guard the value test cannot give: a
// second literal holding the same value today would pass the test above and still have to
// be found by hand at the next upgrade.
//
// Only the image *reference* is searched for, not the bare version, because a comment
// naming a version in prose is documentation rather than a pin.
func TestNoImagePinLiteralsInThisPackage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	const needle = "hyperledger/besu:"
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if name == imagePinDeclarationFile {
			continue
		}
		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		scanned++
		if strings.Contains(string(body), needle) {
			t.Errorf("%s pins an image inline (%q); reference DefaultBesuImage instead so a "+
				"version bump has exactly one place to change", name, needle)
		}
	}

	// A guard that silently scanned nothing — a renamed package dir, a changed working
	// directory — would report success forever. This is the same class of vacuity the
	// licence gate once had.
	if scanned == 0 {
		t.Fatal("scanned no source files; the guard cannot pass by finding nothing to read")
	}
}
