// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// Node state (genesis, besu data, keys) lives in Docker NAMED VOLUMES, never on
// the host FS (roadmap §7). These helpers read/seed files inside a named volume
// through the injectable CommandRunner (a throwaway `alpine` container), so they
// are testable with a FakeRunner and neutralized by the DryRunner in --dry-run.
// The only bind mount kept on the host is the bank pki/ dir (onboarding/KYC).

// volHelperImage is the small throwaway image used to touch named-volume files.
//
// This is the single source of truth for the helper image across this toolkit —
// every call site below derives from it rather than repeating the literal, so a
// version bump is one edit here. Unexported deliberately: nothing outside this
// package needs the value. The version is pinned in docs/TOOLCHAIN.md and gated
// by tools/check-alpine-version.sh.
const volHelperImage = "alpine:3.23"

// volumeOwner is the uid:gid seeded files are given.
//
// The helper below must run as root to write into a freshly created named volume, so
// without this everything it seeds is root-owned. That was harmless while every consumer
// ran as root, and it stopped being harmless when the backend service images became
// non-root (finding R2-M-12): a 0600 private key owned by root is unreadable to the
// service that needs it, and a root-owned directory is unwritable, which surfaced as
// `WARN: PKI bootstrap failed: ... permission denied` rather than as a hard failure.
//
// The invoking user is the right owner because that is the uid those containers are given
// (ENTITY_RUN_UID, the pattern ADR-001 established). Containers that still run as root are
// unaffected — root ignores ownership.
func volumeOwner() string {
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}

// volumeHasFile reports whether filePath exists inside the named volume. It runs
// through the runner so --dry-run (DryRunner returns empty) reports "absent" →
// the step is planned, and no docker is spawned during planning.
func volumeHasFile(ctx context.Context, r exec.CommandRunner, volume, filePath string) bool {
	out, _ := r.Run(ctx, "docker", "run", "--rm", "-v", volume+":/t:ro",
		volHelperImage, "sh", "-c", "test -f /t/"+filePath+" && echo YES")
	return strings.Contains(string(out), "YES")
}

// copyHostFileToVolume copies srcHostDir/<srcRel> (a temp scratch produced by a
// generator container) into <volume>/<dstRel>, chmod'ing to mode. Both the host
// scratch and the volume are mounted into a throwaway container; the host
// scratch is ephemeral (MkdirTemp, removed by the caller) — no node state
// persists on the host.
func copyHostFileToVolume(ctx context.Context, r exec.CommandRunner, srcHostDir, srcRel, volume, dstRel, mode string) error {
	if mode == "" {
		mode = "0644"
	}
	// No --user here: the named volume is freshly created and root-owned, so the
	// helper must run as root to write into it (the source host scratch is mounted
	// read-only and is readable by root). The seeded file and its directory are then
	// chowned to volumeOwner() — Besu chowns its own mounts at start, but the backend
	// services do not, and they no longer run as root. NOT recursive: several callers seed
	// at the volume root, so -R would walk whatever else that volume holds and rewrite
	// ownership no one asked about. Each seeded file gets chowned by its own call, which
	// covers the same set.
	dir := "\"$(dirname /dst/" + dstRel + ")\""
	script := "mkdir -p " + dir + " && cp /src/" + srcRel + " /dst/" + dstRel +
		" && chmod " + mode + " /dst/" + dstRel +
		" && chown " + volumeOwner() + " " + dir + " /dst/" + dstRel
	_, err := r.Run(ctx, "docker", "run", "--rm",
		"-v", srcHostDir+":/src:ro", "-v", volume+":/dst",
		volHelperImage, "sh", "-c", script)
	return err
}

// writeVolumeFile seeds content into <volume>/<filePath> straight from memory —
// no host file. The Runner has no stdin, so the content is base64-embedded in
// the helper's shell command and decoded inside a throwaway container. This
// keeps node state (e.g. a joining bank's genesis) in the named volume only,
// honoring the file-level invariant (nothing on the host but the bank pki/ dir).
// Testable with a FakeRunner; neutralized by the DryRunner in --dry-run.
func writeVolumeFile(ctx context.Context, r exec.CommandRunner, volume, filePath string, content []byte, mode string) error {
	if mode == "" {
		mode = "0644"
	}
	b64 := base64.StdEncoding.EncodeToString(content)
	dir := "\"$(dirname /t/" + filePath + ")\""
	script := "mkdir -p " + dir + " && printf %s '" + b64 + "' | base64 -d > /t/" + filePath +
		" && chmod " + mode + " /t/" + filePath +
		" && chown " + volumeOwner() + " " + dir + " /t/" + filePath
	_, err := r.Run(ctx, "docker", "run", "--rm", "-v", volume+":/t", volHelperImage, "sh", "-c", script)
	return err
}

// readVolumeFile returns the contents of filePath inside the named volume.
func readVolumeFile(ctx context.Context, r exec.CommandRunner, volume, filePath string) ([]byte, error) {
	return r.Run(ctx, "docker", "run", "--rm", "-v", volume+":/t:ro",
		volHelperImage, "cat", "/t/"+filePath)
}
