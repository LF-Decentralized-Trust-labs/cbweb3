// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// templatesDir is the provisioning templates root, relative to this package.
const templatesDir = "../../../provisioning/templates"

// aliasRe captures the network aliases a compose template declares, in template form.
var aliasRe = regexp.MustCompile(`aliases:\s*\n\s*- "([^"]+)"`)

// TestComposeAliasesMatchProxyRoutes pins the two halves of the fix together.
//
// The proxy resolves an upstream that only exists because a compose template declares it
// as a network alias. Nothing at runtime relates the two: change the alias in the template
// and the route still renders, the apply still succeeds, and the portal answers 502 —
// which is exactly how the Costa Rica outage presented. This test fails the moment either
// half moves without the other.
func TestComposeAliasesMatchProxyRoutes(t *testing.T) {
	netPrefix := "central-bank-costa-rica"

	cbAliases := templateAliases(t, "cb-frontend.compose.yaml", netPrefix)
	cb := SpokeConfig{NetPrefix: netPrefix}
	for _, role := range []string{"governance", "treasury", "supervisor"} {
		want := cb.frontendAlias(role)
		if !cbAliases[want] {
			t.Errorf("cb-frontend.compose.yaml declares no alias %q — ProxyRoutes() points at a name the container will not answer to.\n\tdeclared: %v", want, keysOf(cbAliases))
		}
	}

	gwAliases := templateAliases(t, "entity-backend.compose.yaml", netPrefix)
	if want := cb.apiGatewayAlias(); !gwAliases[want] {
		t.Errorf("entity-backend.compose.yaml declares no alias %q\n\tdeclared: %v", want, keysOf(gwAliases))
	}

	bankPrefix := "bank-itau"
	bankAliases := templateAliases(t, "entity-frontend.compose.yaml", bankPrefix)
	bank := JoinConfig{NetPrefix: bankPrefix}
	if want := bank.bankFrontendAlias(); !bankAliases[want] {
		t.Errorf("entity-frontend.compose.yaml declares no alias %q\n\tdeclared: %v", want, keysOf(bankAliases))
	}

	// The hub renders the SAME two templates with ENTITY=hub, so its routes are pinned
	// against them as well. Leaving it out is how it stayed on container names while the
	// CB and bank paths moved off: the guard covered the paths the outage touched, and
	// the one it did not cover was the one still carrying the defect.
	hubPrefix := "hub-cbweb3"
	hub := HubConfig{NetPrefix: hubPrefix}
	hubFrontend := templateAliases(t, "entity-frontend.compose.yaml", hubPrefix)
	if want := hub.frontendAlias(); !hubFrontend[want] {
		t.Errorf("entity-frontend.compose.yaml declares no alias %q for the hub\n\tdeclared: %v", want, keysOf(hubFrontend))
	}
	hubGateway := templateAliases(t, "entity-backend.compose.yaml", hubPrefix)
	if want := hub.apiGatewayAlias(); !hubGateway[want] {
		t.Errorf("entity-backend.compose.yaml declares no alias %q for the hub\n\tdeclared: %v", want, keysOf(hubGateway))
	}
}

// templateAliases reads a template's declared aliases with ENTITY_NET_PREFIX resolved.
func templateAliases(t *testing.T, name, netPrefix string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(templatesDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	out := map[string]bool{}
	for _, m := range aliasRe.FindAllStringSubmatch(string(raw), -1) {
		alias := m[1]
		for _, marker := range []string{"${ENTITY_NET_PREFIX:?}", "${ENTITY_NET_PREFIX}"} {
			alias = strings.ReplaceAll(alias, marker, netPrefix)
		}
		out[alias] = true
	}
	if len(out) == 0 {
		t.Fatalf("%s declares no network aliases; the proxy has nothing short enough to resolve", name)
	}
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
