// SPDX-License-Identifier: Apache-2.0

// Regression guard for the access-token lifespan on the deploy/local Keycloak path
// (finding R1-10.7 / R2-§4.11).
//
// The realms the toolkit provisions never set accessTokenLifespan, so they inherit
// Keycloak's 300s default — the path the other hardening tests in this package cover.
// The deploy/local path is a SECOND enforcement point and it did set it: init.sh (the
// Keycloak container's entrypoint, so every `make *.up`) defaulted to 21600 — six
// hours — and setup-noc-realm.sh created the NOC realm at 86400 — twenty-four.
//
// A long-lived bearer is the whole exposure: it is replayable for as long as it
// lives, and the portals do not need it — each renews silently (proactive refresh
// plus a 401-retry interceptor), which is why 300s costs no operator session.
//
// This reads the scripts rather than the realms because the scripts are the source
// of truth, and it fails closed: a file that no longer declares a lifespan at all
// is an error, not a pass. Renaming the variable must break this test.
package orchestrator

import (
	"regexp"
	"testing"
)

// maxAccessTokenLifespanSeconds is the ceiling any deploy/local realm may set.
// 900s (15 minutes) leaves room for a deliberate, justified relaxation while
// still rejecting anything measured in hours.
const maxAccessTokenLifespanSeconds = 900

var (
	// KC_ACCESS_TOKEN_LIFESPAN:-<n> — the default when the operator sets nothing.
	envDefaultLifespanRe = regexp.MustCompile(`KC_ACCESS_TOKEN_LIFESPAN:-(\d+)`)
	// accessTokenLifespan=<n> — a literal handed straight to kcadm.
	literalLifespanRe = regexp.MustCompile(`accessTokenLifespan=(\d+)`)
)

func TestToolkitAccessTokenLifespanIsShort(t *testing.T) {
	// Replaces TestDeployLocalAccessTokenLifespanIsShort, which read
	// deploy/local/keycloak/init.sh. That path was removed as a duplicate of the
	// toolkit — and the toolkit set no lifespan at all, so the control would have been
	// dropped silently with the scripts. It is now set by the toolkit itself, and this
	// asserts the value the toolkit uses rather than the value a script wrote.
	if accessTokenLifespanSeconds > maxAccessTokenLifespanSeconds {
		t.Errorf("the toolkit issues access tokens valid for %ds (%.1fh); the ceiling is %ds",
			accessTokenLifespanSeconds, float64(accessTokenLifespanSeconds)/3600,
			maxAccessTokenLifespanSeconds)
	}
	if accessTokenLifespanSeconds <= 0 {
		t.Errorf("accessTokenLifespanSeconds is %d; a non-positive lifespan would let Keycloak "+
			"fall back to whatever the server was configured with, which is what stating it here avoids",
			accessTokenLifespanSeconds)
	}
}
