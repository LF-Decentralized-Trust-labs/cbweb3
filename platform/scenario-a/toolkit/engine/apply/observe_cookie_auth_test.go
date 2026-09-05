// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// Consequences of moving the NOC login off the browser and onto the backend.

// TestContainerReachableURL: spec.noc.keycloakURL was written for a browser, where
// "localhost" is the host. The grant now runs in a container, where it is not.
func TestContainerReachableURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://localhost:40645", "http://host.docker.internal:40645"},
		{"http://127.0.0.1:40645", "http://host.docker.internal:40645"},
		// Already routable: left exactly as the operator wrote it.
		{"https://kc.example/auth", "https://kc.example/auth"},
		{"http://keycloak.internal:8080", "http://keycloak.internal:8080"},
	}
	for _, tc := range cases {
		if got := containerReachableURL(tc.in); got != tc.want {
			t.Errorf("containerReachableURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestDeriveNOCCSRFSecret: stable for one stack, distinct between stacks. A secret that
// changed on restart would refuse every CSRF token issued before it, which presents as a
// browser fault rather than a configuration one.
func TestDeriveNOCCSRFSecret(t *testing.T) {
	if a, b := deriveNOCCSRFSecret("cbweb3-noc-brazil"), deriveNOCCSRFSecret("cbweb3-noc-brazil"); a != b {
		t.Error("the secret is not stable for the same stack")
	}
	if a, b := deriveNOCCSRFSecret("cbweb3-noc-brazil"), deriveNOCCSRFSecret("cbweb3-noc-colombia"); a == b {
		t.Error("two different stacks share a CSRF secret")
	}
	if deriveNOCCSRFSecret("x") == "" {
		t.Error("empty secret")
	}
}

func manifestWithOrigins(origins ...string) *manifest.Manifest {
	m := &manifest.Manifest{}
	m.Spec.NOC = &manifest.NOC{PortalOrigins: origins}
	return m
}

// TestNOCPortalOrigins_ListsEveryPortalThatMayLogIn is the difference from Scenario B, and
// the reason this is a list at all: ONE NOC backend is shared by every CB portal on the
// host (the samples' own comment says :32645 and :32745 both point at :28645). A browser
// refuses to send credentials to a wildcard, so each origin has to be named.
func TestNOCPortalOrigins_ListsEveryPortalThatMayLogIn(t *testing.T) {
	got := nocPortalOrigins(manifestWithOrigins("http://localhost:32645", "http://localhost:32745"), "localhost", false)

	for _, want := range []string{"http://localhost:32645", "http://localhost:32745"} {
		if !strings.Contains(got, want) {
			t.Errorf("origins %q is missing %q; that portal logs in and is then anonymous", got, want)
		}
	}
	if strings.Contains(got, "*") {
		t.Error("a wildcard origin cannot carry credentials")
	}
}

// TestNOCPortalOrigins_FallsBackToTheSingleCBConvention keeps existing manifests working: a
// manifest written before this field existed must still start, rather than failing on a
// value it could not have known to set.
func TestNOCPortalOrigins_FallsBackToTheSingleCBConvention(t *testing.T) {
	// No spec.noc block at all — the shape every manifest written before this field has.
	got := nocPortalOrigins(&manifest.Manifest{}, "localhost", false)
	if got != "http://localhost:32645" {
		t.Errorf("fallback = %q, want the founding CB's NOC portal", got)
	}
}

// TestNOCPortalOrigins_ProxyCollapsesToOneOrigin: behind the proxy the portal is same-origin
// with the backend, so the manifest's list is irrelevant and must not widen it.
func TestNOCPortalOrigins_ProxyCollapsesToOneOrigin(t *testing.T) {
	got := nocPortalOrigins(manifestWithOrigins("http://should-not-appear:1"), "cb.example", true)
	if strings.Contains(got, "should-not-appear") {
		t.Errorf("proxy mode leaked a manifest origin: %q", got)
	}
}
