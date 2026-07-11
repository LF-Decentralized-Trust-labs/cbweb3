package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// genGenesisHubStep generates the hub's QBFT genesis (via
// `besu operator generate-blockchain-config`) and seeds genesis.json into
// GenesisDir, which the hub template mounts. Idempotent: skipped if genesis.json
// already exists. Reproduces deploy/local/hub-besu/startBesu.sh's genesis step.
func genGenesisHubStep(c HubConfig) Step {
	genesisFile := filepath.Join(c.GenesisDir, "genesis.json")
	workDir := filepath.Join(c.GenesisDir, ".work")
	return Step{
		Name: "gen-genesis-hub",
		Check: func(context.Context) (bool, error) {
			_, err := os.Stat(genesisFile)
			return err == nil, nil
		},
		Run: func(ctx context.Context) error {
			if err := os.MkdirAll(workDir, 0o755); err != nil {
				return err
			}
			cfgPath := filepath.Join(workDir, "qbftConfig.json")
			if err := os.WriteFile(cfgPath, qbftConfig(c.ChainID, c.ValidatorCount), 0o644); err != nil {
				return err
			}
			// besu operator generate-blockchain-config → workDir/networkFiles/genesis.json
			if _, err := c.Runner.Run(ctx, "docker", "run", "--rm",
				"-v", workDir+":/work", c.BesuImage,
				"operator", "generate-blockchain-config",
				"--config-file=/work/qbftConfig.json", "--to=/work/networkFiles"); err != nil {
				return err
			}
			if err := os.MkdirAll(c.GenesisDir, 0o755); err != nil {
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
