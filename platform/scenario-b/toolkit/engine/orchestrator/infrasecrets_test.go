// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The property that matters most is STABILITY across runs. Postgres stores the
// password it was first initialised with, so a value regenerated on the second
// `apply` would lock the stack out of its own database — an idempotent tool that
// breaks on re-run is worse than a hardcoded credential.
func TestGeneratedSecretIsStableAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	first, err := resolveInfraSecret(dir, "POSTGRES_PASSWORD")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first == "" {
		t.Fatal("generated an empty secret")
	}
	second, err := resolveInfraSecret(dir, "POSTGRES_PASSWORD")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if first != second {
		t.Errorf("secret changed between calls: %q then %q; a re-run would lock the stack out of its own database", first, second)
	}
}

// Different entities must not share a password: the whole point of dropping the
// hardcoded one is that compromising a spoke does not hand over every other spoke.
func TestSecretsDifferPerDataDir(t *testing.T) {
	a, err := resolveInfraSecret(t.TempDir(), "POSTGRES_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	b, err := resolveInfraSecret(t.TempDir(), "POSTGRES_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two entities got the same generated password")
	}
}

// Two different credentials in the same entity must also differ, or Redis and
// Postgres end up sharing one secret and the blast radius is back.
func TestSecretsDifferPerName(t *testing.T) {
	dir := t.TempDir()
	pg, _ := resolveInfraSecret(dir, "POSTGRES_PASSWORD")
	rd, _ := resolveInfraSecret(dir, "REDIS_PASSWORD")
	if pg == rd {
		t.Error("POSTGRES_PASSWORD and REDIS_PASSWORD got the same value")
	}
}

// An operator-supplied value wins and is NOT persisted: the tool must not quietly
// keep a copy of a secret the operator manages elsewhere (a vault, a CI secret).
func TestOperatorSuppliedValueWinsAndIsNotPersisted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POSTGRES_PASSWORD", "from-the-operator")

	got, err := resolveInfraSecret(dir, "POSTGRES_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-the-operator" {
		t.Errorf("got %q, want the operator's value", got)
	}
	if raw, err := os.ReadFile(filepath.Join(dir, infraSecretsFile)); err == nil {
		if strings.Contains(string(raw), "from-the-operator") {
			t.Error("the operator's secret was written to disk; it must only be read from the environment")
		}
	}
}

// The file must not be world- or group-readable. It holds live credentials.
func TestSecretsFileIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	if _, err := resolveInfraSecret(dir, "REDIS_PASSWORD"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, infraSecretsFile))
	if err != nil {
		t.Fatalf("secrets file missing: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode is %04o, want 0600", perm)
	}
}

// A generated secret must be long enough and drawn from a wide alphabet, and must
// not be a recognisable constant. Guards against a future "simplification" that
// derives it from the entity name, which would be a hardcoded password again —
// computable by anyone holding the source.
func TestGeneratedSecretIsNotDerivedFromPublicData(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveInfraSecret(dir, "POSTGRES_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 32 {
		t.Errorf("secret is %d chars, want at least 32", len(got))
	}
	for _, bad := range []string{"cbweb3", "default", "admin", "password", filepath.Base(dir)} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(bad)) {
			t.Errorf("secret contains %q, so it is guessable: %q", bad, got)
		}
	}
}

// A config with no data dir (hand-built, as in these tests) must still get a real
// secret and must NOT write one into the working directory.
func TestNoDataDirGeneratesEphemeralWithoutWriting(t *testing.T) {
	before, _ := filepath.Glob(filepath.Join(".", infraSecretsFile))
	got, err := resolveInfraSecret("", "POSTGRES_PASSWORD")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) < 32 {
		t.Errorf("secret is %d chars, want at least 32", len(got))
	}
	after, _ := filepath.Glob(filepath.Join(".", infraSecretsFile))
	if len(after) != len(before) {
		t.Error("a secrets file was written into the working directory")
	}
	other, _ := resolveInfraSecret("", "POSTGRES_PASSWORD")
	if got == other {
		t.Error("ephemeral secrets repeated; they must not be a constant")
	}
}
