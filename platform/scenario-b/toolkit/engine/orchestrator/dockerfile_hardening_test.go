// SPDX-License-Identifier: Apache-2.0

// Guard for finding R2-M-12: every backend service image must be built on a pinned base
// and must not run as root.
//
// This is a scan rather than an assertion about one file on purpose. The finding was not
// that a particular Dockerfile was wrong — it was that the whole set had drifted onto
// `alpine:latest` with no USER, and a new service copied from an existing one would
// inherit both. A test that reads the directory catches the next copy.
package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var reFromLatest = regexp.MustCompile(`(?m)^FROM\s+\S+:latest`)

// serviceDockerfiles are this scenario's backend service images, relative to the toolkit
// module root. Vendored trees are excluded: their Dockerfiles are third-party artefacts
// the licence-header policy and this policy both leave alone.
func serviceDockerfiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "backend", "services")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("backend/services not reachable from this module: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name(), "Dockerfile")
		b, err := os.ReadFile(p)
		if err != nil {
			continue // a service without its own Dockerfile is not this test's business
		}
		out[e.Name()] = string(b)
	}
	if len(out) == 0 {
		t.Skip("no service Dockerfiles found")
	}
	return out
}

// A moving tag makes the image a different thing on two machines and lets a rebuild pull
// in content nobody reviewed.
func TestServiceDockerfiles_BaseImagesArePinned(t *testing.T) {
	for svc, body := range serviceDockerfiles(t) {
		if m := reFromLatest.FindString(body); m != "" {
			t.Errorf("%s: base image is not pinned: %q", svc, strings.TrimSpace(m))
		}
	}
}

// A container that runs as root turns any container escape or mounted-path mistake into a
// root-owned one. The toolkit may override the uid for services that write to a
// host-owned mount, but the image's own default must not be root.
func TestServiceDockerfiles_DoNotRunAsRoot(t *testing.T) {
	for svc, body := range serviceDockerfiles(t) {
		var user string
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "USER ") {
				user = strings.TrimSpace(strings.TrimPrefix(line, "USER "))
			}
		}
		switch {
		case user == "":
			t.Errorf("%s: no USER directive — the image runs as root", svc)
		case user == "root" || strings.HasPrefix(user, "0:") || user == "0":
			t.Errorf("%s: USER %q is root", svc, user)
		}
	}
}
