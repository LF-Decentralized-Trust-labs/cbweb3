// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// keycloakScript runs the entity's Keycloak provisioning against a fake runner and returns
// the single `bash -c` script it would have executed. Every assertion below reads that
// script, because the defects this guards are things the script fails to contain.
func keycloakScript(t *testing.T, fake *exec.FakeRunner) string {
	t.Helper()
	for _, c := range fake.Calls {
		// docker exec <container> bash -c <script>
		if c.Name == "docker" && len(c.Args) >= 5 && c.Args[0] == "exec" && c.Args[2] == "bash" && c.Args[3] == "-c" {
			return c.Args[4]
		}
	}
	t.Fatalf("no `docker exec ... bash -c` call recorded; got %d calls", len(fake.Calls))
	return ""
}

// The hub used to provision no operator accounts at all: HubConfig carried no AdminUsers,
// apply never passed spec.adminUsers, and provisionKeycloakRealm never called
// appendKeycloakUsers. A clean found-hub therefore produced a realm with zero users while
// the manifest declared two, so nobody could log into the portal the hub serves and the
// declared credentials described accounts that were never going to exist.
func TestFoundHubProvisionsDeclaredAdminUsers(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	cfg.AdminUsers = []AdminUser{
		{Role: "GOVERNANCE", Username: "admin@hub.governance.gov", Password: "hub-governance-local"},
		{Role: "NOC_ADMIN", Username: "admin@hub.noc.gov", Password: "hub-noc-local"},
	}

	if err := cfg.provisionKeycloakRealm(context.Background()); err != nil {
		t.Fatalf("provisionKeycloakRealm: %v", err)
	}
	script := keycloakScript(t, fake)

	for _, u := range cfg.AdminUsers {
		if !strings.Contains(script, "create users -r "+hubKeycloakRealm+" -s username="+u.Username) {
			t.Errorf("hub provisioning does not create user %q", u.Username)
		}
		if !strings.Contains(script, "set-password -r "+hubKeycloakRealm+" --username "+u.Username) {
			t.Errorf("hub provisioning does not set a password for %q", u.Username)
		}
	}
}

// A hub manifest may legitimately declare no operator accounts. That must produce a valid
// script, not a dangling separator — the users block is appended straight onto a chain
// that already ends in " && ".
func TestFoundHubWithoutAdminUsersProducesWellFormedScript(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	cfg.AdminUsers = nil

	if err := cfg.provisionKeycloakRealm(context.Background()); err != nil {
		t.Fatalf("provisionKeycloakRealm: %v", err)
	}
	script := keycloakScript(t, fake)

	if strings.Contains(script, "&&  &&") || strings.Contains(script, "&& &&") {
		t.Errorf("script contains an empty command between separators:\n%s", script)
	}
	if strings.HasSuffix(strings.TrimSpace(script), "&&") {
		t.Errorf("script ends in a dangling separator:\n%s", script)
	}
}

// `|| true` reported success for every failure, not just the "already exists" it was there
// for. It is replaced by a helper that keeps the chain going AND puts the reason on stderr,
// so a malformed argument or an auth failure leaves a trace instead of vanishing.
func TestKeycloakProvisioningDoesNotSwallowFailures(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	if err := cfg.provisionKeycloakRealm(context.Background()); err != nil {
		t.Fatalf("provisionKeycloakRealm: %v", err)
	}
	script := keycloakScript(t, fake)

	if strings.Contains(script, "|| true") {
		t.Error("provisioning still discards kcadm failures with `|| true`")
	}
	if !strings.Contains(script, "kcw()") {
		t.Error("script does not define the kcw helper that reports a tolerated failure")
	}
}

// The end-state assertions are what hold regardless of *why* something is missing. Without
// them a create that failed, or one that succeeded without producing the object, still
// leaves the step reporting `done`.
func TestKeycloakProvisioningAssertsEndState(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testHubConfig(t, fake)
	cfg.AdminUsers = []AdminUser{{Role: "NOC_ADMIN", Username: "admin@hub.noc.gov", Password: "x"}}
	if err := cfg.provisionKeycloakRealm(context.Background()); err != nil {
		t.Fatalf("provisionKeycloakRealm: %v", err)
	}
	script := keycloakScript(t, fake)

	// Each assertion must be able to fail the step, which is what `exit 1` encodes.
	for _, want := range []string{
		"realm " + hubKeycloakRealm + " does not exist after provisioning",
		"did not accept sslRequired=NONE",
		"client " + hubKeycloakClient + " does not exist in realm " + hubKeycloakRealm,
		"user admin@hub.noc.gov was not created in realm " + hubKeycloakRealm,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("missing end-state assertion for %q", want)
		}
	}
	if n := strings.Count(script, "exit 1"); n < 4 {
		t.Errorf("expected at least 4 failing assertions, found %d", n)
	}
}
