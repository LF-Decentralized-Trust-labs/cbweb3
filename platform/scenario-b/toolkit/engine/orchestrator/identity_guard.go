// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// pkiMountDestination is where every entity template binds its PKI directory inside the container.
const pkiMountDestination = "/workspace/backend/config/pki"

// identityDirConflict refuses to mint a second identity for an entity that is already running from a
// different directory.
//
// A manifest's node.dataDir is relative, so it resolves against the working directory of whoever runs
// apply. Run it from elsewhere and the entity silently gets a fresh data dir: gen-csr's idempotency
// check is "does the key file exist here?", finds nothing, and generates a new keypair; compose then
// rebinds the running gateway to that directory. The bank now signs with a key its central bank never
// certified — every internal call fails to verify — while the certified key sits unused in the original
// directory. No step fails, so the first symptom is a broken bank.
//
// boundPKIDir is what the running container actually binds; empty means nothing is running yet, which
// is a first join and must proceed.
func identityDirConflict(entityID, resolvedPKIDir, boundPKIDir string) error {
	if strings.TrimSpace(boundPKIDir) == "" {
		return nil
	}
	resolved, err := filepath.Abs(resolvedPKIDir)
	if err != nil {
		return fmt.Errorf("resolve pki dir %q: %w", resolvedPKIDir, err)
	}
	bound, err := filepath.Abs(strings.TrimSpace(boundPKIDir))
	if err != nil {
		return fmt.Errorf("resolve bound pki dir %q: %w", boundPKIDir, err)
	}
	if resolved == bound {
		return nil
	}
	return fmt.Errorf(
		"%s is already provisioned from a different directory — refusing to generate a second identity.\n"+
			"  this run resolved:      %s\n"+
			"  the running gateway is: %s\n"+
			"node.dataDir is relative, so it follows the working directory. Re-run from the directory the\n"+
			"entity was deployed from, or give node.dataDir an absolute path. Generating a key here would\n"+
			"replace the identity the central bank certified, and every internal call would stop verifying.",
		entityID, resolved, bound)
}

// parsePKIMountSource picks the host path bound at pkiMountDestination out of the inspect output.
//
// The format asked for is one "source\x00destination" pair per line: a NUL separator cannot occur in a
// path, so a path containing spaces or colons still parses.
func parsePKIMountSource(inspectOutput []byte) string {
	for _, line := range bytes.Split(inspectOutput, []byte("\n")) {
		source, destination, found := bytes.Cut(bytes.TrimSpace(line), []byte("\x00"))
		if !found {
			continue
		}
		if string(destination) == pkiMountDestination {
			return string(source)
		}
	}
	return ""
}

// boundPKIDir reports the host directory the entity's api-gateway currently binds as its PKI dir.
//
// It returns ("", nil) only when docker answered and there is nothing to bind — no such container, or
// a container with no PKI mount. When docker could not be asked at all it returns an error, and the
// caller must NOT read that as "nothing is running".
//
// That distinction is the whole point. This used to return "" for both, and "" allows the run to
// proceed, so any docker failure — daemon down, unreachable DOCKER_HOST, permission denied — silently
// disabled the guard. It failed open exactly where it was needed most: on a remote or multi-host
// daemon, the container this run would duplicate is on ANOTHER machine, and the only way to see it is
// the call that just failed. A single-host operator loses nothing by failing closed here, because
// apply shells out to compose moments later and would fail anyway.
func boundPKIDir(ctx context.Context, r exec.CommandRunner, composeProject string) (string, error) {
	names, err := r.Run(ctx, "docker", "ps", "-a",
		"--filter", "label=com.docker.compose.project="+composeProject,
		"--filter", "label=com.docker.compose.service=api-gateway",
		"--format", "{{.Names}}")
	if err != nil {
		return "", fmt.Errorf("docker ps: %w", err)
	}
	name := strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(names)), "\n", 2)[0])
	if name == "" {
		// docker answered: this entity has no gateway container. A first join, and it must proceed.
		return "", nil
	}
	mounts, err := r.Run(ctx, "docker", "inspect", name,
		"--format", `{{range .Mounts}}{{.Source}}{{printf "\x00"}}{{.Destination}}{{println}}{{end}}`)
	if err != nil {
		// The container was listed a moment ago but cannot be read now. Still "cannot tell", not
		// "nothing is running".
		return "", fmt.Errorf("docker inspect %s: %w", name, err)
	}
	return parsePKIMountSource(mounts), nil
}

// ensureIdentityDirUnchanged is the guard as used by a step.
func ensureIdentityDirUnchanged(ctx context.Context, r exec.CommandRunner, composeProject, entityID, resolvedPKIDir string) error {
	bound, err := boundPKIDir(ctx, r, composeProject)
	if err != nil {
		return fmt.Errorf(
			"%s: cannot verify which directory this entity is already provisioned from — refusing to\n"+
				"generate an identity.\n"+
				"  %v\n"+
				"This guard exists to refuse a SECOND identity for an entity that is already running from\n"+
				"another directory. Treating an unanswerable docker as \"nothing is running\" would disable it\n"+
				"precisely where it matters: with a remote or multi-host daemon, the container this run would\n"+
				"duplicate is on another machine, and this is the call that would have found it.\n"+
				"Restore access to the docker daemon and re-run.",
			entityID, err)
	}
	return identityDirConflict(entityID, resolvedPKIDir, bound)
}
