// SPDX-License-Identifier: Apache-2.0

// The Keycloak admin secret must never be written into a generated provisioning
// script.
//
// Every provisioning step builds a shell script and hands the whole thing to
// `docker exec … bash -c`. A secret interpolated into that text lands in THREE
// process lists, not one: the container's (shell argv plus the kcadm JVM's own
// argv), the host's (the script is an argument to the docker client), and the
// toolkit's own argv for the same reason.
//
// The escape needs nothing from the host: the value is already inside the
// container, in Keycloak's own environment as KC_BOOTSTRAP_ADMIN_PASSWORD, so the
// script references the NAME and the value never crosses the boundary. kcadm reads
// its password from KC_CLI_PASSWORD when the flag is absent — the exact analogue of
// REDISCLI_AUTH, which this codebase already uses for Redis.
//
// Verified against quay.io/keycloak/keycloak:26.0, the pinned image, on 2026-09-02:
// docker exec inherits the container's environment; `config credentials` with no
// --password authenticates from KC_CLI_PASSWORD and an authenticated read then
// returns real data; and with the variable unset and no flag it fails with
// "Console is not active, but password is required" and exit 1 — so the variable is
// doing the work, and a failure still breaks the script's && chain.
package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// sentinelAdminPassword is resolved through resolveInfraSecret's own first branch —
// it reads os.Getenv(name) before any file — so the value the script would embed is
// known exactly. Reading the generated secrets file instead would make this test
// vacuous once the fix lands: with nothing interpolating the secret, the read would
// mint a fresh value that trivially does not appear in the script.
const sentinelAdminPassword = "s3ntinel-admin-secret-must-not-appear"

// TestKeycloakProvisioning_NeverEmbedsTheAdminSecret is the load-bearing assertion:
// the resolved secret must not appear anywhere in the generated script.
func TestKeycloakProvisioning_NeverEmbedsTheAdminSecret(t *testing.T) {
	for _, tc := range keycloakProvisioningModes() {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("KC_ADMIN_PASSWORD", sentinelAdminPassword)

			fake := &exec.FakeRunner{}
			tc.run(t, fake)
			script := keycloakScriptFrom(t, fake)

			if strings.Contains(script, sentinelAdminPassword) {
				t.Errorf("%s embeds the Keycloak admin secret in the generated script.\n"+
					"That text is an argument to `docker exec`, so the value lands in the container's "+
					"argv, the host's argv and the toolkit's own. Reference the container's "+
					"KC_BOOTSTRAP_ADMIN_PASSWORD instead — the value is already there.", tc.mode)
			}
		})
	}
}

// TestKeycloakProvisioning_AuthenticatesFromTheContainerEnvironment pins the shape,
// which the value check alone cannot: a script that stopped passing the flag but
// also stopped authenticating would satisfy the test above and fail on a real
// server.
func TestKeycloakProvisioning_AuthenticatesFromTheContainerEnvironment(t *testing.T) {
	for _, tc := range keycloakProvisioningModes() {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("KC_ADMIN_PASSWORD", sentinelAdminPassword)

			fake := &exec.FakeRunner{}
			tc.run(t, fake)
			script := keycloakScriptFrom(t, fake)

			if i := indexOfCredentialsWithPasswordFlag(script); i >= 0 {
				t.Errorf("%s still passes --password to `config credentials` (at %d); "+
					"drop the flag and let kcadm read KC_CLI_PASSWORD", tc.mode, i)
			}

			// Matched as an assignment of the container's own variable, not as the bare
			// name: a comment or an error message mentioning KC_CLI_PASSWORD must not
			// satisfy this. Same reasoning as indexOfRealmUpdateSettingSSL.
			if !strings.Contains(script, `KC_CLI_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD"`) {
				t.Errorf("%s does not export KC_CLI_PASSWORD from the container's "+
					"KC_BOOTSTRAP_ADMIN_PASSWORD, so `config credentials` has no password to use "+
					"and would fail with \"Console is not active, but password is required\"", tc.mode)
			}
		})
	}
}

// indexOfCredentialsWithPasswordFlag returns the offset of a `config credentials`
// command that also carries --password, or -1.
//
// Scoped to one command by splitting on the `&&` the chain is built from, so an
// unrelated later command carrying --password (there is none today, but a realm
// import or a user create could) does not implicate the login, and vice versa.
func indexOfCredentialsWithPasswordFlag(script string) int {
	offset := 0
	for _, segment := range strings.Split(script, "&&") {
		if strings.Contains(segment, "config credentials") && strings.Contains(segment, "--password") {
			return offset
		}
		offset += len(segment) + len("&&")
	}
	return -1
}

// keycloakProvisioningModes is the shared table: every mode that generates a
// Keycloak provisioning script. Kept beside the exposure tests rather than inlined
// so a fourth mode is covered by both of them at once.
func keycloakProvisioningModes() []struct {
	mode string
	run  func(*testing.T, *exec.FakeRunner)
} {
	return []struct {
		mode string
		run  func(*testing.T, *exec.FakeRunner)
	}{
		{
			mode: "found-hub",
			run: func(t *testing.T, fake *exec.FakeRunner) {
				cfg := testHubConfig(t, fake)
				if err := cfg.provisionKeycloakRealm(context.Background()); err != nil {
					t.Fatalf("provision: %v", err)
				}
			},
		},
		{
			mode: "found-spoke",
			run: func(t *testing.T, fake *exec.FakeRunner) {
				cfg := testSpokeCfg(t, fake)
				if err := cfg.provisionKeycloakRealm(context.Background()); err != nil {
					t.Fatalf("provision: %v", err)
				}
			},
		},
		{
			mode: "join",
			run: func(t *testing.T, fake *exec.FakeRunner) {
				cfg := testJoinCfg(t, fake)
				if err := cfg.provisionKeycloakRealm(context.Background()); err != nil {
					t.Fatalf("provision: %v", err)
				}
			},
		},
	}
}
