// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// genGenesisHubStep generates the hub's QBFT genesis and seeds it (plus the
// validator node key) into the hub's named volumes. Thin wrapper over the
// generalized genGenesisStep (used by both hub and spoke).
func genGenesisHubStep(c HubConfig) Step {
	return genGenesisStep("gen-genesis-hub", c.ChainID, c.genesisVolume(), c.besuDataVolume(), c.ValidatorCount, c.Runner, c.BesuImage)
}

// genGenesisStep generates a QBFT genesis (via `besu operator
// generate-blockchain-config`) and seeds genesis.json into the `<prefix>_genesis`
// named volume and the validator node key into `<prefix>_besu_data/key` — NO node
// state on the host (roadmap §7). Idempotent: skipped if genesis.json already
// exists in the volume. Reused by hub and spoke (parametrized by name/chainID/
// volumes). Zero-gas: all forks at block 0 + zeroBaseFee, matching the reference
// network so contracts deploy with gasPrice 0.
func genGenesisStep(name string, chainID uint64, genesisVolume, besuDataVolume string, validators int, runner exec.CommandRunner, image string) Step {
	if validators < 1 {
		validators = 1
	}
	if image == "" {
		image = "hyperledger/besu:25.8.0"
	}
	return Step{
		Name: name,
		Check: func(ctx context.Context) (bool, error) {
			return volumeHasFile(ctx, runner, genesisVolume, "genesis.json"), nil
		},
		Run: func(ctx context.Context) error {
			// Ephemeral host scratch for the besu operator output; removed after
			// the artifacts are copied into the named volumes.
			workDir, err := os.MkdirTemp("", "cbweb3b-genesis-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(workDir)

			cfg, err := qbftConfig(chainID, validators)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(workDir, "qbftConfig.json"), cfg, 0o644); err != nil {
				return err
			}

			// Run besu as the host user: as root the image's besu-entry.sh runs
			// besu twice (a --print-paths-and-exit permission pass) and the first
			// run already creates --to, so the real run fails "Output directory
			// already exists"; non-root runs it once and the files are host-owned.
			runArgs := []string{"run", "--rm"}
			if u := dockerUserArg(); u != "" {
				runArgs = append(runArgs, "--user", u)
			}
			runArgs = append(runArgs, "-v", workDir+":/work", image,
				"operator", "generate-blockchain-config",
				"--config-file=/work/qbftConfig.json", "--to=/work/networkFiles",
				"--private-key-file-name=key")
			if _, err := runner.Run(ctx, "docker", runArgs...); err != nil {
				return err
			}

			// Seed genesis.json into the genesis volume.
			if err := copyHostFileToVolume(ctx, runner, workDir, "networkFiles/genesis.json", genesisVolume, "genesis.json", "0644"); err != nil {
				return err
			}
			// Seed the (single) validator node key into <besu_data>/key, matching
			// besu's default node-key location under --data-path.
			keyRel, err := singleKeyRelPath(workDir)
			if err != nil {
				return err
			}
			return copyHostFileToVolume(ctx, runner, workDir, keyRel, besuDataVolume, "key", "0600")
		},
	}
}

// singleKeyRelPath returns the relative path (under workDir) of the single node
// key produced by `--private-key-file-name=key` (networkFiles/keys/<addr>/key).
func singleKeyRelPath(workDir string) (string, error) {
	keysDir := filepath.Join(workDir, "networkFiles", "keys")
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return "", fmt.Errorf("read keys dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			return filepath.Join("networkFiles", "keys", e.Name(), "key"), nil
		}
	}
	return "", fmt.Errorf("no node key generated under %s", keysDir)
}

// qbftConfig builds the besu operator config for a zero-gas QBFT network: all
// forks at block 0 + shanghaiTime/cancunTime (so modern Solidity/PUSH0 deploys)
// + zeroBaseFee (gasPrice 0). No dev wallets are pre-funded in genesis — the
// network is zero-gas, so any account (deployer, CB, and each commercial bank's
// runtime-generated key) transacts without a balance. This mirrors scenario-a,
// whose genesis carries no pre-funded accounts and lets each entity create its
// wallet at runtime. The single 0x…01 alloc is a conventional placeholder.
func qbftConfig(chainID uint64, validators int) ([]byte, error) {
	if validators < 1 {
		validators = 1
	}
	alloc := map[string]any{
		"0x0000000000000000000000000000000000000001": map[string]any{"balance": "0x0"},
	}
	cfg := map[string]any{
		"genesis": map[string]any{
			"config": map[string]any{
				"chainId":             chainID,
				"homesteadBlock":      0,
				"eip150Block":         0,
				"eip155Block":         0,
				"eip158Block":         0,
				"byzantiumBlock":      0,
				"constantinopleBlock": 0,
				"petersburgBlock":     0,
				"istanbulBlock":       0,
				"berlinBlock":         0,
				"londonBlock":         0,
				"preMergeForkBlock":   0,
				"shanghaiTime":        0,
				"cancunTime":          0,
				"qbft": map[string]any{
					"blockperiodseconds":    2,
					"epochlength":           30000,
					"requesttimeoutseconds": 4,
				},
				"zeroBaseFee": true,
			},
			"nonce":      "0x0",
			"timestamp":  "0x58ee40ba",
			"gasLimit":   "0x1c9c380",
			"difficulty": "0x1",
			"mixHash":    "0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365",
			"coinbase":   "0x0000000000000000000000000000000000000000",
			"alloc":      alloc,
		},
		"blockchain": map[string]any{
			"nodes": map[string]any{"generate": true, "count": validators},
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// dockerUserArg returns "<uid>:<gid>" for `docker run --user` on Unix, or "" on
// platforms without POSIX uids where it does not apply.
func dockerUserArg() string {
	uid, gid := os.Getuid(), os.Getgid()
	if uid < 0 || gid < 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", uid, gid)
}
