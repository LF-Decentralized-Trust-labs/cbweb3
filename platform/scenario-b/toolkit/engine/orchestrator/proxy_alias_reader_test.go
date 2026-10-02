// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

// The reader behind TestComposeAliasesMatchProxyRoutes used to be a regex:
//
//	regexp.MustCompile(`aliases:\s*\n\s*- "([^"]+)"`)
//
// It had two failure modes, and they point opposite ways.
//
//   - It captured only the FIRST item of each `aliases:` block. A service that grew a second
//     alias, with the proxy's one second, failed the guard — and the message said the template
//     declares no such alias, when it declares exactly that. A false red costs whoever touches
//     the template an afternoon, and teaches them the guard is unreliable.
//   - It required quotes. `- ${ENTITY_NET_PREFIX:?}-governance` is valid YAML and the regex did
//     not see it. That one fails safe, but only by luck.
//
// These cases pin the reader itself rather than the templates, so a future rewrite has to keep
// the behaviour that made the regex wrong. Fixtures, not real templates: a real template that
// happens to satisfy the case today would stop testing it tomorrow.

func writeTemplate(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	name := "fixture.compose.yaml"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

func TestTemplateAliases_ReadsEveryAliasInABlock(t *testing.T) {
	dir := writeTemplate(t, `
services:
  frontend:
    networks:
      entity_net:
        aliases:
          - "something-else"
          - "${ENTITY_NET_PREFIX:?}-governance"
`)
	got := templateAliasesIn(t, dir, "fixture.compose.yaml", "cb-costa-rica")

	if !got["cb-costa-rica-governance"] {
		t.Errorf("the second alias in the block was not read; a service with two aliases would "+
			"fail the guard while declaring exactly what it needs. Read: %v", keysOf(got))
	}
	if !got["something-else"] {
		t.Errorf("the first alias was dropped: %v", keysOf(got))
	}
}

func TestTemplateAliases_ReadsUnquotedAliases(t *testing.T) {
	dir := writeTemplate(t, `
services:
  frontend:
    networks:
      entity_net:
        aliases:
          - ${ENTITY_NET_PREFIX:?}-governance
`)
	if got := templateAliasesIn(t, dir, "fixture.compose.yaml", "cb-costa-rica"); !got["cb-costa-rica-governance"] {
		t.Errorf("an unquoted alias was not read; it is valid YAML and the template may be written "+
			"either way. Read: %v", keysOf(got))
	}
}

// Both spellings of the variable appear in the templates, and both must resolve.
func TestTemplateAliases_ResolvesBothVariableForms(t *testing.T) {
	dir := writeTemplate(t, `
services:
  a:
    networks:
      entity_net:
        aliases: ["${ENTITY_NET_PREFIX}-plain"]
  b:
    networks:
      entity_net:
        aliases: ["${ENTITY_NET_PREFIX:?}-required"]
`)
	got := templateAliasesIn(t, dir, "fixture.compose.yaml", "pfx")
	for _, want := range []string{"pfx-plain", "pfx-required"} {
		if !got[want] {
			t.Errorf("%q not resolved: %v", want, keysOf(got))
		}
	}
}

// The short form `networks: [entity_net]` carries no aliases and is valid compose. The reader
// must skip it rather than fail parsing the whole file — a template mixing both forms would
// otherwise report zero aliases for the services that do declare them.
func TestTemplateAliases_ToleratesTheShortNetworkForm(t *testing.T) {
	dir := writeTemplate(t, `
services:
  plain:
    networks:
      - entity_net
  aliased:
    networks:
      entity_net:
        aliases: ["${ENTITY_NET_PREFIX:?}-governance"]
`)
	if got := templateAliasesIn(t, dir, "fixture.compose.yaml", "pfx"); !got["pfx-governance"] {
		t.Errorf("a template mixing the list and map forms of `networks` lost the aliases: %v", keysOf(got))
	}
}

// A template with no aliases at all must be a failure, not an empty answer: the guard's whole
// premise is that the proxy has a short name to resolve.
func TestTemplateAliases_NoAliasesIsAFailure(t *testing.T) {
	dir := writeTemplate(t, "services:\n  plain:\n    image: nginx\n")
	fake := &fakeT{T: t}
	templateAliasesIn(fake, dir, "fixture.compose.yaml", "pfx")
	if !fake.failed {
		t.Error("a template declaring no aliases returned quietly; the guard would pass vacuously")
	}
}

// fakeT records a Fatalf instead of aborting, so a test can assert that the reader refuses
// rather than returning an empty answer. Embeds *testing.T to satisfy testing.TB, whose
// unexported method makes it unimplementable from outside the testing package.
type fakeT struct {
	*testing.T
	failed bool
}

func (f *fakeT) Fatalf(string, ...any) { f.failed = true }
func (f *fakeT) Errorf(string, ...any) { f.failed = true }
