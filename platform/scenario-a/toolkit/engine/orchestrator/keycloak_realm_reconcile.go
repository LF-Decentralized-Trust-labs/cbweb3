// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// reconcile-keycloak-realm converges what the realm import cannot reapply.
//
// The Keycloak container runs `start-dev --import-realm`
// (provisioning/templates/entity-keycloak/keycloak-compose.yaml:38), and --import-realm does not
// apply a file for a realm that already exists. Everything the realm carries is therefore
// first-apply-only: a client's webOrigins and redirectUris, the realm's sslRequired and its
// access-token lifespan. The manifest could declare a new portal origin, the step would rewrite
// the JSON into the import volume, Keycloak would decline to read it, and the apply would report
// success. Nothing said otherwise, because A's apply had exactly one reconcile step —
// reconcile-admin-users, covering roles, users, passwords and grants and nothing else.
//
// Scenario B reached the same conclusion one release earlier for the NOC client's origins
// (reconcile-noc-origins): "already provisioned" must not be read as "already correct". This is
// the same idea for the realm itself, and the two are deliberate copies of an approach, not of
// code — the realms, clients and settings differ.
//
// Scope, stated because the omission is deliberate: client SECRETS are not converged. They are
// derived from the entity name (`<entity>-local-secret`, keycloak.go:161), so they cannot drift
// on their own, only by hand. Rotating one is an act with a blast radius — every backend holding
// the old value fails until it re-reads its environment — and that belongs to a deliberate
// operation, not to an apply passing through.

// realmSettingsFields are the realm-level attributes this step owns. Adding one here means
// adding it to the read script, the comparison and the reconcile script; the tests fail
// otherwise, which is the intended way to find out.
const (
	// accessTokenLifespanSeconds and sslRequiredFor come from keycloak.go, so the value this
	// step converges towards is by construction the same one renderRealmJSON writes. Reading
	// them from a second place is how the import and the reconciliation would drift apart.
	realmSSLField      = "sslRequired"
	realmLifespanField = "accessTokenLifespan"
)

// browserOrigin is the shape a webOrigin may take before it is embedded in the script. The whole
// script is one argv element handed to `bash -c`, so a value carrying a quote or a backslash
// would either break the JSON or, worse, be swallowed silently — which is how Scenario B once
// produced "Cannot parse the JSON" from inside a `|| true`.
var browserOrigin = regexp.MustCompile(`^https?://[A-Za-z0-9._~:\-\[\]]+$`)

// jsonOriginArray encodes origins as a JSON array literal for kcadm's `-s field=<json>` form,
// refusing anything it cannot prove safe to embed.
func jsonOriginArray(values []string) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("no origins given; a realm with none would fall back to a wildcard")
	}
	for _, v := range values {
		if !browserOrigin.MatchString(strings.TrimSuffix(v, "/*")) {
			return "", fmt.Errorf("origin %q is not a plain scheme://host[:port] value", v)
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode origins: %w", err)
	}
	return string(encoded), nil
}

// realmStateScript prints the realm's current state in labelled lines, one fact per line.
//
// One `--fields` call per attribute rather than one call returning several: kcadm's CSV gives no
// way to tell an empty second column from a missing one, and this runs once per apply, so
// unambiguity is worth the round-trips.
func realmStateScript(kc string, plan KeycloakRealmPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s >/dev/null 2>&1 || exit 1\n", kcadmLogin(kc))
	fmt.Fprintf(&b, "echo \"SSL $(%[1]s get realms/%[2]s --fields %[3]s --format csv --noquotes)\"\n",
		kc, plan.Realm, realmSSLField)
	fmt.Fprintf(&b, "echo \"LIFESPAN $(%[1]s get realms/%[2]s --fields %[3]s --format csv --noquotes)\"\n",
		kc, plan.Realm, realmLifespanField)
	for _, c := range plan.Clients {
		fmt.Fprintf(&b, "echo \"ORIGINS %[3]s $(%[1]s get clients -r %[2]s -q clientId=%[3]s --fields webOrigins --format csv --noquotes)\"\n",
			kc, plan.Realm, c.ClientID)
		fmt.Fprintf(&b, "echo \"REDIRECTS %[3]s $(%[1]s get clients -r %[2]s -q clientId=%[3]s --fields redirectUris --format csv --noquotes)\"\n",
			kc, plan.Realm, c.ClientID)
	}
	return b.String()
}

// realmReconcileScript brings the realm and its clients back to what the manifest declares.
func realmReconcileScript(kc string, plan KeycloakRealmPlan) (string, error) {
	origins, err := jsonOriginArray(plan.Origins)
	if err != nil {
		return "", fmt.Errorf("realm %s webOrigins: %w", plan.Realm, err)
	}
	redirects, err := jsonOriginArray(redirectURIsFor(plan.Origins))
	if err != nil {
		return "", fmt.Errorf("realm %s redirectUris: %w", plan.Realm, err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s || exit 1\n", kcadmLogin(kc))
	fmt.Fprintf(&b, "%[1]s update realms/%[2]s -s %[3]s=%[4]s -s %[5]s=%[6]d || exit 1\n",
		kc, plan.Realm, realmSSLField, sslRequiredFor(plan.Environment),
		realmLifespanField, accessTokenLifespanSeconds)
	for _, c := range plan.Clients {
		// No `|| true` on the lookup. An absent client means provisioning never created it,
		// which is precisely the silent outcome this step exists to end.
		fmt.Fprintf(&b, "KCID=$(%[1]s get clients -r %[2]s -q clientId=%[3]s --fields id --format csv --noquotes)\n",
			kc, plan.Realm, c.ClientID)
		fmt.Fprintf(&b, "[ -n \"$KCID\" ] || { echo 'client %[1]s not found in realm %[2]s' >&2; exit 1; }\n",
			c.ClientID, plan.Realm)
		fmt.Fprintf(&b, "%[1]s update clients/$KCID -r %[2]s -s 'webOrigins=%[3]s' -s 'redirectUris=%[4]s' || exit 1\n",
			kc, plan.Realm, origins, redirects)
	}
	return b.String(), nil
}

// realmState is the parsed output of realmStateScript.
type realmState struct {
	ssl       string
	lifespan  string
	origins   map[string][]string
	redirects map[string][]string
}

// parseRealmState reads the labelled lines. Anything it cannot read is left zero, which the
// comparison treats as drift — the safe direction, since converging a realm that was already
// correct costs one kcadm call and reporting a drifted realm as correct costs a silent outage.
func parseRealmState(out string) realmState {
	st := realmState{origins: map[string][]string{}, redirects: map[string][]string{}}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "SSL":
			st.ssl = fields[1]
		case "LIFESPAN":
			st.lifespan = fields[1]
		case "ORIGINS":
			if len(fields) >= 3 {
				st.origins[fields[1]] = splitCSVList(fields[2])
			} else {
				st.origins[fields[1]] = nil
			}
		case "REDIRECTS":
			if len(fields) >= 3 {
				st.redirects[fields[1]] = splitCSVList(fields[2])
			} else {
				st.redirects[fields[1]] = nil
			}
		}
	}
	return st
}

func splitCSVList(in string) []string {
	var out []string
	for _, f := range strings.Split(in, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// sameSet compares as sets: Keycloak returns these lists in its own order, and reconciling on
// order alone would rewrite the realm on every apply.
//
// Equality, not a subset test, and for the reason Scenario B recorded: a superset leaves the
// portal working, so a subset test looks harmless — but it is what would let an origin added out
// of band persist forever, because the Check passes and the Run that normalises the list never
// happens. This list is the set of pages allowed to read the client's token response.
func sameSet(have, want []string) bool {
	if len(have) != len(want) {
		return false
	}
	h := append([]string(nil), have...)
	w := append([]string(nil), want...)
	sort.Strings(h)
	sort.Strings(w)
	for i := range h {
		if h[i] != w[i] {
			return false
		}
	}
	return true
}

// realmSatisfied reports whether the realm already matches the plan in every attribute this step
// owns.
func realmSatisfied(out string, plan KeycloakRealmPlan) bool {
	st := parseRealmState(out)
	if !strings.EqualFold(st.ssl, sslRequiredFor(plan.Environment)) {
		return false
	}
	if st.lifespan != strconv.Itoa(accessTokenLifespanSeconds) {
		return false
	}
	wantRedirects := redirectURIsFor(plan.Origins)
	for _, c := range plan.Clients {
		if !sameSet(st.origins[c.ClientID], plan.Origins) {
			return false
		}
		if !sameSet(st.redirects[c.ClientID], wantRedirects) {
			return false
		}
	}
	return true
}

// reconcileKeycloakRealmStep converges the realm settings and client origins on every run.
type reconcileKeycloakRealmStep struct {
	name         string
	entityPrefix string
	// kcAdminPass is deliberately still held after the scripts stopped carrying it: the
	// exposure assertion in the tests can only prove the secret is not embedded if the step
	// actually has one to embed. Same reasoning as reconcileAdminUsersStep.
	kcAdminPass   string
	realms        []KeycloakRealmPlan
	dockerExecCmd func(ctx context.Context, container, script string) ([]byte, error)
}

func newReconcileKeycloakRealmStep(name, entityPrefix, kcAdminPass string, realms []KeycloakRealmPlan) Step {
	return &reconcileKeycloakRealmStep{
		name: name, entityPrefix: entityPrefix, kcAdminPass: kcAdminPass, realms: realms,
		dockerExecCmd: dockerExecScript,
	}
}

func (s *reconcileKeycloakRealmStep) Name() string { return s.name }

func (s *reconcileKeycloakRealmStep) container() string { return s.entityPrefix + "-keycloak" }

// Check reports satisfied only when every declared realm already matches. An unreachable
// Keycloak answers "not satisfied" so the Run converges — a provisioned entity with its
// containers down is ordinary, and skipping there is exactly when this must not skip.
func (s *reconcileKeycloakRealmStep) Check(ctx context.Context) (bool, error) {
	for _, plan := range s.realms {
		out, err := s.dockerExecCmd(ctx, s.container(), realmStateScript(keycloakAdminCLI, plan))
		if err != nil {
			return false, nil
		}
		if !realmSatisfied(string(out), plan) {
			return false, nil
		}
	}
	return true, nil
}

func (s *reconcileKeycloakRealmStep) Run(ctx context.Context) error {
	for _, plan := range s.realms {
		script, err := realmReconcileScript(keycloakAdminCLI, plan)
		if err != nil {
			return fmt.Errorf("build reconcile script for realm %s: %w", plan.Realm, err)
		}
		if _, err := s.dockerExecCmd(ctx, s.container(), script); err != nil {
			return fmt.Errorf("reconcile realm %s: %w", plan.Realm, err)
		}
	}
	return nil
}
