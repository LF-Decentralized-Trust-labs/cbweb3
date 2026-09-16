// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The second half of the defect in keycloak_realm_readiness_test.go.
//
// Making the readiness Check gate on the declared realms stops a MISSING realm going unnoticed.
// It does nothing for a realm that exists and has drifted: the container runs
// `start-dev --import-realm`, and --import-realm skips a realm that is already there. So a
// changed origin, redirect URI, sslRequired or token lifespan is written into the import volume,
// ignored by Keycloak, and the apply reports success.
//
// A's apply had exactly one reconcile step — reconcile-admin-users, for roles, users, passwords
// and grants. Everything else the realm carries had no runtime convergence at all. This step is
// the counterpart for the realm itself and its clients, and it is the same idea Scenario B
// reached one release earlier for the NOC client's origins (reconcile-noc-origins): "already
// provisioned" must not be read as "already correct".
//
// Client SECRETS are deliberately out of scope. They are derived from the entity name
// (`<entity>-local-secret`, keycloak.go:161), so they cannot drift on their own — only by hand —
// and rotating one is an act with a blast radius: every backend holding the old value fails
// until it re-reads its environment. That is a deliberate operation, not something an apply
// should do in passing.

var probeRealm = KeycloakRealmPlan{
	Realm:       "cb-realm",
	Environment: "local",
	Origins:     []string{"http://localhost:5173", "http://localhost:5174"},
	Clients: []KeycloakClientPlan{
		{ClientID: "cb-client", Secret: "cb-local-secret", Roles: []string{"ROLE_GOVERNANCE"}},
	},
}

// currentState renders what the read script prints for a realm that matches the plan, so each
// test can perturb exactly one line.
func currentState(ssl, lifespan, origins, redirects string) string {
	return strings.Join([]string{
		"SSL " + ssl,
		"LIFESPAN " + lifespan,
		"ORIGINS cb-client " + origins,
		"REDIRECTS cb-client " + redirects,
	}, "\n") + "\n"
}

func matchingState() string {
	return currentState("none", "300",
		"http://localhost:5173,http://localhost:5174",
		"http://localhost:5173/*,http://localhost:5174/*")
}

func realmStep(out string, err error, recorded *[]string) *reconcileKeycloakRealmStep {
	return &reconcileKeycloakRealmStep{
		name: StepReconcileKeycloakRealm, entityPrefix: "cb",
		realms: []KeycloakRealmPlan{probeRealm},
		dockerExecCmd: func(_ context.Context, _, script string) ([]byte, error) {
			if recorded != nil {
				*recorded = append(*recorded, script)
			}
			return []byte(out), err
		},
	}
}

func TestReconcileKeycloakRealm_MatchingRealmIsSatisfied(t *testing.T) {
	ok, err := realmStep(matchingState(), nil, nil).Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !ok {
		t.Error("a realm that already matches the manifest must not be reconciled again")
	}
}

// The case an operator actually hits: a portal moves to a new host, the manifest gains an
// origin, and the import silently declines to apply it.
func TestReconcileKeycloakRealm_DriftedOriginIsNotSatisfied(t *testing.T) {
	out := currentState("none", "300",
		"http://localhost:5173",
		"http://localhost:5173/*,http://localhost:5174/*")

	ok, _ := realmStep(out, nil, nil).Check(context.Background())
	if ok {
		t.Error("a client missing a declared webOrigin must trigger reconciliation — otherwise the " +
			"new portal's browser calls are refused by CORS with nothing in the apply saying so")
	}
}

// Equality, not a subset test, and for Scenario B's reason: a superset leaves the portal working,
// so a subset test looks harmless — but it is what lets an origin added out of band persist
// forever, because the Check passes and the Run that normalises the list never happens.
func TestReconcileKeycloakRealm_UndeclaredOriginIsNotSatisfied(t *testing.T) {
	out := currentState("none", "300",
		"http://localhost:5173,http://localhost:5174,http://evil.example",
		"http://localhost:5173/*,http://localhost:5174/*")

	ok, _ := realmStep(out, nil, nil).Check(context.Background())
	if ok {
		t.Error("an origin nothing declares must trigger reconciliation; this list is the set of " +
			"pages allowed to read the client's token response")
	}
}

// Order is Keycloak's, not ours.
func TestReconcileKeycloakRealm_OriginOrderIsNotIdentity(t *testing.T) {
	out := currentState("none", "300",
		"http://localhost:5174,http://localhost:5173",
		"http://localhost:5174/*,http://localhost:5173/*")

	ok, _ := realmStep(out, nil, nil).Check(context.Background())
	if !ok {
		t.Error("the same set in another order is the same set; reconciling on order alone would " +
			"rewrite the realm on every apply")
	}
}

func TestReconcileKeycloakRealm_DriftedRealmSettingsAreNotSatisfied(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ssl      string
		lifespan string
	}{
		{"token lifespan lengthened by hand", "none", "1800"},
		{"sslRequired left at the Keycloak default", "external", "300"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := currentState(tc.ssl, tc.lifespan,
				"http://localhost:5173,http://localhost:5174",
				"http://localhost:5173/*,http://localhost:5174/*")
			if ok, _ := realmStep(out, nil, nil).Check(context.Background()); ok {
				t.Errorf("%s must trigger reconciliation", tc.name)
			}
		})
	}
}

// A provisioned entity with its containers down cannot answer. Reporting satisfied there would
// skip convergence exactly when it is needed — the same rule the admin-users step follows.
func TestReconcileKeycloakRealm_UnreachableKeycloakConverges(t *testing.T) {
	ok, err := realmStep("", errors.New("container not running"), nil).Check(context.Background())
	if err != nil {
		t.Fatalf("an unreachable Keycloak is an answer, not an error: %v", err)
	}
	if ok {
		t.Fatal("must report unsatisfied so the Run converges")
	}
}

func TestReconcileKeycloakRealm_RunSetsEverythingItChecks(t *testing.T) {
	var scripts []string
	if err := realmStep("", nil, &scripts).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected one script for one realm, got %d", len(scripts))
	}
	script := scripts[0]

	for _, want := range []string{
		"update realms/cb-realm",
		"sslRequired=none",
		"accessTokenLifespan=300",
		`webOrigins=["http://localhost:5173","http://localhost:5174"]`,
		`redirectUris=["http://localhost:5173/*","http://localhost:5174/*"]`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("the reconcile script does not set %q; the Check would keep reporting drift\n%s", want, script)
		}
	}
}

// No `|| true` on the client lookup. An absent client means provisioning never created it, which
// is exactly the silent outcome this step exists to end — swallowing it would converge nothing
// and report success.
func TestReconcileKeycloakRealm_AbsentClientFailsTheStep(t *testing.T) {
	var scripts []string
	_ = realmStep("", nil, &scripts).Run(context.Background())
	script := scripts[0]

	if !strings.Contains(script, "exit 1") {
		t.Error("the reconcile script never exits non-zero; a missing client would pass in silence")
	}
	if strings.Contains(script, "clientId=cb-client --fields id --format csv --noquotes) || true") {
		t.Error("the client lookup is guarded by `|| true`, so an absent client is swallowed")
	}
}

// The rule PR #234 established for the other kcadm script in this package, applied here so the
// new script cannot reintroduce what that one removed.
func TestReconcileKeycloakRealm_NeverPutsTheAdminSecretInArgv(t *testing.T) {
	var scripts []string
	step := realmStep(matchingState(), nil, &scripts)
	step.kcAdminPass = "s3ntinel-admin-secret-must-not-appear"

	if _, err := step.Check(context.Background()); err != nil {
		t.Fatalf("check: %v", err)
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, script := range scripts {
		if strings.Contains(script, step.kcAdminPass) {
			t.Error("the script embeds the Keycloak admin secret; it lands in the container's argv, " +
				"the host's and the toolkit's. Reference KC_BOOTSTRAP_ADMIN_PASSWORD instead")
		}
		if strings.Contains(script, "config credentials") && strings.Contains(script, "--password") {
			t.Error("the script passes --password to `config credentials`; let kcadm read KC_CLI_PASSWORD")
		}
	}
}

// A step absent from the canonical order never runs in a planned apply, and no other test would
// notice. The admin-users step has the same guard for the same reason.
func TestReconcileKeycloakRealmIsInTheCanonicalFoundOrder(t *testing.T) {
	provision, reconcile := -1, -1
	for i, name := range CanonicalStepOrder {
		switch name {
		case StepProvisionKeycloak:
			provision = i
		case StepReconcileKeycloakRealm:
			reconcile = i
		}
	}
	if reconcile < 0 {
		t.Fatalf("%s is not in CanonicalStepOrder, so a planned apply never runs it", StepReconcileKeycloakRealm)
	}
	if provision < 0 || reconcile < provision {
		t.Errorf("%s must come after %s: there is no realm to converge before it is imported",
			StepReconcileKeycloakRealm, StepProvisionKeycloak)
	}
}

// The convergence has to reach commercial banks, not only central banks.
//
// Join had zero reconcile steps: a bank's realm was imported once and then frozen, while its
// declarations kept following the manifest — spec.adminUsers for the roles, and
// spec.frontendHost / spec.proxy for the client's webOrigins and redirectUris. So the case this
// whole change exists for (a portal moves host, the manifest gains an origin, the apply reports
// success, the browser's calls are refused by CORS) reproduced on every bank, while the found
// pipeline was covered. In the sample stacks that is two banks per central bank.
func TestKeycloakConvergenceReachesCommercialBanks(t *testing.T) {
	provision, users, realm := -1, -1, -1
	for i, name := range CanonicalJoinStepOrder {
		switch name {
		case StepProvisionBankKeycloak:
			provision = i
		case StepReconcileAdminUsers:
			users = i
		case StepReconcileKeycloakRealm:
			realm = i
		}
	}
	if provision < 0 {
		t.Fatalf("%s is not in CanonicalJoinStepOrder", StepProvisionBankKeycloak)
	}
	for _, tc := range []struct {
		step  string
		at    int
		drift string
	}{
		{StepReconcileAdminUsers, users, "a role newly declared in spec.adminUsers never reaches a bank that is already provisioned"},
		{StepReconcileKeycloakRealm, realm, "an origin added to the manifest reaches the import volume and stops there"},
	} {
		if tc.at < 0 {
			t.Errorf("%s is not in CanonicalJoinStepOrder, so %s", tc.step, tc.drift)
			continue
		}
		if tc.at < provision {
			t.Errorf("%s must come after %s: there is no realm to converge before it exists",
				tc.step, StepProvisionBankKeycloak)
		}
	}
}
