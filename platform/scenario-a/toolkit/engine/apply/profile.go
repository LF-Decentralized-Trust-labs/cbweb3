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
}

// LocalProfileFromExDir builds a LocalProfile for the given manifest spec.
// exDir is filepath.Dir(os.Executable()).
// dataDir is m.Spec.Node.DataDir.
// rpcPort is m.Spec.Node.RPC.Port.
func LocalProfileFromExDir(exDir, dataDir string, rpcPort int) LocalProfile {
	p := LocalProfile{}

	p.BesuRPCURL = fmt.Sprintf("http://localhost:%d", rpcPort)

	p.PaladinCBURL = envOr("CBWEB3_PALADIN_CB_URL", "http://localhost:31648")

	p.ScriptsDir = envOr("CBWEB3_SCRIPTS_DIR",
		filepath.Join(exDir, "..", "..", "deploy", "local", "paladin", "scripts"))

	p.ComposeTemplatePath = envOr("CBWEB3_COMPOSE_TEMPLATE",
		filepath.Join(exDir, "..", "..", "provisioning", "templates", "central-bank", "paladin-compose.yaml"))

	p.PaladinConfigDir = envOr("CBWEB3_PALADIN_CONFIG_DIR",
		filepath.Join(exDir, "..", "..", "provisioning", "templates", "central-bank", "paladin-config"))

	p.OutputDir = envOr("CBWEB3_OUTPUT_DIR", filepath.Dir(dataDir))

	p.CommercialBankComposePath = envOr("CBWEB3_COMMERCIAL_BANK_COMPOSE",
		filepath.Join(exDir, "..", "..", "provisioning", "templates", "commercial-bank", "docker-compose.yaml"))

	p.BackendComposePath = envOr("CBWEB3_BACKEND_COMPOSE", "")

	p.CentralBankComposePath = envOr("CBWEB3_CENTRAL_BANK_COMPOSE",
		filepath.Join(exDir, "..", "..", "provisioning", "templates", "central-bank", "docker-compose.yaml"))

	p.BesuImage = envOr("CBWEB3_BESU_IMAGE", "hyperledger/besu:25.8.0")

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
