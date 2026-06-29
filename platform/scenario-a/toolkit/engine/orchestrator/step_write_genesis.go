// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

// writeGenesisStep writes the genesis.json from the join bundle into
// SPOKE_DATA_DIR/genesis/genesis.json. It NEVER overwrites an existing genesis
// whose hash diverges from the bundle (genesis-once invariant).
type writeGenesisStep struct {
	dataDir     string
	genesisB64  string // bundle.Spec.Genesis.Content
	genesisHash string // bundle.Spec.Genesis.Hash, format "sha256:<hex>"
}

func newWriteGenesisStep(dataDir, genesisB64, genesisHash string) Step {
	return &writeGenesisStep{dataDir: dataDir, genesisB64: genesisB64, genesisHash: genesisHash}
}

func (s *writeGenesisStep) Name() string { return StepWriteGenesis }

func (s *writeGenesisStep) Check(_ context.Context) (bool, error) {
	path := filepath.Join(s.dataDir, "genesis", "genesis.json")
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	// File exists: it is "done" only if its hash matches the bundle.
	if hashMatches(existing, s.genesisHash) {
		return true, nil
	}
	return false, fmt.Errorf("genesis.json already exists at %s with a hash that diverges from the bundle — refusing to overwrite (genesis-once invariant)", path)
}

func (s *writeGenesisStep) Run(_ context.Context) error {
	content, err := base64.StdEncoding.DecodeString(s.genesisB64)
	if err != nil {
		return fmt.Errorf("decode bundle genesis content (base64): %w", err)
	}
	if !hashMatches(content, s.genesisHash) {
		return fmt.Errorf("bundle genesis content does not match bundle genesis hash %q", s.genesisHash)
	}

	genesisDir := filepath.Join(s.dataDir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		return fmt.Errorf("create genesis dir: %w", err)
	}
	path := filepath.Join(genesisDir, "genesis.json")

	// Guard against overwriting a divergent existing file (race with Check).
	if existing, err := os.ReadFile(path); err == nil && !hashMatches(existing, s.genesisHash) {
		return fmt.Errorf("genesis.json already exists with divergent hash — refusing to overwrite")
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write genesis.json: %w", err)
	}
	return nil
}

// hashMatches reports whether sha256(data) equals the "sha256:<hex>" expected hash.
func hashMatches(data []byte, expected string) bool {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum) == expected
}
