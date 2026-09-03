// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"os"
	"regexp"
	"testing"
)

// The origin pattern is written twice on purpose: this package validates spec.noc.portalOrigins
// before anything is provisioned, and the orchestrator re-checks at the point the value is
// embedded in a JSON array inside a single-quoted `bash -c` argument for kcadm. The duplication
// is deliberate — manifest must not import orchestrator — but nothing kept the two identical.
//
// Drift here is not cosmetic and fails at the worst moment: a manifest that passes validation
// and then dies mid-deploy, after other entities are already provisioned. The looser side
// decides which, and either direction is a bad afternoon.
//
// Same idea as the authz parity gate: compare across the boundary rather than trust a comment.

const orchestratorSource = "../orchestrator/step_found_spoke.go"

// browserOriginPatternLiteral pulls the orchestrator's regexp out of its source, since an
// unexported var in another package cannot be read directly.
var browserOriginPatternLiteral = regexp.MustCompile(
	"browserOriginPattern\\s*=\\s*regexp\\.MustCompile\\(`([^`]*)`\\)")

func TestBrowserOriginPatternMatchesTheOrchestrator(t *testing.T) {
	src, err := os.ReadFile(orchestratorSource)
	if err != nil {
		t.Fatalf("read %s: %v", orchestratorSource, err)
	}
	m := browserOriginPatternLiteral.FindSubmatch(src)
	if m == nil {
		t.Fatalf("browserOriginPattern not found in %s — if it moved, point this test at its new "+
			"home rather than deleting the check", orchestratorSource)
	}
	if got, want := string(m[1]), browserOrigin.String(); got != want {
		t.Errorf("the two origin patterns have drifted; a manifest could pass validation and then "+
			"fail mid-deploy.\n  manifest:     %s\n  orchestrator: %s", want, got)
	}
}

// Whatever the patterns say, they must agree on the values that matter — the shapes the
// manifest field is documented to accept, and the ones it exists to keep out.
func TestBrowserOriginPatternAcceptsAndRejectsTheDocumentedShapes(t *testing.T) {
	for _, ok := range []string{
		"http://localhost:3030",
		"https://noc.example.org",
		"http://10.0.0.9:3030",
		"http://noc-host",
	} {
		if !browserOrigin.MatchString(ok) {
			t.Errorf("rejected a documented-valid origin: %q", ok)
		}
	}
	for _, bad := range []string{
		"*",
		"http://localhost:3030/",
		"http://localhost:3030/noc",
		"localhost:3030",
		"http://*.example.org:3030",
		"ftp://localhost:3030",
		"http://localhost:3030 http://evil.example",
	} {
		if browserOrigin.MatchString(bad) {
			t.Errorf("accepted a value that cannot be embedded safely: %q", bad)
		}
	}
}
