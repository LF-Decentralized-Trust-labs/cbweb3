// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// writeGenesisStep writes the genesis.json from the join bundle into the named
// volume ${spokeID}_${bankID}_genesis, mounted by the commercial-bank compose
// template's besu service at /opt/besu/genesis (no host filesystem involved —
// deviation from the original SPOKE_DATA_DIR bind-mount design; see
// specs/032-commercial-bank-join/research.md addendum). It NEVER overwrites an
// existing genesis whose hash diverges from the bundle (genesis-once invariant).
type writeGenesisStep struct {
	spokeID     string
	bankID      string
	genesisB64  string // bundle.Spec.Genesis.Content
	genesisHash string // bundle.Spec.Genesis.Hash, format "sha256:<hex>"
}

func newWriteGenesisStep(spokeID, bankID, genesisB64, genesisHash string) Step {
	return &writeGenesisStep{spokeID: spokeID, bankID: bankID, genesisB64: genesisB64, genesisHash: genesisHash}
}

func (s *writeGenesisStep) Name() string { return StepWriteGenesis }

// genesisVolume mirrors the besu_data naming convention (${SPOKE_ID}_${BANK_ID}_besu_data).
func (s *writeGenesisStep) genesisVolume() string { return s.spokeID + "_" + s.bankID + "_genesis" }

func (s *writeGenesisStep) Check(ctx context.Context) (bool, error) {
	existing, err := readVolumeFile(ctx, s.genesisVolume(), "genesis.json")
	if err != nil {
		if errors.Is(err, ErrVolumeFileNotFound) {
			return false, nil
		}
		return false, err
	}
	// File exists: it is "done" only if its hash matches the bundle.
	if hashMatches(existing, s.genesisHash) {
		return true, nil
	}
	return false, fmt.Errorf("genesis.json already exists in volume %s with a hash that diverges from the bundle — refusing to overwrite (genesis-once invariant)", s.genesisVolume())
}

func (s *writeGenesisStep) Run(ctx context.Context) error {
	content, err := base64.StdEncoding.DecodeString(s.genesisB64)
	if err != nil {
		return fmt.Errorf("decode bundle genesis content (base64): %w", err)
	}
	if !hashMatches(content, s.genesisHash) {
		return fmt.Errorf("bundle genesis content does not match bundle genesis hash %q", s.genesisHash)
	}

	// Guard against overwriting a divergent existing file (race with Check).
	if existing, err := readVolumeFile(ctx, s.genesisVolume(), "genesis.json"); err == nil && !hashMatches(existing, s.genesisHash) {
		return fmt.Errorf("genesis.json already exists in volume %s with divergent hash — refusing to overwrite", s.genesisVolume())
	} else if err != nil && !errors.Is(err, ErrVolumeFileNotFound) {
		return fmt.Errorf("check existing genesis.json in volume %s: %w", s.genesisVolume(), err)
	}

	if err := writeVolumeFile(ctx, s.genesisVolume(), "genesis.json", content, "0644"); err != nil {
		return fmt.Errorf("write genesis.json to volume %s: %w", s.genesisVolume(), err)
	}
	return nil
}

// hashMatches reports whether sha256(data) equals the "sha256:<hex>" expected hash.
func hashMatches(data []byte, expected string) bool {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum) == expected
}
