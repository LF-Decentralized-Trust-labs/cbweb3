// SPDX-License-Identifier: Apache-2.0

// Guard for the finding that a generated container name can outgrow a DNS label.
//
// Containers reach each other BY NAME over the per-entity docker network — the
// reverse proxy dials the portals, and KC_DB_URL dials postgres — and a DNS label
// cannot exceed 63 octets (RFC 1035). Past that, Docker's embedded DNS refuses the
// name outright and the caller sees a 502 or a connection error against a container
// that is running and healthy. Nothing in the name says why.
//
// This is not hypothetical here. On the proxy-smoke topology a central bank's
// governance and supervisor frontends come out at 64 octets and answer 502, while its
// treasury frontend at 62 resolves. PR #226 fixes the proxy half of that by routing
// through short network aliases; this guard covers the other half, which an alias does
// not help: every OTHER name resolved container-to-container, postgres included, and
// every entity name a future manifest may declare.
//
// It lives in package apply because that is where the prefix is derived
// (sanitizePrefix + "sc-b-cbweb3-"), so the arithmetic sits next to its inputs.
package apply

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
)

// dnsLabelMax is the maximum length of a single DNS label (RFC 1035 §2.3.4).
// Docker's embedded DNS enforces it on container names and network aliases.
const dnsLabelMax = 63

// entityServiceSuffix matches the container_name patterns whose value is the
// container prefix, the entity, and a literal service suffix, e.g.
//
//	container_name: "${CONTAINER_PREFIX:?}-${ENTITY:?}-payment-orchestrator"
var entityServiceSuffix = regexp.MustCompile(`container_name:\s*"\$\{CONTAINER_PREFIX[^}]*\}-\$\{ENTITY[^}]*\}((?:-[a-z0-9]+)+)"`)

// templateServiceSuffixes returns every service suffix the compose templates append
// after the entity, keyed by the template it came from.
//
// Read from the templates rather than listed here on purpose: a service added by
// copying an existing one inherits the naming, and a hardcoded list would not notice.
// An empty result is a failure, not a skip.
func templateServiceSuffixes(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "provisioning", "templates")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("provisioning/templates not reachable from this module (the guard cannot run): %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, m := range entityServiceSuffix.FindAllStringSubmatch(string(body), -1) {
			out[m[1]] = e.Name()
		}
	}
	if len(out) == 0 {
		t.Fatalf("no CONTAINER_PREFIX/ENTITY container_name found under %s — the guard would pass vacuously", root)
	}
	return out
}

// containerBase reproduces the "<prefix>-<entity>" half of every generated name, the
// way apply.go builds it for each mode. Kept in one place, and pinned against a real
// deployed name by TestContainerBaseMatchesADeployedName so it cannot drift silently.
func containerBase(pd *manifest.ParticipantDeployment) string {
	prefix := "sc-b-cbweb3-" + sanitizePrefix(pd.Metadata.Name)
	var entity string
	switch pd.Spec.Mode {
	case manifest.ModeFoundHub:
		entity = "hub"
	case manifest.ModeFoundSpoke:
		// firstNonEmpty(pd.Spec.Topology.Role, "central-bank") — see apply.go.
		entity = "central-bank"
		if pd.Spec.Topology.Role != "" {
			entity = pd.Spec.Topology.Role
		}
	default: // join, and anything else that names itself
		entity = sanitizePrefix(pd.Metadata.Name)
	}
	return prefix + "-" + entity
}

// sampleAndLNETManifests loads every checked-in manifest of this scenario: the
// samples and the deploy-lnet templates. Generated state (cbweb3-data, bundles) is
// skipped — it is gitignored and carries whatever was last deployed locally.
func sampleAndLNETManifests(t *testing.T) map[string]*manifest.ParticipantDeployment {
	t.Helper()
	out := map[string]*manifest.ParticipantDeployment{}

	samples := filepath.Join("..", "..", "..", "samples")
	entries, err := os.ReadDir(samples)
	if err != nil {
		t.Fatalf("samples/ not reachable from this module (the guard cannot run): %v", err)
	}
	skip := map[string]bool{"cbweb3-data": true, "bundles": true}
	for _, e := range entries {
		if !e.IsDir() || skip[e.Name()] {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(samples, e.Name(), "*.yaml"))
		for _, f := range files {
			pd, err := manifest.Load(f)
			if err != nil {
				t.Fatalf("load %s: %v", f, err)
			}
			out[filepath.Join(e.Name(), filepath.Base(f))] = pd
		}
	}

	lnet := filepath.Join("..", "..", "..", "..", "deploy-lnet")
	for _, dir := range []string{
		filepath.Join(lnet, "scenario-b", "manifests"),
		filepath.Join(lnet, "hub", "manifests"),
	} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.yaml.tmpl"))
		for _, f := range files {
			pd, err := manifest.Load(f)
			if err != nil {
				t.Fatalf("load %s: %v", f, err)
			}
			out[filepath.Join(filepath.Base(filepath.Dir(filepath.Dir(f))), filepath.Base(f))] = pd
		}
	}

	if len(out) == 0 {
		t.Fatal("no manifests found — the guard would pass vacuously")
	}
	return out
}

// The Scenario B samples and deploy-lnet manifests DO cross the bound today, so this
// cannot be a plain assertion — it would fail on develop for a defect it does not own.
// It is a ratchet instead: it reports the whole over-long set and fails when the worst
// case gets WORSE, so the naming cannot quietly degrade while the fix is in flight.
//
// Why the deploys still work. Docker Compose registers each SERVICE name as a network
// alias, so a container whose container_name overflows is still reachable at its short
// service name from inside the project — which is how the api-gateway keeps talking to
// a 65-octet payment-orchestrator. Only a consumer that addresses the container_name
// itself is affected, and the reverse proxy is one: it is an external container joined
// to the network, with no service of its own. That is the observed 502 on governance
// and supervisor, and it is what PR #226 fixes by routing through short aliases.
//
// So the over-long names divide in two:
//   - resolved by container_name today  -> broken, fixed by #226 (the portals)
//   - resolved by service alias today   -> latent, and a trap for the next consumer
//     that reaches for the container_name (payment-orchestrator, postgres, …)
//
// worstOverflow is the longest generated name across all checked-in manifests, measured
// on develop. Lower it when names get shorter; raising it needs a reason in the PR.
const worstOverflow = 69 // sc-b-cbweb3-central-bank-costa-rica-central-bank-payment-orchestrator

func TestGeneratedContainerNamesFitDNSLabel(t *testing.T) {
	suffixes := templateServiceSuffixes(t)

	type overflow struct {
		name, where, template string
	}
	var over []overflow
	worst := 0
	for where, pd := range sampleAndLNETManifests(t) {
		base := containerBase(pd)
		for suffix, template := range suffixes {
			name := base + suffix
			if len(name) <= dnsLabelMax {
				continue
			}
			over = append(over, overflow{name, where, template})
			if len(name) > worst {
				worst = len(name)
			}
		}
	}

	sort.Slice(over, func(i, j int) bool { return len(over[i].name) > len(over[j].name) })
	for _, o := range over {
		t.Logf("%d octets (limit %d): %s  [%s, %s]", len(o.name), dnsLabelMax, o.name, o.where, o.template)
	}
	t.Logf("%d generated names exceed the DNS label limit; worst case %d octets", len(over), worst)

	if worst > worstOverflow {
		t.Errorf("worst generated container name is now %d octets, past the %d recorded on develop: a new entity name or service suffix made the DNS-label problem worse. Shorten it, or route the consumer through a network alias (see PR #226).",
			worst, worstOverflow)
	}
	if len(over) > 0 && worst < worstOverflow {
		t.Errorf("worst generated container name is down to %d octets from the recorded %d — lower worstOverflow so the ratchet keeps its grip",
			worst, worstOverflow)
	}
	if len(over) == 0 {
		t.Errorf("no generated name exceeds %d octets any more — the fix landed; drop this ratchet and assert the bound outright, as scenario-a does",
			dnsLabelMax)
	}
}

// containerBase reimplements a rule that lives in apply.go. Pin it against a name a
// real deploy produced, so a change there fails here instead of making the guard lie.
func TestContainerBaseMatchesADeployedName(t *testing.T) {
	pd := &manifest.ParticipantDeployment{}
	pd.Metadata.Name = "central-bank-brazil"
	pd.Spec.Mode = manifest.ModeFoundSpoke
	// Observed on a live found-spoke run of scenario-b/samples/brazil:
	//   sc-b-cbweb3-central-bank-brazil-central-bank-governance-frontend
	const want = "sc-b-cbweb3-central-bank-brazil-central-bank"
	if got := containerBase(pd); got != want {
		t.Errorf("containerBase() = %q; want %q (the prefix+entity a real deploy produced) — apply.go's derivation changed and this guard's arithmetic is now wrong", got, want)
	}
}

// The bound is on prefix + entity + suffix, so what an operator may name an entity
// depends on the longest suffix in the templates. Scenario B's budget is already
// smaller than the names its own samples use — that is the same defect from the other
// side — so this reports the number and ratchets it: it fails if a new service suffix
// shrinks the budget further.
//
// entityNameBudget is what a found-spoke metadata.name may be today, measured on
// develop. It is 17 while the samples use 19 ("central-bank-brazil"), which is exactly
// why their frontends overflow.
const entityNameBudget = 17

func TestEntityNameHeadroomIsStated(t *testing.T) {
	longest, from := "", ""
	for suffix, template := range templateServiceSuffixes(t) {
		if len(suffix) > len(longest) {
			longest, from = suffix, template
		}
	}
	// A found-spoke pays "sc-b-cbweb3-" + name + "-central-bank" + suffix, so the name
	// is charged once and the role once. This is the tightest of the three modes.
	budget := dnsLabelMax - len("sc-b-cbweb3-") - len("-central-bank") - len(longest)
	t.Logf("longest service suffix %q (%s); a found-spoke metadata.name may be up to %d octets, while the samples use %d",
		longest, from, budget, len("central-bank-brazil"))

	if budget < entityNameBudget {
		t.Errorf("a found-spoke entity name may now be only %d octets, down from the %d recorded on develop (longest suffix %q from %s): a new service suffix tightened an already-broken bound",
			budget, entityNameBudget, longest, from)
	}
	if budget > entityNameBudget {
		t.Errorf("the entity-name budget grew to %d octets from the recorded %d — raise entityNameBudget so the ratchet keeps its grip",
			budget, entityNameBudget)
	}
}
