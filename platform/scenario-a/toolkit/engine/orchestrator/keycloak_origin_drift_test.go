// SPDX-License-Identifier: Apache-2.0

// Drift guard between the two places an entity's browser origins are consumed
// (finding R1-10.7).
//
// The claim the hardening rests on is that a realm's redirectUris/webOrigins and the
// api-gateway's CORS_ALLOW_ORIGINS are the SAME list — so Keycloak cannot start trusting
// an origin the gateway rejects, or the reverse. Today that holds because buildSteps
// calls one function for both. Nothing enforced it: the two call sites are independent
// expressions a dozen lines apart, and a future change to one is invisible to the other.
//
// These tests assert the property rather than the current spelling. They walk the steps
// the engine actually builds, read the origins each side ended up with, and recompute the
// gateway's list from the env step's OWN inputs (its besuRPCPort/frontendHost/proxy). A
// change that gives the two sides different hosts, ports or proxy handling fails here.
package orchestrator

import (
	"io"
	"strings"
	"testing"
)

// keycloakStepIn locates the Keycloak provisioning step in a built plan.
func keycloakStepIn(t *testing.T, steps []Step) *provisionKeycloakStep {
	t.Helper()
	for _, s := range steps {
		if kc, ok := s.(*provisionKeycloakStep); ok {
			return kc
		}
	}
	t.Fatal("no provisionKeycloakStep in the built plan")
	return nil
}

func assertRealmOrigins(t *testing.T, label string, kc *provisionKeycloakStep, want []string) {
	t.Helper()
	if len(kc.realms) == 0 {
		t.Fatalf("%s: the Keycloak step provisions no realms", label)
	}
	for _, r := range kc.realms {
		if strings.Join(r.Origins, ",") != strings.Join(want, ",") {
			t.Errorf("%s: realm %q origins = %v, but the api-gateway is given %v — "+
				"Keycloak and the gateway have drifted", label, r.Realm, r.Origins, want)
		}
	}
}

// mode:found — the CB's realms must carry exactly the origins its api-gateway allows.
func TestCBRealmOriginsMatchTheGatewayCORS(t *testing.T) {
	for _, tc := range []struct {
		name         string
		frontendHost string
		proxy        string
	}{
		{name: "localhost", frontendHost: ""},
		{name: "routable host", frontendHost: "10.0.0.7"},
		{name: "behind the proxy", frontendHost: "cb.example.org", proxy: "enable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testManifest(t.TempDir())
			m.Spec.FrontendHost = tc.frontendHost
			m.Spec.Proxy = tc.proxy

			steps := buildSteps(m, testDeps(), m.Spec.Node.DataDir, ProvisioningState{})

			var env *renderCBEnvStep
			for _, s := range steps {
				if e, ok := s.(*renderCBEnvStep); ok {
					env = e
				}
			}
			if env == nil {
				t.Fatal("no renderCBEnvStep in the built plan")
			}
			// Recomputed from the ENV step's own fields — the gateway's side of the
			// contract — not from the manifest, so a divergence in either construction
			// site shows up here.
			want := splitOrigins(cbCORSOriginsFor(entityPorts(env.besuRPCPort), env.frontendHost, env.proxy))
			assertRealmOrigins(t, "mode:found", keycloakStepIn(t, steps), want)
		})
	}
}

// mode:join — same contract for a commercial bank's single portal origin.
func TestBankRealmOriginsMatchTheGatewayCORS(t *testing.T) {
	for _, tc := range []struct {
		name         string
		frontendHost string
		proxy        string
	}{
		{name: "localhost", frontendHost: ""},
		{name: "routable host", frontendHost: "10.0.0.7"},
		{name: "behind the proxy", frontendHost: "bank.example.org", proxy: "enable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			m := testJoinManifest(dataDir)
			m.Spec.FrontendHost = tc.frontendHost
			m.Spec.Proxy = tc.proxy

			steps := buildJoinSteps(m, testJoinBundle(), testJoinDeps(), dataDir, io.Discard)

			var env *renderBankEnvStep
			for _, s := range steps {
				if e, ok := s.(*renderBankEnvStep); ok {
					env = e
				}
			}
			if env == nil {
				t.Fatal("no renderBankEnvStep in the built plan")
			}
			want := splitOrigins(bankCORSOriginsFor(entityPorts(env.besuRPCPort), env.frontendHost, env.proxy))
			assertRealmOrigins(t, "mode:join", keycloakStepIn(t, steps), want)
		})
	}
}
