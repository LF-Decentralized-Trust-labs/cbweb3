// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The noc-portal client's web origins are registered by provision-keycloak-*, whose Check
// short-circuits as soon as KEYCLOAK_CLIENT_SECRET exists in the entity env. So on an entity
// that is ALREADY provisioned that step is skipped, and a newly declared origin — a standalone
// NOC portal added to spec.noc.portalOrigins — never reaches Keycloak. The portal stays dead
// with a CORS refusal until someone redeploys from scratch or edits Keycloak by hand.
//
// This is the same shape that made the Keycloak assertions in the hub-provisioning work
// unable to repair anything: "already provisioned" was read as "already correct". Registering
// origins is declarative and cheap, so it gets its own step that converges every run instead
// of riding on a step that only ever runs once.

// keycloakAdminCLI is kcadm inside the Keycloak container image.
const keycloakAdminCLI = "/opt/keycloak/bin/kcadm.sh"

// nocOriginsReconcileScript builds the kcadm script that sets the noc-portal client's
// webOrigins to exactly `origins`.
//
// It is declarative on purpose: the manifest is the source of truth for who may read this
// public client's token response, so an origin added by hand is removed on the next apply.
// That is the point of spec.noc.portalOrigins — a place to declare an origin that survives.
//
// The Check below enforces the same thing, and must: a subset test would pass whenever every
// declared origin is present, the step would skip, and this Run would never execute — so an
// extra origin added out of band would survive every apply. On a public client's CORS
// allowlist that is the difference between "re-applying revokes a rogue origin" and "it does
// not". The toolkit is the only writer of this field (create here, update there), so
// converging on the declared set costs no legitimate origin.
//
// The whole script is handed to `bash -c` as ONE argv element, so the single-quoted JSON array
// survives intact. Origins are validated first: this is the same embedding that once produced
// "Cannot parse the JSON" from a backslash-escaped value, swallowed by a `|| true`.
func nocOriginsReconcileScript(kc, realm, adminPassword string, origins []string) (string, error) {
	webOrigins, err := jsonStringArray(origins)
	if err != nil {
		return "", fmt.Errorf("noc-portal webOrigins: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%[1]s config credentials --server http://localhost:8080 --realm master "+
		"--user admin --password %[2]s && ", kc, adminPassword)
	// --format csv --noquotes prints the bare id, so no JSON parsing is needed in the shell.
	fmt.Fprintf(&b, "KCID=$(%[1]s get clients -r %[2]s -q clientId=%[3]s --fields id --format csv --noquotes) && ",
		kc, realm, nocKeycloakClient)
	// No `|| true` here. An absent client is a real failure: it means provisioning never
	// created it, which is precisely the silent outcome the existence check downstream of
	// appendNOCPortalClient was added to stop.
	fmt.Fprintf(&b, "{ [ -n \"$KCID\" ] || { echo 'client %[1]s not found in realm %[2]s' >&2; exit 1; }; } && ",
		nocKeycloakClient, realm)
	fmt.Fprintf(&b, "%[1]s update clients/$KCID -r %[2]s -s 'webOrigins=%[3]s'", kc, realm, webOrigins)
	return b.String(), nil
}

// nocOriginsReadScript prints the client's current webOrigins, one per line.
func nocOriginsReadScript(kc, realm, adminPassword string) string {
	return fmt.Sprintf("%[1]s config credentials --server http://localhost:8080 --realm master "+
		"--user admin --password %[2]s >/dev/null && "+
		"%[1]s get clients -r %[3]s -q clientId=%[4]s --fields webOrigins --format csv --noquotes",
		kc, adminPassword, realm, nocKeycloakClient)
}

// originsMatchDeclared reports whether the registered origins are EXACTLY the declared set.
//
// Equality, not a subset test. A superset still leaves the portal working, so a subset test
// looks harmless — but it is what would let an origin added out of band persist forever: with
// every declared origin present the Check passes, the step skips, and the Run that normalises
// the list never runs. This is a public client's CORS allowlist, so the set of origins that
// may read its token response is exactly the set the manifest declares.
//
// Order is not identity: Keycloak returns the list in its own order, so both sides are
// compared as sets.
func originsMatchDeclared(current []byte, want []string) bool {
	have := make(map[string]struct{})
	for _, line := range strings.Split(string(current), "\n") {
		for _, f := range strings.Split(line, ",") {
			if f = strings.TrimSpace(f); f != "" {
				have[f] = struct{}{}
			}
		}
	}
	wanted := make(map[string]struct{}, len(want))
	for _, w := range want {
		if w = strings.TrimSpace(w); w != "" {
			wanted[w] = struct{}{}
		}
	}
	if len(have) != len(wanted) {
		return false
	}
	for w := range wanted {
		if _, ok := have[w]; !ok {
			return false
		}
	}
	return true
}

// sortedOrigins is only for stable messages/tests; Keycloak does not care about order.
func sortedOrigins(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// reconcileNOCOrigins runs the update inside the entity's Keycloak container.
func reconcileNOCOrigins(ctx context.Context, r exec.CommandRunner, container, kc, realm, adminPassword string, origins []string) error {
	script, err := nocOriginsReconcileScript(kc, realm, adminPassword, origins)
	if err != nil {
		return err
	}
	if _, err := r.Run(ctx, "docker", "exec", container, "bash", "-c", script); err != nil {
		return fmt.Errorf("reconcile noc-portal webOrigins %v: %w", sortedOrigins(origins), err)
	}
	return nil
}

// nocOriginsAlreadyRegistered is the step's Check: true only when the registered origins are
// exactly the declared set, so the Run also converges a list that has drifted by addition.
//
// Any failure to ask — Keycloak not running yet, container absent, kcadm error — returns
// (false, nil), NOT an error: the step then runs, and its Run starts Keycloak before
// reconciling. Returning an error here would fail the whole apply on an entity whose data dir
// exists but whose containers are down, which is an ordinary state (the earlier steps that
// would have started Keycloak are themselves skipped once the entity is provisioned).
func nocOriginsAlreadyRegistered(ctx context.Context, r exec.CommandRunner, container, kc, realm, adminPassword string, origins []string) (bool, error) {
	out, err := r.Run(ctx, "docker", "exec", container, "bash", "-c", nocOriginsReadScript(kc, realm, adminPassword))
	if err != nil {
		return false, nil
	}
	return originsMatchDeclared(out, origins), nil
}
