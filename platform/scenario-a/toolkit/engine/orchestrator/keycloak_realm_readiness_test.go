// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"strings"
	"testing"
)

// The defect this pins: provision-keycloak reported success over a realm it had never imported.
//
// Two independent paths produced it, and either one alone is enough.
//
//  1. The step's Check probed `/realms/master`. That realm exists in every Keycloak, so the
//     Check answered "satisfied" whenever the container was up — on any entity already
//     provisioned the step was skipped outright, without even rewriting the realm JSON into
//     the import volume.
//  2. The container runs `start-dev --import-realm` (keycloak-compose.yaml:38), and
//     --import-realm does not apply a file for a realm that already exists. So even when the
//     step did run, an existing realm was left as it was — and the readiness probe on master
//     answered 200 regardless.
//
// The consequence was silent: A's apply has exactly one reconcile step (reconcile-admin-users,
// for roles/users/passwords/grants), so a client, an origin, a redirect URI or a token lifespan
// declared in the manifest never reached an entity that already had the realm, and the apply
// reported success.
//
// This file covers path 1: the step must gate on the realms it is there to create. Path 2 —
// converging a realm that exists but has drifted — is keycloak_realm_reconcile_test.go.

func realmPlans(names ...string) []KeycloakRealmPlan {
	plans := make([]KeycloakRealmPlan, 0, len(names))
	for _, n := range names {
		plans = append(plans, KeycloakRealmPlan{Realm: n, Environment: "local", Origins: []string{"http://localhost:5173"}})
	}
	return plans
}

// probeStep builds the step with a recorded health probe, so the test sees exactly which URLs
// the Check asks about and can answer per URL.
func probeStep(t *testing.T, plans []KeycloakRealmPlan, answer func(url string) bool) (*provisionKeycloakStep, *[]string) {
	t.Helper()
	var asked []string
	s := &provisionKeycloakStep{
		name: StepProvisionKeycloak, entityPrefix: "cb", hostPort: 8081, realms: plans,
		healthy: func(_ context.Context, url string) bool {
			asked = append(asked, url)
			return answer(url)
		},
	}
	return s, &asked
}

// The load-bearing case: Keycloak is up and serving master, and the entity's realm does not
// exist. Before this fix the Check answered satisfied and the step never ran.
func TestProvisionKeycloak_UnimportedRealmIsNotSatisfied(t *testing.T) {
	s, asked := probeStep(t, realmPlans("cb-realm"), func(url string) bool {
		return strings.HasSuffix(url, "/realms/master")
	})

	ok, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if ok {
		t.Error("a running Keycloak that does not serve the declared realm must not report satisfied — " +
			"that is how an entity's realm silently never gets imported")
	}
	for _, url := range *asked {
		if strings.HasSuffix(url, "/realms/master") {
			t.Errorf("the Check asked about %q; master exists in every Keycloak and answers 200 "+
				"whatever the entity's own realm looks like", url)
		}
	}
}

func TestProvisionKeycloak_AsksAboutEveryDeclaredRealm(t *testing.T) {
	plans := realmPlans("cb-realm", "noc")
	s, asked := probeStep(t, plans, func(string) bool { return true })

	ok, err := s.Check(context.Background())
	if err != nil || !ok {
		t.Fatalf("every realm served must report satisfied: ok=%v err=%v", ok, err)
	}
	for _, plan := range plans {
		want := "/realms/" + plan.Realm
		found := false
		for _, url := range *asked {
			if strings.HasSuffix(url, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the Check never asked about realm %q (asked: %v); a realm nobody probes is a "+
				"realm that can go missing without the apply noticing", plan.Realm, *asked)
		}
	}
}

// One missing realm out of several is the upgrade case: an entity provisioned before a second
// realm was declared. Reporting satisfied there is what leaves it permanently absent.
func TestProvisionKeycloak_OneMissingRealmIsNotSatisfied(t *testing.T) {
	s, _ := probeStep(t, realmPlans("cb-realm", "noc"), func(url string) bool {
		return !strings.HasSuffix(url, "/realms/noc")
	})

	ok, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if ok {
		t.Error("one declared realm missing must report unsatisfied, so the Run imports it")
	}
}

// A step with nothing declared has nothing to gate on. It must not answer "satisfied" by
// finding no realms to check — the same vacuity the licence gate once had.
func TestProvisionKeycloak_NoDeclaredRealmsIsNotVacuouslySatisfied(t *testing.T) {
	s, asked := probeStep(t, nil, func(string) bool { return true })

	ok, _ := s.Check(context.Background())
	if ok && len(*asked) == 0 {
		t.Error("with no realms declared the Check probed nothing and reported satisfied; it must " +
			"fall back to a probe that at least proves the server is up")
	}
}
