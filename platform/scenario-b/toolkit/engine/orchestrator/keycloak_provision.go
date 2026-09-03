// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"fmt"
	"strings"
)

// Keycloak provisioning is a chain of kcadm commands run in one `bash -c`. Every create
// has to tolerate a second `apply` finding the object already there, which is why each one
// carried `|| true`. That suffix cannot tell the case it exists for — "already exists" —
// from the cases it must not swallow: a malformed argument, an auth failure, an
// unreachable server, a schema rejection. Each of those read as success, and the step
// reported `done` over a realm that was missing whatever had failed.
//
// What replaces it: `kcw` tolerates the re-run but keeps the reason on stderr, and
// appendKeycloakAssertions fails the step when the realm does not hold what it should —
// whatever the reason, including a create that "succeeded" without producing the object.

// kcadmPreamble defines the shell helper that replaces `|| true`. It keeps the command
// chain going (a re-run must not fail on an existing object) while putting the reason on
// stderr, so a failure that is NOT "already exists" leaves a trace to debug from instead
// of vanishing.
const kcadmPreamble = `kcw() { echo "kcadm: $1 did not apply cleanly (continuing; the end state is asserted below)" >&2; }; `

// appendKeycloakAssertions appends end-state checks for everything whose silent absence
// breaks the entity, and fails the step naming the missing object. This is the check that
// holds regardless of *why* something is missing: a create that failed, a create that
// succeeded against the wrong realm, or an object removed between steps.
//
// It is deliberately a handful of assertions rather than one per create: a single
// `get users` covers every declared account, so coverage does not cost a round-trip per
// command.
//
// Each assertion is written as `{ check; } || { echo …; exit 1; }`, mirroring the check
// already guarding the noc-portal client.
//
// Scope, stated because it is easy to over-read: these run when the provisioning step
// runs, which is the FIRST apply for an entity. The step's Check short-circuits once
// KEYCLOAK_CLIENT_SECRET is present in the env file, so a later apply skips the whole
// chain and these assertions with it. That covers the defect they exist for — a first
// provisioning that reports `done` over a realm missing what it should hold — but it is
// not a drift detector: an object deleted after provisioning is not noticed by a
// re-apply. Making it one means running the assertions outside the skipped step, which
// is a separate change.
func appendKeycloakAssertions(b *strings.Builder, kc, realm, confidentialClient string, users []AdminUser) {
	// The realm itself. Everything below is meaningless without it, and its absence is
	// the failure mode that produces the most confusing downstream errors.
	fmt.Fprintf(b, " && ({ %[1]s get realms/%[2]s --fields realm | grep -q '\"%[2]s\"'; } "+
		"|| { echo 'keycloak: realm %[2]s does not exist after provisioning' >&2; exit 1; })",
		kc, realm)

	// sslRequired=NONE is what lets the browser-direct password grant work over plain
	// HTTP in the local lab. Without it Keycloak answers "HTTPS required" and the NOC
	// portal cannot log in — a failure that looks like bad credentials, not like
	// provisioning.
	// Keycloak normalises the value to lower case ("none"), so the check is
	// case-insensitive: matching the literal NONE we sent fails against what it stores.
	fmt.Fprintf(b, " && ({ %[1]s get realms/%[2]s --fields sslRequired | grep -qi '\"none\"'; } "+
		"|| { echo 'keycloak: realm %[2]s did not accept sslRequired=NONE; browser password grants will fail with \"HTTPS required\"' >&2; exit 1; })",
		kc, realm)

	// The confidential backend client. Without it the api-gateway cannot obtain a token
	// and every authenticated route fails at once.
	fmt.Fprintf(b, " && ({ %[1]s get clients -r %[2]s -q clientId=%[3]s --fields id | grep -q '\"id\"'; } "+
		"|| { echo 'keycloak: client %[3]s does not exist in realm %[2]s; the backend cannot obtain a token' >&2; exit 1; })",
		kc, realm, confidentialClient)

	// Every operator account the manifest declares. A realm with no users is the exact
	// defect this assertion exists to stop shipping silently: the manifest promises
	// credentials, the portal refuses them, and nothing in the run says so.
	for _, u := range users {
		fmt.Fprintf(b, " && ({ %[1]s get users -r %[2]s -q username=%[3]s --fields username | grep -q '\"%[3]s\"'; } "+
			"|| { echo 'keycloak: user %[3]s was not created in realm %[2]s; the manifest declares credentials that do not exist' >&2; exit 1; })",
			kc, realm, u.Username)
	}
}
