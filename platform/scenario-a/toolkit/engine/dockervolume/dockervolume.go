// SPDX-License-Identifier: Apache-2.0

// Package dockervolume reads and writes files inside named Docker volumes
// without ever touching the host filesystem. It backs the artifacts that
// migrated off SPOKE_DATA_DIR bind mounts (config, genesis, keycloak realms —
// see specs/026-tk4-compose-central-bank/plan.md and
// specs/032-commercial-bank-join/research.md addenda): content is piped
// directly from Go memory into a throwaway container's stdin, so secrets never
// land on the host disk even transiently.
//
// Each call is an independent, stateless `docker run --rm` invocation — no
// auxiliary container is kept alive across calls, so a crash or interruption
// never leaks a helper container that needs manual cleanup.
package dockervolume

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strings"
)

// HelperImage is the throwaway image used to read/write files inside a named
// Docker volume. Pinned and small; already pulled by besu-data-init /
// paladin-data-init in the compose templates, so this adds no new image.
const HelperImage = "alpine:3.20"

// ErrNotFound is returned by ReadFile when path does not exist inside the volume.
var ErrNotFound = errors.New("dockervolume: file not found in volume")

// WriteFile writes content to path inside the named Docker volume, creating
// parent directories as needed. If the volume does not exist yet, Docker
// creates it empty as a side effect of the mount (standard
// `docker run -v name:/path` behavior) — no separate "docker volume create"
// is needed.
//
// mode is a chmod octal string (e.g. "0644", "0600"); empty defaults to "0644".
func WriteFile(ctx context.Context, volume, filePath string, content []byte, mode string) error {
	if mode == "" {
		mode = "0644"
	}
	target := path.Join("/target", filePath)
	script := fmt.Sprintf("mkdir -p %q && cat > %q && chmod %s %q", path.Dir(target), target, mode, target)

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i",
		"-v", volume+":/target",
		HelperImage, "sh", "-c", script)
	cmd.Stdin = bytes.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write %s into volume %s: %w\nstderr:\n%s", filePath, volume, err, stderr.String())
	}
	return nil
}

// ReadFile reads path from inside the named Docker volume. Returns ErrNotFound
// (wrapped) if the file does not exist.
func ReadFile(ctx context.Context, volume, filePath string) ([]byte, error) {
	target := path.Join("/target", filePath)
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", volume+":/target:ro",
		HelperImage, "cat", target)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "No such file or directory") {
			return nil, fmt.Errorf("%w: %s in volume %s", ErrNotFound, filePath, volume)
		}
		return nil, fmt.Errorf("read %s from volume %s: %w\nstderr:\n%s", filePath, volume, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// FileExists reports whether path exists inside the named Docker volume. A
// nonexistent volume is treated as "file does not exist" (false, nil), not an
// error — Docker auto-creates an empty volume as a side effect of this check,
// which is harmless: the real seed step creates the same named volume anyway.
func FileExists(ctx context.Context, volume, filePath string) (bool, error) {
	target := path.Join("/target", filePath)
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", volume+":/target:ro",
		HelperImage, "test", "-f", target)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, fmt.Errorf("check %s in volume %s: %w", filePath, volume, err)
}
