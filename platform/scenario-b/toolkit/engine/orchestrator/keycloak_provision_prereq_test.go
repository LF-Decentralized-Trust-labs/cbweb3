// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The assertions appended by appendKeycloakAssertions demand a realm state that only the
// script's own commands can produce. Nothing tied the two halves together, and they came
// apart: the join path emitted the assertions without ever emitting the
// `update realms -s sslRequired=NONE` that found-hub and found-spoke emit, so its gate
// could not pass against a real Keycloak — whose default for a new realm is
// sslRequired=external. Every bank join died on
//
//	keycloak: realm cbweb3 did not accept sslRequired=NONE
//
// which took down samples/deploy-all.sh at its first bank.
//
// The existing tests could not see it. keycloak_provision_test.go asserts the assertion
// MESSAGES appear; keycloak_assertions_predicate_test.go runs the PREDICATES against
// recorded Keycloak output. Both are about the assertions in isolation. The step tests run
// the real builder but against exec.FakeRunner, which returns success for every command —
// so the generated `&&` chain is never evaluated and a script that cannot possibly satisfy
// its own gate reports a pass.
//
// This test closes that seam: it takes the script each mode actually generates and checks
// it establishes the preconditions its assertions check. It is deliberately about the
// relationship between the two, not about either one.

// keycloakScriptFrom runs the step and returns the shell script it handed to
// `docker exec … bash -c <script>`, which is the artefact under test.
func keycloakScriptFrom(t *testing.T, fake *exec.FakeRunner) string {
	t.Helper()
	for _, call := range fake.Calls {
		if call.Name != "docker" {
			continue
		}
		for i, arg := range call.Args {
			// The script is the argument after `-c`, and only the kcadm exec has one.
			if arg == "-c" && i+1 < len(call.Args) && strings.Contains(call.Args[i+1], "kcadm.sh") {
				return call.Args[i+1]
			}
		}
	}
	t.Fatalf("no `docker exec … bash -c <kcadm script>` call was recorded; calls: %+v", fake.Calls)
	return ""
}

func TestKeycloakProvisioning_SetsWhatItsOwnAssertionsDemand(t *testing.T) {
	cases := []struct {
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

	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			fake := &exec.FakeRunner{}
			tc.run(t, fake)
			script := keycloakScriptFrom(t, fake)

			// Only meaningful for a script that actually gates on the property.
			if !strings.Contains(script, "--fields sslRequired") {
				t.Skipf("%s does not assert sslRequired; nothing to establish", tc.mode)
			}

			// Matched as a COMMAND, not as the bare literal. The assertion's own failure
			// message contains the string "sslRequired=NONE", so a naive Contains check
			// passes against a script that only ever complains about the property —
			// which is exactly the state the join path was in.
			set := indexOfRealmUpdateSettingSSL(script)
			assert := strings.Index(script, "--fields sslRequired")

			if set < 0 {
				t.Errorf("%s asserts the realm has sslRequired=none but no `update realms … -s sslRequired=NONE` "+
					"sets it.\nKeycloak defaults a new realm to sslRequired=external, so this gate can only "+
					"fail on a real server. Emit the update as the other modes do.", tc.mode)
				return
			}

			// Order matters as much as presence: setting it after the gate would fail just
			// as reliably, and a refactor could reorder them without anything noticing.
			if set > assert {
				t.Errorf("%s sets sslRequired=NONE (at %d) only after asserting it (at %d)", tc.mode, set, assert)
			}
		})
	}
}

// indexOfRealmUpdateSettingSSL returns the offset of an `update realms … -s sslRequired=NONE`
// command, or -1. Scoped to a single command by splitting on the `&&` the chain is built
// from, so the assertion's error message — which quotes the same property — cannot satisfy it.
func indexOfRealmUpdateSettingSSL(script string) int {
	offset := 0
	for _, segment := range strings.Split(script, "&&") {
		if strings.Contains(segment, "update realms") && strings.Contains(segment, "-s sslRequired=NONE") {
			return offset
		}
		offset += len(segment) + len("&&")
	}
	return -1
}
