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
	"encoding/json"
	"strings"
	"testing"
)

func nocClientCmd(t *testing.T, origins []string) string {
	t.Helper()
	var b strings.Builder
	if err := appendNOCPortalClient(&b, "/opt/keycloak/bin/kcadm.sh", spokeKeycloakRealm, origins); err != nil {
		t.Fatalf("appendNOCPortalClient(%v): %v", origins, err)
	}
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

// ── The kcadm argument must be provably valid JSON ───────────────────────────
//
// The finding this PR closes was never really "a wildcard": it was an argument kcadm
// could not parse, behind a `|| true` that hid the refusal. Stripping the two characters
// that happened to appear in the old value leaves the class open — a backslash produces
// an invalid JSON escape and the exact same silent absence of the client. So the
// sanitizer proves what it accepts instead of enumerating what it rejects.

func TestJSONStringArray_RejectsWhatItCannotProveValid(t *testing.T) {
	for _, bad := range []string{
		`http://host"`,    // closes the JSON string
		`http://ho\st:1`,  // invalid JSON escape — the character the first fix missed
		`http://host\`,    // trailing backslash — "unexpected end of JSON input"
		"http://host'",    // closes the single-quoted -s argument
		"http://host\n",   // control character: never valid inside a JSON string
		"http://host:1 x", // whitespace: splits the argument
		"http://ho$t:1",   // shell expansion
		"http://host;id",  // command separator
		"ftp://host",      // not a browser origin
		"*",               // the wildcard the finding is about
		"",                // an empty origin is not an origin
	} {
		if got, err := jsonStringArray([]string{bad}); err == nil {
			t.Errorf("jsonStringArray(%q) = %s, want an error", bad, got)
		}
	}
}

func TestJSONStringArray_RejectsAnEmptyList(t *testing.T) {
	if got, err := jsonStringArray(nil); err == nil {
		t.Errorf("jsonStringArray(nil) = %s, want an error — a client with no web origin "+
			"cannot serve the browser grant, and there is no wildcard to fall back to", got)
	}
}

func TestJSONStringArray_EncodesTheOriginsItAccepts(t *testing.T) {
	in := []string{"http://localhost:45845", "http://10.0.0.7:45845", "https://cb.example.org"}
	got, err := jsonStringArray(in)
	if err != nil {
		t.Fatalf("jsonStringArray(%v): %v", in, err)
	}
	var back []string
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Fatalf("emitted %s, which does not parse as JSON: %v", got, err)
	}
	if strings.Join(back, ",") != strings.Join(in, ",") {
		t.Errorf("round-trip = %v, want %v", back, in)
	}
}

// End to end: whatever reaches the -s argument must parse as a JSON array. This is the
// assertion that would have failed on the original `[\"*\"]` form.
func TestAppendNOCPortalClient_WebOriginsArgumentParsesAsJSON(t *testing.T) {
	cmd := nocClientCmd(t, []string{"http://localhost:45845", "http://10.0.0.7:45845"})
	const marker = "-s 'webOrigins="
	i := strings.Index(cmd, marker)
	if i < 0 {
		t.Fatalf("no webOrigins argument in:\n%s", cmd)
	}
	rest := cmd[i+len(marker):]
	j := strings.Index(rest, "'")
	if j < 0 {
		t.Fatalf("unterminated webOrigins argument in:\n%s", cmd)
	}
	var origins []string
	if err := json.Unmarshal([]byte(rest[:j]), &origins); err != nil {
		t.Fatalf("webOrigins argument %q does not parse as JSON (kcadm would answer "+
			"\"Cannot parse the JSON\"): %v", rest[:j], err)
	}
	if len(origins) != 2 {
		t.Errorf("webOrigins = %v, want the two portal origins", origins)
	}
}

// A rejected origin must abort the whole command, not emit a partial one.
func TestAppendNOCPortalClient_FailsClosedOnAnUnsafeOrigin(t *testing.T) {
	var b strings.Builder
	err := appendNOCPortalClient(&b, "/opt/keycloak/bin/kcadm.sh", spokeKeycloakRealm, []string{`http://ho\st:1`})
	if err == nil {
		t.Fatal("appendNOCPortalClient accepted an origin that renders as invalid JSON")
	}
	if b.Len() != 0 {
		t.Errorf("a rejected origin must leave no partial command, got:\n%s", b.String())
	}
}

// ── The create must not be able to fail silently ─────────────────────────────
//
// `|| true` keeps the create idempotent, but it also turned "kcadm refused this" into
// "looks like success" — which is why the client was absent on every provisioned entity
// and nobody saw it. The create is now followed by an existence check that exits
// non-zero, converting that into a named error.
func TestAppendNOCPortalClient_VerifiesTheClientExists(t *testing.T) {
	cmd := nocClientCmd(t, []string{"http://localhost:45845"})
	for _, want := range []string{
		"get clients -r " + spokeKeycloakRealm + " -q clientId=" + nocKeycloakClient,
		"exit 1",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("noc-portal creation is not verified — missing %q in:\n%s", want, cmd)
		}
	}
	// The check must come after the create, or it verifies nothing.
	if strings.Index(cmd, "create clients") > strings.Index(cmd, "get clients") {
		t.Errorf("the existence check runs before the create:\n%s", cmd)
	}
}
