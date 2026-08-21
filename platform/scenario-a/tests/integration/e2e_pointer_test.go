// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite
// +build integration_lite

package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Scenario A's end-to-end suite lives in THIS directory under the `integration` tag,
// while `tests/e2e/` holds only a README pointing here. That arrangement is fine as
// long as the pointer is true, and worthless the moment it is not: an empty `tests/e2e`
// with a stale README is exactly the "Scenario A has no E2E coverage" conclusion that
// card R1-12.10 recorded in the first place.
//
// So the pointer is asserted rather than trusted. This test carries the
// `integration_lite` tag deliberately: that is the hermetic lane CI runs on every pull
// request, so a move or rename fails in CI rather than on someone's next reading.
func TestE2EPointerIsCurrent(t *testing.T) {
	readme := filepath.Join("..", "e2e", "README.md")
	body, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("tests/e2e/README.md is missing: %v\n"+
			"An empty tests/e2e directory implies Scenario A has no end-to-end tests, which is false.", err)
	}
	text := string(body)

	// Everything the README claims exists, checked as a path relative to this package.
	for _, ref := range []struct {
		path string
		why  string
	}{
		{filepath.Join("..", "TEST-CATALOG.md"), "the catalogue the README sends readers to"},
		{filepath.Join("..", "..", "tryouts"), "the scripted E2E walkthroughs (E2E-A-01..04)"},
		{filepath.Join("..", "..", "tryouts", "tryout-fx-agreement-e2e.sh"), "the full atomic-swap walkthrough"},
		{filepath.Join("..", "..", "tryouts", "QUICKSTART-FX-E2E.md"), "the quickstart the README names"},
		{filepath.Join("..", "..", "make", "20-tests.mk"), "the makefile holding the e2e target"},
	} {
		if _, err := os.Stat(ref.path); err != nil {
			t.Errorf("%s no longer exists (%s): %v", ref.path, ref.why, err)
		}
	}

	// The live entry point the README names must still be a test in this package.
	if !anyFileContains(t, ".", "func TestFullHappyPath(") {
		t.Error("TestFullHappyPath is gone from this package; tests/e2e/README.md names it as the live E2E entry point")
	}

	// And the make target it tells people to run must still be defined.
	mk, err := os.ReadFile(filepath.Join("..", "..", "make", "20-tests.mk"))
	if err != nil {
		t.Fatalf("cannot read the tests makefile: %v", err)
	}
	for _, target := range []string{"scenario-a.test-integration:", "scenario-a.test-backend-coverage:"} {
		if !strings.Contains(string(mk), target) {
			t.Errorf("the %q target named in tests/e2e/README.md is no longer defined in make/20-tests.mk", target)
		}
	}

	// Guard against the README being emptied into a stub that says nothing useful.
	for _, must := range []string{
		"TestFullHappyPath", "integration_lite", "TEST-CATALOG.md", "tryouts",
		"scenario-a.test-integration",
	} {
		if !strings.Contains(text, must) {
			t.Errorf("tests/e2e/README.md no longer mentions %q; the pointer has lost its content", must)
		}
	}
}

// anyFileContains reports whether any .go file in dir contains needle.
func anyFileContains(t *testing.T, dir, needle string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("cannot read %s: %v", e.Name(), err)
		}
		if strings.Contains(string(b), needle) {
			return true
		}
	}
	return false
}
