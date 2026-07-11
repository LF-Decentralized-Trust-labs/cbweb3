package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// genGenesisHubStep generates the hub's QBFT genesis. Thin wrapper over the
// generalized genGenesisStep (used by both hub and spoke).
func genGenesisHubStep(c HubConfig) Step {
	return genGenesisStep("gen-genesis-hub", c.ChainID, c.GenesisDir, c.ValidatorCount, c.Runner, c.BesuImage)
}

// genGenesisStep generates a QBFT genesis (via `besu operator
// generate-blockchain-config`) and seeds genesis.json into genesisDir, which the
// node template mounts. Idempotent: skipped if genesis.json already exists.
// Reproduces the genesis step of deploy/local/*-besu/startBesu.sh. Reused by hub
// and spoke (parametrized by name/chainID/genesisDir).
func genGenesisStep(name string, chainID uint64, genesisDir string, validators int, runner exec.CommandRunner, image string) Step {
	if validators < 1 {
		validators = 1
	}
	if image == "" {
		image = "hyperledger/besu:25.8.0"
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	workDir := filepath.Join(genesisDir, ".work")
	return Step{
		Name: name,
		Check: func(context.Context) (bool, error) {
			_, err := os.Stat(genesisFile)
			return err == nil, nil
		},
		Run: func(ctx context.Context) error {
			if err := os.MkdirAll(workDir, 0o755); err != nil {
				return err
			}
			cfgPath := filepath.Join(workDir, "qbftConfig.json")
			if err := os.WriteFile(cfgPath, qbftConfig(chainID, validators), 0o644); err != nil {
				return err
			}
			if _, err := runner.Run(ctx, "docker", "run", "--rm",
				"-v", workDir+":/work", image,
				"operator", "generate-blockchain-config",
				"--config-file=/work/qbftConfig.json", "--to=/work/networkFiles"); err != nil {
				return err
			}
			if err := os.MkdirAll(genesisDir, 0o755); err != nil {
				return err
			}
			return copyFile(filepath.Join(workDir, "networkFiles", "genesis.json"), genesisFile)
		},
	}
}

// qbftConfig returns a minimal besu operator config for a QBFT network.
func qbftConfig(chainID uint64, validators int) []byte {
	if validators < 1 {
		validators = 1
	}
	return []byte(fmt.Sprintf(`{
  "genesis": {
    "config": {
      "chainId": %d,
      "berlinBlock": 0,
      "qbft": { "blockperiodseconds": 2, "epochlength": 30000, "requesttimeoutseconds": 4 }
    },
    "nonce": "0x0",
    "gasLimit": "0x1fffffffffffff",
    "difficulty": "0x1",
    "mixHash": "0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365",
    "coinbase": "0x0000000000000000000000000000000000000000",
    "alloc": {}
  },
  "blockchain": { "nodes": { "generate": true, "count": %d } }
}`, chainID, validators))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
