// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os/exec"
	"testing"
)

// requireDocker skips the test if the docker CLI is not reachable — several
// orchestrator steps now read/write named Docker volumes instead of
// SPOKE_DATA_DIR bind mounts (see specs/026-tk4-compose-central-bank/plan.md
// addendum), so their tests exercise real `docker run` against a throwaway
// volume. The read/write/exists mechanism itself is covered directly in
// engine/dockervolume.
func requireDocker(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("docker not available; skipping volume-backed test")
	}
}

func cleanupVolume(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		_ = exec.Command("docker", "volume", "rm", "-f", name).Run()
	})
}
