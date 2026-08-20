// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"strings"
	"testing"
)

// audienceMapperArg must produce a kcadm -s flag that adds an oidc-audience-mapper
// stamping the custom audience into access tokens (not id tokens).
func TestAudienceMapperArg(t *testing.T) {
	got := audienceMapperArg(keycloakBackendAudience)
	for _, want := range []string{
		"-s 'protocolMappers=",
		"oidc-audience-mapper",
		`"included.custom.audience":"cbweb3-backend"`,
		`"access.token.claim":"true"`,
		`"id.token.claim":"false"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("audienceMapperArg missing %q\n got: %s", want, got)
		}
	}
}

// The backend and NOC audiences must be distinct so a NOC-portal token cannot
// satisfy the api-gateway's aud check (and vice versa).
func TestBackendAndNOCAudiencesAreDistinct(t *testing.T) {
	if keycloakBackendAudience == keycloakNOCAudience || keycloakBackendAudience == "" || keycloakNOCAudience == "" {
		t.Fatalf("backend (%q) and NOC (%q) audiences must be distinct and non-empty",
			keycloakBackendAudience, keycloakNOCAudience)
	}
}

// The public noc-portal client must stamp the NOC audience — and NOT the backend
// audience — so a token minted for the NOC portal is not accepted at the gateway.
func TestNOCPortalClientStampsNOCAudience(t *testing.T) {
	var b strings.Builder
	if err := appendNOCPortalClient(&b, "/opt/keycloak/bin/kcadm.sh", spokeKeycloakRealm, []string{"http://localhost:45845"}); err != nil {
		t.Fatalf("appendNOCPortalClient: %v", err)
	}
	s := b.String()
	if !strings.Contains(s, "oidc-audience-mapper") || !strings.Contains(s, keycloakNOCAudience) {
		t.Errorf("noc-portal must stamp audience %q via an oidc-audience-mapper; got: %s", keycloakNOCAudience, s)
	}
	if strings.Contains(s, keycloakBackendAudience) {
		t.Errorf("noc-portal must NOT stamp the backend audience %q; got: %s", keycloakBackendAudience, s)
	}
}
