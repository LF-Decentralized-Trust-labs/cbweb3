// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// infraSecretsFile holds the per-entity infrastructure credentials this toolkit
// generates. It lives beside the entity's provisioning state, is created 0600, and
// is read back on every later run.
const infraSecretsFile = ".infra-secrets.env"

// resolveInfraSecret returns the value to use for an infrastructure credential
// (POSTGRES_PASSWORD, REDIS_PASSWORD, KC_ADMIN_PASSWORD) for the entity whose
// state lives in dataDir.
//
// Why this exists. These credentials used to be constants in this file: every
// entity of every deployment — local lab and VM alike — got POSTGRES_PASSWORD
// "cbweb3" and Keycloak admin/admin. Anyone with the source had the password for
// every database in every network, and compromising one entity handed over the
// rest. Redis had no password at all while holding the relay-auth replay guard and
// the auth service's PKI login nonces, so deleting a key there re-arms a signature
// that was already spent.
//
// Resolution order, and the reasoning for each step:
//
//  1. The environment wins. An operator who manages secrets elsewhere (a vault, CI)
//     must be able to supply them, and the value is deliberately NOT persisted —
//     the tool should not keep its own copy of a secret it does not own.
//
//  2. Otherwise a previously generated value is read back from dataDir. This is the
//     property that actually matters: Postgres stores the password it was
//     initialised with, so regenerating on the second `apply` would lock the stack
//     out of its own database. An idempotent tool that breaks on re-run would be
//     worse than the hardcoded credential it replaced.
//
//  3. Otherwise one is generated from crypto/rand and persisted. Deliberately NOT
//     derived from the entity name or any other public input: a deterministic
//     function of public data is a hardcoded password with extra steps, computable
//     by anyone holding this source.
func resolveInfraSecret(dataDir, name string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v, nil
	}

	// No dataDir means there is nowhere stable to persist to — hand-built configs in
	// tests, mostly. Generate an ephemeral value rather than writing into the working
	// directory, and never fall back to a constant: a test that silently used a fixed
	// password would be the very thing this function removes.
	if strings.TrimSpace(dataDir) == "" {
		return generateSecret(name)
	}

	path := filepath.Join(dataDir, infraSecretsFile)
	existing, err := readInfraSecrets(path)
	if err != nil {
		return "", err
	}
	if v, ok := existing[name]; ok && v != "" {
		return v, nil
	}

	value, err := generateSecret(name)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dataDir, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s=%s\n", name, value); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	// An earlier run (or a restore) may have left a laxer mode behind.
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("chmod %s: %w", path, err)
	}
	return value, nil
}

// generateSecret returns 32 bytes of crypto/rand as URL-safe base64.
func generateSecret(name string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate %s: %w", name, err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// readInfraSecrets parses NAME=VALUE lines. A missing file is not an error: it is
// the first run.
func readInfraSecrets(path string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

// secretsDirOf maps a rendered env-file path to the entity data dir that holds the
// secrets file. An empty path means "no stable location" (hand-built configs in
// tests), which resolveInfraSecret answers with an ephemeral value.
func secretsDirOf(envFile string) string {
	if strings.TrimSpace(envFile) == "" {
		return ""
	}
	return filepath.Dir(envFile)
}

// mustInfraSecret is resolveInfraSecret for the compose-env builders, which return
// a map rather than an error.
//
// Failing here means crypto/rand or the data dir is unusable, and the alternative
// to stopping is emitting an env without the credential — which compose would then
// reject at `${...:?}` with a message about a missing variable, sending the reader
// looking for a configuration mistake that is not there. A clear stop beats a
// misleading error two layers down.
func mustInfraSecret(dataDir, name string) string {
	v, err := resolveInfraSecret(dataDir, name)
	if err != nil {
		panic(fmt.Sprintf("orchestrator: cannot resolve %s for %q: %v", name, dataDir, err))
	}
	return v
}
