// SPDX-License-Identifier: Apache-2.0

// Hardening tests for the rendered Keycloak realm-import document (finding R1-10.7 /
// R2-§4.11).
//
// The generator shipped three permissive defaults: `redirectUris: ["*"]`,
// `webOrigins: ["*"]` and `sslRequired: "none"`, on every realm of every entity. The
// wildcards are the load-bearing ones — a wildcard redirect URI turns the realm into an
// open redirector for the authorization code, and a wildcard web origin lets any page
// read token responses from the browser. They were the same value in production as in a
// laptop stack, because nothing in the plan carried an environment or an origin list.
//
// These tests pin: no wildcard survives rendering, the origins that ARE rendered are the
// ones the entity actually serves its portals from, and sslRequired is decided by the
// deployment environment rather than hardcoded open.
package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
)

// renderedRealm renders a plan and returns the parsed document.
func renderedRealm(t *testing.T, plan KeycloakRealmPlan) map[string]any {
	t.Helper()
	data, err := renderRealmJSON(plan)
	if err != nil {
		t.Fatalf("renderRealmJSON: %v", err)
	}
	var realm map[string]any
	if err := json.Unmarshal(data, &realm); err != nil {
		t.Fatalf("invalid realm JSON: %v", err)
	}
	return realm
}

// clientStrings pulls a string-slice field off every client in a rendered realm.
func clientStrings(t *testing.T, realm map[string]any, field string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	clients, ok := realm["clients"].([]any)
	if !ok {
		t.Fatalf("rendered realm has no clients: %v", realm)
	}
	for _, c := range clients {
		cm := c.(map[string]any)
		id := cm["clientId"].(string)
		raw, present := cm[field]
		if !present {
			out[id] = nil
			continue
		}
		var vals []string
		for _, v := range raw.([]any) {
			vals = append(vals, v.(string))
		}
		out[id] = vals
	}
	return out
}

func localPlan() KeycloakRealmPlan {
	return KeycloakRealmPlan{
		Realm:       "central-bank",
		Environment: "local",
		Origins:     []string{"http://localhost:31649", "http://localhost:31650"},
		Clients:     []KeycloakClientPlan{{ClientID: "api-gateway", Secret: "s3cret", Audience: "cbweb3-backend"}},
	}
}

// No client may be rendered with a wildcard redirect URI or web origin, in any
// environment. This is the finding itself.
func TestRenderRealmJSON_NoWildcardRedirectOrOrigin(t *testing.T) {
	for _, env := range []string{"local", "staging", "prod"} {
		plan := localPlan()
		plan.Environment = env
		realm := renderedRealm(t, plan)

		for field := range map[string]bool{"redirectUris": true, "webOrigins": true} {
			for client, vals := range clientStrings(t, realm, field) {
				for _, v := range vals {
					if v == "*" || v == "+" {
						t.Errorf("env=%s client=%s %s contains the wildcard %q", env, client, field, v)
					}
				}
			}
		}
	}
}

// The rendered values must be derived from the origins the entity actually serves its
// portals from — the same list the api-gateway is given as CORS_ALLOW_ORIGINS — so the
// two cannot drift. redirectUris need a path suffix; webOrigins take the bare origin.
func TestRenderRealmJSON_UsesTheEntityPortalOrigins(t *testing.T) {
	plan := localPlan()
	realm := renderedRealm(t, plan)

	redirects := clientStrings(t, realm, "redirectUris")["api-gateway"]
	origins := clientStrings(t, realm, "webOrigins")["api-gateway"]

	for _, want := range plan.Origins {
		if !containsString(origins, want) {
			t.Errorf("webOrigins %v is missing the portal origin %q", origins, want)
		}
		if !containsString(redirects, want+"/*") {
			t.Errorf("redirectUris %v is missing %q", redirects, want+"/*")
		}
	}
	if len(origins) != len(plan.Origins) {
		t.Errorf("webOrigins %v carries entries beyond the portal origins %v", origins, plan.Origins)
	}
}

// Fail closed: a plan with no origins must be an error, never a document that falls back
// to a wildcard. A missing origin list is a provisioning bug, and rendering "*" for it is
// exactly how this finding came to exist.
func TestRenderRealmJSON_NoOriginsIsAnError(t *testing.T) {
	plan := localPlan()
	plan.Origins = nil
	if _, err := renderRealmJSON(plan); err == nil {
		t.Fatal("renderRealmJSON accepted a plan with no origins; it must fail closed")
	}
}

// sslRequired follows the environment. `none` is a local-only affordance: the portals and
// Keycloak are reached over plain HTTP on a developer machine. Anything else must demand
// TLS on non-private addresses, which is Keycloak's own default.
func TestRenderRealmJSON_SSLRequiredFollowsEnvironment(t *testing.T) {
	for env, want := range map[string]string{
		"local":   "none",
		"staging": "external",
		"prod":    "external",
	} {
		plan := localPlan()
		plan.Environment = env
		realm := renderedRealm(t, plan)
		if got := realm["sslRequired"]; got != want {
			t.Errorf("env=%s sslRequired = %v, want %q", env, got, want)
		}
	}
}

// An unset environment must not read as local. A plan that forgot to carry one gets the
// hardened value, so the permissive path is only ever reached by asking for it.
func TestRenderRealmJSON_UnsetEnvironmentIsHardened(t *testing.T) {
	plan := localPlan()
	plan.Environment = ""
	realm := renderedRealm(t, plan)
	if got := realm["sslRequired"]; got != "external" {
		t.Errorf("sslRequired with no environment = %v, want \"external\"", got)
	}
}

// The plans the orchestrator actually builds must carry origins — otherwise the
// fail-closed render above would break provisioning rather than harden it.
func TestRealmPlans_CarryOrigins(t *testing.T) {
	origins := []string{"http://localhost:31649"}
	for _, plan := range centralBankRealmPlans("central-bank-brazil", nil, "local", origins) {
		if len(plan.Origins) == 0 {
			t.Errorf("central-bank plan %q carries no origins", plan.Realm)
		}
		if plan.Environment != "local" {
			t.Errorf("central-bank plan %q environment = %q, want \"local\"", plan.Realm, plan.Environment)
		}
	}
	bank := commercialBankRealmPlan("bank-itau", nil, "local", origins)
	if len(bank.Origins) == 0 {
		t.Error("commercial-bank plan carries no origins")
	}
}

// Guard against the wildcard returning as a literal anywhere in the document, including
// in a field these tests do not enumerate.
func TestRenderRealmJSON_DocumentHasNoBareWildcard(t *testing.T) {
	data, err := renderRealmJSON(localPlan())
	if err != nil {
		t.Fatalf("renderRealmJSON: %v", err)
	}
	if strings.Contains(string(data), `"*"`) {
		t.Errorf("rendered realm still contains a bare \"*\":\n%s", data)
	}
}

// testPortalOrigins is the origin list the pre-existing tests pass now that a realm plan
// must carry one. Any non-empty list satisfies them; the hardening assertions above are
// what pin the rendered content.
var testPortalOrigins = []string{"http://localhost:31649", "http://localhost:31650"}
