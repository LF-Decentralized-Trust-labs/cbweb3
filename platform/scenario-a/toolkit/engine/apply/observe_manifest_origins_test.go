// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// The mechanism half of the NOC session change was built, tested and then set by no
// manifest at all — which is worse than not having it: the fallback quietly resolves the
// CORS origin to http://localhost:32645, so a portal on any other port (or on a remote VM)
// logs in and is anonymous from the next request on, with nothing in the UI to explain it.
//
// This walks the observe manifests this repository actually deploys and requires each one
// to state its origins in one of the two valid ways. It is deliberately a test of the
// FILES, not of the code: the code was already correct.

// manifestVar matches the ${IP_...} placeholders in the deploy-lnet templates. They are
// substituted at deploy time; here any routable-looking value serves, since what is under
// test is whether the FIELD is declared.
var manifestVar = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}`)

// observeManifestDirs are the directories holding Scenario A's observe manifests, relative
// to this package. deploy-lnet is shared between scenarios; only its scenario-a half is read.
var observeManifestDirs = []string{
	"../../../samples",
	"../../../../deploy-lnet/scenario-a/manifests",
}

func TestObserveManifestsDeclareTheirPortalOrigins(t *testing.T) {
	checked := 0
	for _, dir := range observeManifestDirs {
		if _, err := os.Stat(dir); err != nil {
			// The toolkit is buildable outside the repo tree; a missing directory is not a
			// failure, but it must not silently reduce this test to nothing either (see the
			// count assertion below).
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yaml.tmpl") {
				return nil
			}
			raw, rErr := os.ReadFile(path)
			if rErr != nil {
				return nil
			}
			m, lErr := loadManifestBytes(t, manifestVar.ReplaceAllString(string(raw), "10.0.0.1"))
			if lErr != nil || m.Spec.Mode != "observe" {
				return nil
			}
			checked++
			assertOriginsDeclared(t, path, m)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if checked == 0 {
		t.Fatal("no observe manifest was checked; this guard has stopped guarding anything")
	}
}

// assertOriginsDeclared holds each observe manifest to one of the two ways of naming the
// origin the NOC backend answers CORS with. Anything else falls through to the single-CB
// localhost convention, which is right for exactly one deployment and wrong for the rest.
func assertOriginsDeclared(t *testing.T, path string, m *manifest.Manifest) {
	t.Helper()
	if m.Spec.Proxy == "enable" {
		// Behind the proxy the portal is same-origin with the backend, so one value is
		// enough — but it has to be the real host: frontendHost is what that origin is
		// built from, and an empty one yields https://localhost on a remote VM.
		if strings.TrimSpace(m.Spec.FrontendHost) == "" {
			t.Errorf("%s: proxy is enabled but spec.frontendHost is empty, so the NOC backend "+
				"would answer CORS with localhost and no browser could log in", path)
		}
		return
	}
	if m.Spec.NOC == nil || len(m.Spec.NOC.PortalOrigins) == 0 {
		t.Errorf("%s: port-based deployment with no spec.noc.portalOrigins.\n"+
			"Every portal pointing at this backend has to be named — an unlisted one logs in "+
			"and is then anonymous, with no error to explain it. Declare the origins, or set "+
			"proxy: enable + frontendHost.", path)
		return
	}
	for _, o := range m.Spec.NOC.PortalOrigins {
		if strings.Contains(o, "*") {
			t.Errorf("%s: origin %q is a wildcard; a browser will not send credentials to one", path, o)
		}
	}
}

// loadManifestBytes parses manifest YAML from memory. The deploy-lnet manifests are
// templates, so they cannot be read from disk by manifest.Load without substitution.
func loadManifestBytes(t *testing.T, content string) (*manifest.Manifest, error) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp manifest: %v", err)
	}
	return manifest.Load(p)
}
