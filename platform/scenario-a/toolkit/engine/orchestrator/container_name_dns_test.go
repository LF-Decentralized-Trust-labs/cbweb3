// SPDX-License-Identifier: Apache-2.0

// Guard for the finding that a generated container name can outgrow a DNS label.
//
// Containers reach each other BY NAME over the per-entity docker network — the
// reverse proxy dials the portals, and KC_DB_URL dials postgres — and a DNS label
// cannot exceed 63 octets (RFC 1035). Past that, Docker's embedded DNS refuses the
// name outright and the caller sees a 502 or a connection error against a container
// that is running and healthy. Nothing in the name says why.
//
// It has already happened in Scenario B: a central bank's governance and supervisor
// frontends came out at 64 octets and answered 502 through the proxy, while the
// treasury frontend at 62 resolved. That is what PR #226 fixes there, by routing the
// proxy through short network aliases.
//
// Scenario A is not broken today — its longest name is well inside the bound — but it
// has the same shape (`cbweb3-<entity>-<service>`) and nothing stopping it. The bound
// is on the SUM of a prefix nobody thinks about, an entity name the operator chooses,
// and the longest service suffix in the templates. This test reads all three and does
// the arithmetic, so the next long entity name fails here instead of in a deploy.
package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// dnsLabelMax is the maximum length of a single DNS label (RFC 1035 §2.3.4).
// Docker's embedded DNS enforces it on container names and network aliases.
const dnsLabelMax = 63

// entityServiceSuffix matches the container_name patterns whose value is the entity
// prefix plus a literal service suffix, e.g.
//
//	container_name: ${ENTITY_PREFIX}-payment-orchestrator
//	container_name: ${ENTITY_INFRA_PREFIX:?ENTITY_INFRA_PREFIX is required}-postgres
//
// ENTITY_INFRA_PREFIX and ENTITY_PREFIX are both set to entityContainerPrefix(entity)
// (see step_start_infra.go and step_start_backend_stack.go), so one rule covers both.
var entityServiceSuffix = regexp.MustCompile(`container_name:\s*\$\{ENTITY(?:_INFRA)?_PREFIX[^}]*\}((?:-[a-z0-9]+)+)`)

// templateServiceSuffixes returns every service suffix the compose templates append
// to the entity prefix, keyed by the template it came from.
//
// Read from the templates rather than listed here on purpose: a service added by
// copying an existing one inherits the naming, and a hardcoded list would not notice.
// An empty result is a failure, not a skip.
func templateServiceSuffixes(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "provisioning", "templates")
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range entityServiceSuffix.FindAllStringSubmatch(string(body), -1) {
			out[m[1]] = filepath.Base(path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("provisioning/templates not reachable from this module (the guard cannot run): %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("no entity-prefixed container_name found under %s — the guard would pass vacuously", root)
	}
	return out
}

// Every container name the toolkit will generate for a checked-in manifest must fit a
// DNS label. Covers the samples and the deploy-lnet manifests: an over-long name is a
// deploy-time failure in either.
func TestGeneratedContainerNamesFitDNSLabel(t *testing.T) {
	suffixes := templateServiceSuffixes(t)

	entities := map[string]string{} // entity name -> where it is declared
	for name, m := range sampleManifests(t) {
		if m.Metadata.Name != "" {
			entities[m.Metadata.Name] = name
		}
	}
	lnet := filepath.Join("..", "..", "..", "..", "deploy-lnet", "scenario-a", "manifests")
	for name, m := range deployLNETManifests(t, lnet) {
		if m.Metadata.Name != "" {
			entities[m.Metadata.Name] = name
		}
	}
	if len(entities) == 0 {
		t.Fatal("no manifest declared metadata.name — the guard checked nothing")
	}

	for entity, where := range entities {
		// entity is the manifest's metadata.name; see orchestrator.go, which does
		// exactly `entity := m.Metadata.Name` before deriving every name from it.
		prefix := entityContainerPrefix(entity)
		for suffix, template := range suffixes {
			if n := len(prefix + suffix); n > dnsLabelMax {
				t.Errorf("%s: entity %q produces container name %q (%d octets, limit %d, from %s): docker's embedded DNS will refuse it and callers see a 502 against a healthy container",
					where, entity, prefix+suffix, n, dnsLabelMax, template)
			}
		}
	}
}

// The bound is on prefix + entity + suffix, so the headroom an operator actually has
// depends on the longest suffix in the templates. This states it, and fails if a new
// service eats the margin without anyone noticing.
func TestEntityNameHeadroomIsStated(t *testing.T) {
	longest, from := "", ""
	for suffix, template := range templateServiceSuffixes(t) {
		if len(suffix) > len(longest) {
			longest, from = suffix, template
		}
	}
	// entityContainerPrefix is "cbweb3-" + entity, so the fixed cost is that literal
	// plus the longest suffix; what remains is the operator's budget for metadata.name.
	budget := dnsLabelMax - len(entityContainerPrefix("")) - len(longest)
	t.Logf("longest service suffix %q (%s); metadata.name may be up to %d octets", longest, from, budget)
	if budget < 20 {
		t.Errorf("an entity name may now be only %d octets (longest suffix %q from %s): the samples already use %d ('central-bank-costa-rica'), so this is too tight to be safe — shorten the suffix or route through a network alias",
			budget, longest, from, len("central-bank-costa-rica"))
	}
}
