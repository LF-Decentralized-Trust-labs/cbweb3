// SPDX-License-Identifier: Apache-2.0

package composetemplate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkNoSecrets strips every ${...} reference before looking for literal secret
// material, which means a secret hidden INSIDE a variable's default value —
// ${POSTGRES_PASSWORD:-cbweb3} — was invisible to it. That is a credential
// hardcoded in the template with two extra characters, and it is worse than an
// obvious one: it reads as parameterized, so a reviewer sees a variable and moves
// on, while every deployment that does not set the variable silently shares one
// well-known password.
func TestSecretDefaultIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want bool // want a finding
	}{
		{"password with a literal default", `      DSN: "postgres://u:${POSTGRES_PASSWORD:-cbweb3}@db:5432/x"`, true},
		{"password with a required marker", `      DSN: "postgres://u:${POSTGRES_PASSWORD:?}@db:5432/x"`, false},
		{"password with a message-carrying required marker", `      PW: "${REDIS_PASSWORD:?REDIS_PASSWORD is required}"`, false},
		{"bare password reference", `      PW: "${POSTGRES_PASSWORD}"`, false},
		{"empty default is not a secret", `      PW: "${POSTGRES_PASSWORD:-}"`, false},
		{"secret in the name, literal default", `      S: "${INTERNAL_RELAY_AUTH_SECRET:-dev-secret}"`, true},
		{"api key with a literal default", `      K: "${SOME_API_KEY:-abc123}"`, true},
		{"non-credential variable may default", `      TAG: "${POSTGRES_IMAGE_TAG:-17-alpine}"`, false},
		{"non-credential port may default", `      PORT: "${REDIS_PORT:-6379}"`, false},
		{"comment is ignored", `      # ${POSTGRES_PASSWORD:-cbweb3} was the old default`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := Result{OK: true}
			checkNoSecrets(tc.raw, &res)
			got := len(res.Errors) > 0
			if got != tc.want {
				t.Errorf("finding=%v want=%v for %q\n  findings: %v", got, tc.want, tc.raw, res.Errors)
			}
		})
	}
}

// The shipped templates must not carry credential defaults. This is the assertion
// that makes the check above worth having: it is what fails when someone adds a
// convenient fallback to keep a local deploy frictionless.
func TestShippedTemplatesCarryNoCredentialDefaults(t *testing.T) {
	root, err := filepath.Abs("../../../provisioning/templates")
	if err != nil {
		t.Fatalf("resolve template dir: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("template dir unavailable: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		checked++
		res := Result{OK: true}
		checkNoSecrets(string(raw), &res)
		for _, f := range res.Errors {
			t.Errorf("%s: %s", e.Name(), f)
		}
	}
	if checked == 0 {
		t.Fatal("no templates were checked; the path is wrong and this test proves nothing")
	}
	t.Logf("checked %d templates", checked)
}
