// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
)

// manifest.MaxMetadataNameLen is a number, and a number in one package about strings in another
// is exactly the kind of thing that is right on the day it is written and wrong a release later.
//
// This reads the alias suffixes out of the compose templates and recomputes the budget. A new
// service with a longer suffix — say "-noc-orchestrator" — tightens it, and this fails instead of
// quietly leaving names that validation accepts and Docker refuses.
//
// It lives in this package rather than in engine/manifest because manifest must not depend on the
// orchestrator or on the provisioning tree; the arrow points this way.

// aliasSuffixRe matches the part a template appends after the network-prefix variable, e.g.
// "-api-gateway" in "${ENTITY_NET_PREFIX:?}-api-gateway".
var aliasSuffixRe = regexp.MustCompile(`\$\{(?:ENTITY|NOC)_NET_PREFIX[^}]*\}(-[a-z0-9-]+)`)

func TestMaxMetadataNameLenMatchesTheLongestAlias(t *testing.T) {
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		t.Fatalf("read %s: %v", templatesDir, err)
	}

	longest, from := "", ""
	scanned := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(templatesDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		scanned++
		for _, m := range aliasSuffixRe.FindAllStringSubmatch(string(raw), -1) {
			if len(m[1]) > len(longest) {
				longest, from = m[1], e.Name()
			}
		}
	}

	// A guard that silently scanned nothing would agree with any number forever.
	if scanned == 0 {
		t.Fatalf("no compose templates found under %s", templatesDir)
	}
	if longest == "" {
		t.Fatalf("scanned %d templates and found no alias built from a network prefix; either the "+
			"aliases are gone — in which case the proxy is back on container names — or this "+
			"pattern no longer matches how they are written", scanned)
	}

	want := dnsLabelMax - len(longest)
	if manifest.MaxMetadataNameLen != want {
		t.Errorf("manifest.MaxMetadataNameLen is %d, but the longest alias suffix in the templates "+
			"is %q (%s), which leaves %d octets for metadata.name.\n"+
			"A name between %d and %d would pass validation and produce an alias Docker's resolver "+
			"refuses. Update the constant, and say in its comment which suffix set it.",
			manifest.MaxMetadataNameLen, longest, from, want,
			min(want, manifest.MaxMetadataNameLen)+1, max(want, manifest.MaxMetadataNameLen))
	}
}

// The other direction: every alias the Go helpers build must also fit the budget. A helper could
// append a suffix no template declares — TestComposeAliasesMatchProxyRoutes would catch that, but
// only for the routes it knows about, and the arithmetic here is cheap.
func TestGoAliasSuffixesFitTheSameBudget(t *testing.T) {
	name := strings.Repeat("n", manifest.MaxMetadataNameLen)
	cb := SpokeConfig{NetPrefix: name}
	bank := JoinConfig{NetPrefix: name}
	hub := HubConfig{NetPrefix: name}
	noc := ObserveConfig{NetPrefix: name}

	aliases := []string{
		cb.frontendAlias("governance"), cb.frontendAlias("treasury"), cb.frontendAlias("supervisor"),
		cb.apiGatewayAlias(), bank.bankFrontendAlias(), bank.apiGatewayAlias(),
		hub.frontendAlias(), hub.apiGatewayAlias(),
		noc.nocPortalAlias(), noc.nocBackendAlias(),
	}
	for _, alias := range aliases {
		if len(alias) > dnsLabelMax {
			t.Errorf("with a name at the declared limit (%d octets), alias %q is %d octets — over "+
				"the %d-octet DNS label limit. Either the budget is wrong or this helper's suffix is "+
				"longer than the templates admit.", manifest.MaxMetadataNameLen, alias, len(alias), dnsLabelMax)
		}
	}
}
