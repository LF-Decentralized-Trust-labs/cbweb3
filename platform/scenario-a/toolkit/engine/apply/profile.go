// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocalProfile holds the resolved runtime defaults for environment: local.
// Fields come from env vars with binary-relative fallbacks.
type LocalProfile struct {
	BesuRPCURL          string
	PaladinCBURL        string
	ScriptsDir          string
	ComposeTemplatePath string
	PaladinConfigDir    string
	OutputDir           string

	// CommercialBankComposePath is the TK-8 commercial-bank Besu compose template
	// (used by mode:join). BackendComposePath is the bank's backend compose file
	// (optional; empty means the start-backend step is a no-op).
	CommercialBankComposePath string
	BackendComposePath        string

	// CentralBankComposePath is the TK-4 central-bank Besu compose template
	// (used by mode:found start-besu). BesuImage is the pinned bootnode image.
	CentralBankComposePath string
	BesuImage              string

	// PaladinImage is the pinned Paladin image for the spoke's Paladin nodes.
	PaladinImage string

	// ContractsOutDir is the Foundry build output dir (contracts/out) read by the
	// engine to deploy the participant whitelist (IdentityRegistry.sol).
	ContractsOutDir string

	// CommercialBankPaladinComposePath is the commercial-bank Paladin compose
	// template (mode:join Paladin node bring-up, feature 033 US2).
	CommercialBankPaladinComposePath string
}

// scenarioMarker is a path, relative to the Scenario A root, that uniquely
// identifies it. Used to locate the root regardless of where the binary lives.
const scenarioMarker = "provisioning/templates/central-bank/docker-compose.yaml"

// scenarioRoot walks up from each start directory looking for the Scenario A
// root (the directory containing scenarioMarker). It returns the first match,
// or "" if none is found within a bounded number of parents.
//
// This makes template/script resolution independent of where the cbweb3 binary
// is placed (e.g. scenario-a/toolkit/cbweb3, scenario-a/samples/cbweb3, or a
// copy elsewhere in the repo). It tries the executable dir first, then the cwd.
func scenarioRoot(startDirs ...string) string {
	for _, start := range startDirs {
		if start == "" {
			continue
		}
		dir := start
		for i := 0; i < 12; i++ {
			if _, err := os.Stat(filepath.Join(dir, scenarioMarker)); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

// LocalProfileFromExDir builds a LocalProfile for the given manifest spec.
// exDir is filepath.Dir(os.Executable()).
// dataDir is m.Spec.Node.DataDir.
// rpcPort is m.Spec.Node.RPC.Port.
//
// Template/script paths default to locations under the Scenario A root, located
// by scenarioRoot (anchor search) so the binary works from anywhere in the repo.
// Each path can still be overridden by its CBWEB3_* environment variable.
func LocalProfileFromExDir(exDir, dataDir string, rpcPort int) LocalProfile {
	p := LocalProfile{}

	// Resolve the Scenario A root with this precedence:
	//   1. CBWEB3_HOME — explicit single knob (use when the binary runs outside the repo);
	//   2. marker search up from the executable dir, then the cwd (zero-config in-repo);
	//   3. historical exDir/../.. assumption (binary in scenario-a/toolkit).
	// Per-path CBWEB3_* overrides below still take precedence over the derived root.
	root := envOr("CBWEB3_HOME", "")
	if root == "" {
		cwd, _ := os.Getwd()
		root = scenarioRoot(exDir, cwd)
	}
	if root == "" {
		root = filepath.Join(exDir, "..", "..")
	}

	p.BesuRPCURL = fmt.Sprintf("http://localhost:%d", rpcPort)

	p.PaladinCBURL = envOr("CBWEB3_PALADIN_CB_URL", "http://localhost:31648")

	p.ScriptsDir = envOr("CBWEB3_SCRIPTS_DIR",
		filepath.Join(root, "deploy", "local", "paladin", "scripts"))

	p.ComposeTemplatePath = envOr("CBWEB3_COMPOSE_TEMPLATE",
		filepath.Join(root, "provisioning", "templates", "central-bank", "paladin-compose.yaml"))

	p.PaladinConfigDir = envOr("CBWEB3_PALADIN_CONFIG_DIR",
		filepath.Join(root, "provisioning", "templates", "central-bank", "paladin-config"))

	p.OutputDir = envOr("CBWEB3_OUTPUT_DIR", filepath.Dir(dataDir))

	p.CommercialBankComposePath = envOr("CBWEB3_COMMERCIAL_BANK_COMPOSE",
		filepath.Join(root, "provisioning", "templates", "commercial-bank", "docker-compose.yaml"))

	p.BackendComposePath = envOr("CBWEB3_BACKEND_COMPOSE", "")

	p.CentralBankComposePath = envOr("CBWEB3_CENTRAL_BANK_COMPOSE",
		filepath.Join(root, "provisioning", "templates", "central-bank", "docker-compose.yaml"))

	p.BesuImage = envOr("CBWEB3_BESU_IMAGE", "hyperledger/besu:25.8.0")

	p.PaladinImage = envOr("CBWEB3_PALADIN_IMAGE", "docker.io/lfdecentralizedtrust/paladin:v0.15.0-rc.1")

	p.ContractsOutDir = envOr("CBWEB3_CONTRACTS_OUT", filepath.Join(root, "contracts", "out"))

	p.CommercialBankPaladinComposePath = envOr("CBWEB3_COMMERCIAL_BANK_PALADIN_COMPOSE",
		filepath.Join(root, "provisioning", "templates", "commercial-bank", "paladin-compose.yaml"))

	return p
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// firstNonEmpty returns the first non-empty string among its arguments, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
