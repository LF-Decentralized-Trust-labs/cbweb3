// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The other tests in this package assert that each assertion's MESSAGE appears in the
// generated script. That leaves the half that can actually be wrong untested: the
// PREDICATE. Every predicate encodes an assumption about how Keycloak formats what
// `kcadm get` returns, and one of those assumptions was wrong — Keycloak normalises
// sslRequired to lower case, so matching the literal "NONE" we send fails against a
// correctly configured realm and every apply dies on a healthy stack.
//
// A message-text test cannot see that. Reverting `grep -qi '"none"'` to `grep -q '"NONE"'`
// leaves the rest of the suite green, which is how it nearly shipped.
//
// So this runs the generated predicates for real, through bash, against recorded output
// from an actual Keycloak 26.0 realm provisioned the way provisionKeycloakRealm provisions
// one (testdata/kcadm/*.json, captured verbatim). No live server needed at test time; the
// server's formatting is what the fixtures preserve.

const (
	predRealm  = "cbweb3"
	predClient = "spoke-backend"
	predUser   = "admin@brasil.treasury.gov"
)

// fakeKcadm writes a stub that answers each `kcadm get` from a fixture file, so the real
// generated chain runs unmodified. Dispatch order matters: "--fields sslRequired" must be
// matched before the realm case, since both read realms/<realm>.
func fakeKcadm(t *testing.T, fixtureDir string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kcadm.sh")
	script := `#!/usr/bin/env bash
case "$*" in
  *"--fields sslRequired"*) cat "` + fixtureDir + `/ssl-required.json" ;;
  *"--fields realm"*)       cat "` + fixtureDir + `/realm.json" ;;
  *clients*)                cat "` + fixtureDir + `/clients.json" ;;
  *users*)                  cat "` + fixtureDir + `/users.json" ;;
  *) echo "unexpected kcadm call: $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// runAssertions builds the real assertion chain and executes it against fixtureDir.
func runAssertions(t *testing.T, fixtureDir string) (string, error) {
	t.Helper()
	var b strings.Builder
	b.WriteString("true")
	appendKeycloakAssertions(&b, fakeKcadm(t, fixtureDir), predRealm, predClient,
		[]AdminUser{{Role: "TREASURY", Username: predUser, Password: "x"}})
	cmd := exec.Command("bash", "-c", b.String())
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// copyFixtures clones the recorded outputs so a case can overwrite one of them.
func copyFixtures(t *testing.T, overwrite map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"realm.json", "ssl-required.json", "clients.json", "users.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", "kcadm", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if replacement, ok := overwrite[name]; ok {
			data = []byte(replacement)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available; the assertions are shell and cannot be exercised")
	}
}

// The five predicates must all pass against output a healthy realm actually produces.
// This is the case that fails if a predicate encodes the wrong format — including the
// "NONE" vs "none" mismatch that broke a live deploy.
func TestKeycloakAssertionPredicates_PassAgainstRecordedRealmOutput(t *testing.T) {
	requireBash(t)
	out, err := runAssertions(t, copyFixtures(t, nil))
	if err != nil {
		t.Fatalf("assertions failed against a healthy realm's own output: %v\n%s", err, out)
	}
	if out != "" {
		t.Errorf("a healthy realm produced output: %q", out)
	}
}

// And each one must fail — with its own message — when the thing it guards is absent or
// wrong. Without this half, a predicate that always passes would look just as green.
func TestKeycloakAssertionPredicates_FailWhenTheGuardedObjectIsMissing(t *testing.T) {
	requireBash(t)
	cases := []struct {
		name      string
		overwrite map[string]string
		wantMsg   string
	}{
		{
			name:      "realm absent",
			overwrite: map[string]string{"realm.json": ""},
			wantMsg:   "realm cbweb3 does not exist after provisioning",
		},
		{
			// The live failure in reverse: a realm that did NOT take sslRequired=NONE.
			name:      "sslRequired still external",
			overwrite: map[string]string{"ssl-required.json": "{\n  \"sslRequired\" : \"external\"\n}"},
			wantMsg:   "did not accept sslRequired=NONE",
		},
		{
			name:      "confidential client absent",
			overwrite: map[string]string{"clients.json": "[ ]"},
			wantMsg:   "client spoke-backend does not exist in realm cbweb3",
		},
		{
			// The defect the hub card was opened for: a realm with no users.
			name:      "declared user absent",
			overwrite: map[string]string{"users.json": "[ ]"},
			wantMsg:   "user " + predUser + " was not created in realm cbweb3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runAssertions(t, copyFixtures(t, tc.overwrite))
			if err == nil {
				t.Fatalf("assertions passed with %s; the chain must exit non-zero\n%s", tc.name, out)
			}
			if !strings.Contains(out, tc.wantMsg) {
				t.Errorf("message does not name the problem.\n  want substring: %q\n  got: %s", tc.wantMsg, out)
			}
		})
	}
}

// The sslRequired predicate is singled out because its failure mode is the worst kind:
// not a silent pass, but a hard failure on every apply against a correctly configured
// realm. Keycloak stores what we sent as "none"; a case-sensitive match for "NONE" is
// wrong in a way no message-text test can see.
func TestKeycloakAssertionPredicates_SSLRequiredIsMatchedCaseInsensitively(t *testing.T) {
	requireBash(t)
	recorded, err := os.ReadFile(filepath.Join("testdata", "kcadm", "ssl-required.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(recorded), `"NONE"`) {
		t.Fatalf("fixture no longer reflects Keycloak's normalisation; re-capture it:\n%s", recorded)
	}
	if !strings.Contains(string(recorded), `"none"`) {
		t.Fatalf("fixture does not contain the stored value at all:\n%s", recorded)
	}
	// Same value, upper-cased the way it is sent: the predicate must still accept it, so
	// the check cannot be tightened back to a case-sensitive match on either spelling.
	upper := strings.Replace(string(recorded), `"none"`, `"NONE"`, 1)
	if out, err := runAssertions(t, copyFixtures(t, map[string]string{"ssl-required.json": upper})); err != nil {
		t.Fatalf("the predicate rejected the value in the case Keycloak was sent: %v\n%s", err, out)
	}
}
