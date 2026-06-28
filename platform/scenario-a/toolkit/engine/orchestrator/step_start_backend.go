// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// startBackendStep brings up the commercial bank's backend services via the
// bank's backend docker-compose file. When no backend compose path is
// configured, the step is a logged no-op (the backend stack is out of scope of
// the join engine and may be started separately).
type startBackendStep struct {
	spokeID            string
	bankID             string
	dataDir            string
	backendComposePath string
	logw               io.Writer
}

func newStartBackendStep(spokeID, bankID, dataDir, backendComposePath string, logw io.Writer) Step {
	return &startBackendStep{spokeID: spokeID, bankID: bankID, dataDir: dataDir, backendComposePath: backendComposePath, logw: logw}
}

func (s *startBackendStep) Name() string { return StepStartBackend }

func (s *startBackendStep) Check(_ context.Context) (bool, error) {
	// No persistent on-disk signal; rely on the orchestrator's persisted state.
	// When no backend compose is configured the step is trivially complete.
	if s.backendComposePath == "" {
		return true, nil
	}
	return false, nil
}

func (s *startBackendStep) Run(ctx context.Context) error {
	if s.backendComposePath == "" {
		logDetail(s.logw, s.spokeID, s.Name(), "no BackendComposePath configured for "+s.bankID+" — skipping")
		return nil
	}
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.backendComposePath, "up", "-d")
	cmd.Env = append(os.Environ(), "BANK_ID="+s.bankID, "SPOKE_DATA_DIR="+s.dataDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up backend: %w\noutput:\n%s", err, out)
	}
	return nil
}
