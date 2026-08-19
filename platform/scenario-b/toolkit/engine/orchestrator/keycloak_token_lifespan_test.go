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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

func TestDeployLocalAccessTokenLifespanIsShort(t *testing.T) {
	for _, name := range []string{"init.sh", "setup-noc-realm.sh"} {
		path, err := filepath.Abs(filepath.Join("../../../deploy/local/keycloak", name))
		if err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		script := string(body)

		found := 0
		for _, re := range []*regexp.Regexp{envDefaultLifespanRe, literalLifespanRe} {
			for _, m := range re.FindAllStringSubmatch(script, -1) {
				found++
				secs, err := strconv.Atoi(m[1])
				if err != nil {
					t.Fatalf("%s: unparseable lifespan %q", name, m[1])
				}
				if secs > maxAccessTokenLifespanSeconds {
					t.Errorf("%s sets an access-token lifespan of %ds (%.1fh); the ceiling is %ds",
						name, secs, float64(secs)/3600, maxAccessTokenLifespanSeconds)
				}
			}
		}
		if found == 0 {
			t.Errorf("%s declares no access-token lifespan; this guard must not pass by "+
				"failing to find one — update the patterns if the script was reworked", name)
		}
	}
}
