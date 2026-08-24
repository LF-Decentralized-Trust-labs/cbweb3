// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// provision-keycloak-* is skipped once KEYCLOAK_CLIENT_SECRET exists, so an origin added to
// spec.noc.portalOrigins later never reaches an entity that is already provisioned. Measured
// on a running stack: `apply --dry-run` reports provision-keycloak-spoke as "skipped". Without
// a step that converges on its own, the fix would only ever help a from-scratch redeploy.

type reconcileRunner struct {
	readOut  string
	readErr  error
	calls    [][]string
	execFail bool
}

func (r *reconcileRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--fields webOrigins") {
		return []byte(r.readOut), r.readErr
	}
	if r.execFail {
		return nil, errors.New("exec failed")
	}
	return nil, nil
}

func TestNOCOriginsAlreadyRegistered_TrueWhenAllPresent(t *testing.T) {
	r := &reconcileRunner{readOut: "http://localhost:45645,http://localhost:3030\n"}
	ok, err := nocOriginsAlreadyRegistered(context.Background(), r, "kc", keycloakAdminCLI, "cbweb3", "pw",
		[]string{"http://localhost:45645", "http://localhost:3030"})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v; want true, nil", ok, err)
	}
}

func TestNOCOriginsAlreadyRegistered_FalseWhenOneIsMissing(t *testing.T) {
	r := &reconcileRunner{readOut: "http://localhost:45645\n"}
	ok, _ := nocOriginsAlreadyRegistered(context.Background(), r, "kc", keycloakAdminCLI, "cbweb3", "pw",
		[]string{"http://localhost:45645", "http://localhost:3030"})
	if ok {
		t.Fatal("reported satisfied while the standalone NOC origin is absent")
	}
}

// A Check that errors would fail the whole apply on a provisioned entity whose containers are
// down — an ordinary state, since the steps that would start Keycloak are themselves skipped.
// "Could not ask" must mean "run the step", and the Run starts Keycloak itself.
func TestNOCOriginsAlreadyRegistered_UnreachableKeycloakRunsTheStepRatherThanFailing(t *testing.T) {
	r := &reconcileRunner{readErr: errors.New("Error: No such container")}
	ok, err := nocOriginsAlreadyRegistered(context.Background(), r, "kc", keycloakAdminCLI, "cbweb3", "pw",
		[]string{"http://localhost:3030"})
	if err != nil {
		t.Fatalf("err = %v; an unreachable Keycloak must not fail the apply", err)
	}
	if ok {
		t.Fatal("reported satisfied without being able to read anything")
	}
}

func TestNOCOriginsReconcileScript_SetsTheFullList(t *testing.T) {
	got, err := nocOriginsReconcileScript(keycloakAdminCLI, "cbweb3", "pw",
		[]string{"http://localhost:45645", "http://localhost:3030"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`webOrigins=["http://localhost:45645","http://localhost:3030"]`,
		"clientId=noc-portal",
		"update clients/$KCID",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("script is missing %q:\n%s", want, got)
		}
	}
	// An absent client must fail, not be swallowed: that is the silent outcome the existence
	// check downstream of appendNOCPortalClient was added to stop.
	if strings.Contains(got, "update clients/$KCID -r cbweb3 -s 'webOrigins=[]' || true") {
		t.Error("the update is swallowed by || true")
	}
	if !strings.Contains(got, "exit 1") {
		t.Errorf("a missing client does not fail the script:\n%s", got)
	}
}

func TestNOCOriginsReconcileScript_RejectsAMalformedOrigin(t *testing.T) {
	if _, err := nocOriginsReconcileScript(keycloakAdminCLI, "cbweb3", "pw",
		[]string{"http://localhost:3030/"}); err == nil {
		t.Fatal("no error; a value that cannot be embedded must not reach kcadm")
	}
}

func TestOriginsSatisfied_ParsesKcadmCSV(t *testing.T) {
	cases := []struct {
		out  string
		want []string
		ok   bool
	}{
		{"http://localhost:45645,http://localhost:3030\n", []string{"http://localhost:3030"}, true},
		{"http://localhost:45645\n", []string{"http://localhost:3030"}, false},
		{"\n", []string{"http://localhost:3030"}, false},
		{"", nil, true}, // nothing wanted is trivially satisfied
	}
	for _, tc := range cases {
		if got := originsSatisfied([]byte(tc.out), tc.want); got != tc.ok {
			t.Errorf("originsSatisfied(%q, %v) = %v; want %v", tc.out, tc.want, got, tc.ok)
		}
	}
}
