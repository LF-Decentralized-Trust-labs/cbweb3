// SPDX-License-Identifier: Apache-2.0

// Hardening test for the PUBLIC noc-portal Keycloak client (finding R1-10.7 / R2-§4.11).
//
// The client is created with publicClient=true and directAccessGrantsEnabled=true: the NOC
// portal performs a password grant straight from the browser, with no client secret. It
// was also created with webOrigins=["*"], which means any origin could read that token
// response. For a public client that is the worst of the two wildcard classes — nothing
// stands between an attacker's page and a usable token.
//
// These tests pin the wildcard out and the portal's own origin in.
package orchestrator

import (
	"strings"
	"testing"
)

func nocClientCmd(t *testing.T, origins []string) string {
	t.Helper()
	var b strings.Builder
	appendNOCPortalClient(&b, "/opt/keycloak/bin/kcadm.sh", spokeKeycloakRealm, origins)
	return b.String()
}

func TestAppendNOCPortalClient_NoWildcardWebOrigins(t *testing.T) {
	cmd := nocClientCmd(t, []string{"http://localhost:45845"})
	if strings.Contains(cmd, `"*"`) {
		t.Errorf("noc-portal client still grants a wildcard web origin:\n%s", cmd)
	}
}

func TestAppendNOCPortalClient_UsesThePortalOrigins(t *testing.T) {
	origins := []string{"http://localhost:45845", "http://10.0.0.7:45845"}
	cmd := nocClientCmd(t, origins)
	for _, o := range origins {
		if !strings.Contains(cmd, o) {
			t.Errorf("noc-portal webOrigins is missing %q:\n%s", o, cmd)
		}
	}
}

// nocPortalOrigins must resolve to the portal's own port (RPC+12000), add the routable
// host when there is one, and collapse to the single proxy origin behind the proxy.
func TestNOCPortalOrigins(t *testing.T) {
	local := nocPortalOrigins(33845, "localhost", false)
	if len(local) != 1 || local[0] != "http://localhost:45845" {
		t.Errorf("local origins = %v, want [http://localhost:45845]", local)
	}

	routable := nocPortalOrigins(33845, "10.0.0.7", false)
	if len(routable) != 2 || !strings.Contains(strings.Join(routable, ","), "http://10.0.0.7:45845") {
		t.Errorf("routable origins = %v, want localhost + 10.0.0.7 on :45845", routable)
	}

	proxied := nocPortalOrigins(33845, "cb.example.org", true)
	if len(proxied) != 1 || strings.Contains(proxied[0], ":45845") {
		t.Errorf("proxied origins = %v, want a single proxy origin with no portal port", proxied)
	}
}
