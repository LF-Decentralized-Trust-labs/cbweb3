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
	"strings"
	"testing"
)

// serviceDockerfiles are this scenario's backend service images, relative to the toolkit
// module root. Vendored trees are excluded: their Dockerfiles are third-party artefacts
// the licence-header policy and this policy both leave alone.
//
// An unreachable directory or an empty result is a failure, not a skip: the whole point
// of the guard is to notice drift, and a guard that quietly reports "nothing to check"
// after a layout change is the drift it is meant to catch.
func serviceDockerfiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "backend", "services")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("backend/services not reachable from this module (the guard cannot run): %v", err)
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
		t.Fatalf("no service Dockerfiles found under %s — the guard would pass vacuously", root)
	}
	return out
}

// stage is one FROM…(next FROM) block of a Dockerfile.
type stage struct {
	image string // the base image reference, flags and `AS <name>` stripped
	user  string // the last USER in this stage, "" if none
}

// parseStages splits a Dockerfile into its build stages. Stage-aware rather than
// whole-file: a USER set in a builder stage says nothing about the uid the shipped image
// runs as, and reading only the last USER in the file mistakes one for the other.
func parseStages(body string) []stage {
	var stages []stage
	for _, line := range strings.Split(body, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 2 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "FROM":
			rest := f[1:]
			for len(rest) > 0 && strings.HasPrefix(rest[0], "--") { // --platform=…
				rest = rest[1:]
			}
			if len(rest) == 0 {
				continue
			}
			stages = append(stages, stage{image: rest[0]})
		case "USER":
			if len(stages) > 0 {
				stages[len(stages)-1].user = f[1]
			}
		}
	}
	return stages
}

// isPinned reports whether an image reference names one specific artefact. A digest
// always does. Otherwise the tag has to start with a digit — a version. That rejects
// `latest`, a missing tag (implicitly latest), and the named floating tags that read as
// pins but are not: `nonroot` and `stable` are republished on every upstream build, so
// matching only `:latest` would certify them as pinned.
func isPinned(image string) bool {
	if strings.Contains(image, "@sha256:") {
		return true
	}
	// Split off the registry host so a host:port is not mistaken for a tag.
	name := image
	if i := strings.LastIndex(image, "/"); i >= 0 {
		name = image[i+1:]
	}
	_, tag, ok := strings.Cut(name, ":")
	if !ok || tag == "" {
		return false
	}
	return tag[0] >= '0' && tag[0] <= '9'
}

// isRoot reports whether a USER value resolves to uid 0. USER takes `uid[:gid]` or
// `name[:group]`, so only the part before the colon decides it.
func isRoot(user string) bool {
	id, _, _ := strings.Cut(user, ":")
	return id == "root" || id == "0"
}

// A moving tag makes the image a different thing on two machines and lets a rebuild pull
// in content nobody reviewed. Every stage is checked, not just the last: an unpinned
// builder silently changes the compiler the shipped binary was built with.
func TestServiceDockerfiles_BaseImagesArePinned(t *testing.T) {
	for svc, body := range serviceDockerfiles(t) {
		stages := parseStages(body)
		if len(stages) == 0 {
			t.Errorf("%s: no FROM found", svc)
			continue
		}
		for _, s := range stages {
			if !isPinned(s.image) {
				t.Errorf("%s: base image is not pinned: %q (use a version tag or @sha256 digest)", svc, s.image)
			}
		}
	}
}

// A container that runs as root turns any container escape or mounted-path mistake into a
// root-owned one. The toolkit may override the uid for services that write to a
// host-owned mount, but the image's own default must not be root. Only the final stage
// matters — that is the one that ships.
func TestServiceDockerfiles_DoNotRunAsRoot(t *testing.T) {
	for svc, body := range serviceDockerfiles(t) {
		stages := parseStages(body)
		if len(stages) == 0 {
			t.Errorf("%s: no FROM found", svc)
			continue
		}
		final := stages[len(stages)-1]
		switch {
		case final.user == "":
			t.Errorf("%s: the final stage (FROM %s) sets no USER — the image runs as root", svc, final.image)
		case isRoot(final.user):
			t.Errorf("%s: final-stage USER %q is root", svc, final.user)
		}
	}
}

// The guard's own logic, checked against the cases that slipped through the first
// version: a floating tag that is not `latest`, a missing tag, and a USER that belongs
// to the builder stage rather than the one that ships.
func TestDockerfileGuard_Rules(t *testing.T) {
	pins := []struct {
		image  string
		pinned bool
	}{
		{"alpine:3.23", true},
		{"golang:1.26-alpine", true},
		{"gcr.io/distroless/static-debian12:nonroot@sha256:abc", true},
		{"localhost:5000/svc:1.2", true},
		{"alpine:latest", false},
		{"alpine", false},                                    // implicitly latest
		{"gcr.io/distroless/static-debian12:nonroot", false}, // republished upstream
		{"gcr.io/distroless/static-debian12", false},
		{"debian:stable", false},
		{"localhost:5000/svc", false}, // host:port is not a tag
	}
	for _, c := range pins {
		if got := isPinned(c.image); got != c.pinned {
			t.Errorf("isPinned(%q) = %v, want %v", c.image, got, c.pinned)
		}
	}

	for _, c := range []struct {
		user string
		root bool
	}{
		{"0", true}, {"0:0", true}, {"root", true}, {"root:root", true},
		{"10001", false}, {"10001:10001", false}, {"app:app", false},
	} {
		if got := isRoot(c.user); got != c.root {
			t.Errorf("isRoot(%q) = %v, want %v", c.user, got, c.root)
		}
	}

	// A USER in the builder stage says nothing about the shipped image.
	builderOnly := "FROM golang:1.26-alpine AS builder\nUSER 10001:10001\nRUN go build\n\nFROM alpine:3.23\nENTRYPOINT [\"/app\"]\n"
	stages := parseStages(builderOnly)
	if len(stages) != 2 {
		t.Fatalf("parseStages: got %d stages, want 2", len(stages))
	}
	if stages[0].user != "10001:10001" {
		t.Errorf("builder stage user = %q, want 10001:10001", stages[0].user)
	}
	if stages[1].user != "" {
		t.Errorf("final stage user = %q, want empty (the USER belongs to the builder)", stages[1].user)
	}

	// --platform flags must not be mistaken for the image reference.
	if got := parseStages("FROM --platform=$BUILDPLATFORM alpine:3.23\n")[0].image; got != "alpine:3.23" {
		t.Errorf("parseStages with --platform: image = %q, want alpine:3.23", got)
	}
}
