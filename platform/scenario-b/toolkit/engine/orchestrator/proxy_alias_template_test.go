// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// templatesDir is the provisioning templates root, relative to this package.
const templatesDir = "../../../provisioning/templates"

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

	// The fourth route set, and the one PR #226 left behind for the same reason it nearly left
	// the hub behind: the audit reached as far as the author was already looking. The observe
	// stack has its own template and its own network, so nothing about the CB/bank/hub fix
	// touched it.
	nocPrefix := "sc-b-cbweb3-noc-brazil"
	noc := ObserveConfig{NetPrefix: nocPrefix}
	nocAliases := templateAliasesIn(t, templatesDir, "noc-stack.compose.yaml", nocPrefix)
	for _, want := range []string{noc.nocPortalAlias(), noc.nocBackendAlias()} {
		if !nocAliases[want] {
			t.Errorf("noc-stack.compose.yaml declares no alias %q — the proxy would dial a name the "+
				"container will not answer to.\n\tdeclared: %v", want, keysOf(nocAliases))
		}
	}
}

// templateAliases reads a template's declared aliases with ENTITY_NET_PREFIX resolved.
func templateAliases(t testing.TB, name, netPrefix string) map[string]bool {
	t.Helper()
	return templateAliasesIn(t, templatesDir, name, netPrefix)
}

// templateAliasesIn parses one compose template and returns every network alias it declares,
// with ENTITY_NET_PREFIX resolved.
//
// It parses the YAML rather than matching it. The regex this replaced captured only the FIRST
// item of each `aliases:` block and only when quoted, so a service with two aliases failed the
// guard while declaring exactly what it needed — a false red, which is worse than no guard
// because it teaches the next person to distrust a true one. See proxy_alias_reader_test.go.
//
// The dir parameter exists so the reader can be tested against fixtures: a real template that
// happens to exercise a case today would stop exercising it tomorrow.
func templateAliasesIn(t testing.TB, dir, name, netPrefix string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
		return nil
	}

	var doc struct {
		Services map[string]struct {
			// yaml.Node because compose accepts two shapes here: the short list form
			// (`networks: [entity_net]`), which carries no aliases, and the map form, which
			// may. Decoding straight into a map would fail the whole file on the first
			// service written the short way.
			Networks yaml.Node `yaml:"networks"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", name, err)
		return nil
	}

	out := map[string]bool{}
	for _, svc := range doc.Services {
		if svc.Networks.Kind != yaml.MappingNode {
			continue // short form: no aliases to declare
		}
		// A mapping node's Content alternates key, value.
		for i := 1; i < len(svc.Networks.Content); i += 2 {
			var net struct {
				Aliases []string `yaml:"aliases"`
			}
			if err := svc.Networks.Content[i].Decode(&net); err != nil {
				continue // `entity_net:` with a null body is legal and declares nothing
			}
			for _, alias := range net.Aliases {
				out[resolveNetPrefix(alias, netPrefix)] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s declares no network aliases; the proxy has nothing short enough to resolve", name)
		return nil
	}
	return out
}

// resolveNetPrefix substitutes the two spellings of the variable the templates use. Both appear
// in the tree; `:?` is compose's "required, fail if unset" form.
func resolveNetPrefix(alias, netPrefix string) string {
	// NOC_NET_PREFIX is the observe stack's spelling of the same idea; noc-stack.compose.yaml
	// has its own network and its own variable.
	for _, marker := range []string{
		"${ENTITY_NET_PREFIX:?}", "${ENTITY_NET_PREFIX}",
		"${NOC_NET_PREFIX:?}", "${NOC_NET_PREFIX}",
	} {
		alias = strings.ReplaceAll(alias, marker, netPrefix)
	}
	return alias
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
