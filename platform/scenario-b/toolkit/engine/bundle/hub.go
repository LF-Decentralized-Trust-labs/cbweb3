package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// HubBundleVersion is the current hub bundle schema version.
const HubBundleVersion = "cbweb3b/hub-bundle/v1"

// RequiredContracts are the hub contracts that must be present in the bundle
// (deployed by CBWeb3Hub.s.sol). tCeBM_* tokens are optional — they are
// deployed later when each CB registers a currency, not at found-hub.
var RequiredContracts = []string{
	"identityRegistry",
	"fxAgreement",
	"pairRegistry",
	"currencyRegistry",
	"manualOracle",
}

// ValidateHub checks required fields and rejects any private-key material.
func ValidateHub(b HubBundle) error {
	if b.Version == "" {
		return errors.New("hub bundle: version required")
	}
	if b.ChainID == 0 {
		return errors.New("hub bundle: chainId required")
	}
	if b.HubRPC == "" {
		return errors.New("hub bundle: hubRpc required")
	}
	for _, c := range RequiredContracts {
		if addr := b.Contracts[c]; addr == "" {
			return fmt.Errorf("hub bundle: contract %q address missing", c)
		}
	}
	raw, err := yaml.Marshal(b)
	if err != nil {
		return err
	}
	if strings.Contains(string(raw), "PRIVATE KEY") {
		return errors.New("hub bundle: must not contain private key material (no-secrets)")
	}
	return nil
}

// EmitHub writes the bundle to <outDir>/bundles/hub.bundle.yaml (atomic).
func EmitHub(b HubBundle, outDir string) (string, error) {
	if b.Version == "" {
		b.Version = HubBundleVersion
	}
	if err := ValidateHub(b); err != nil {
		return "", err
	}
	dir := filepath.Join(outDir, "bundles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "hub.bundle.yaml")
	raw, err := yaml.Marshal(b)
	if err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		return "", err
	}
	return p, nil
}

// LoadHub reads and validates a hub bundle.
func LoadHub(path string) (HubBundle, error) {
	var b HubBundle
	raw, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	if err := yaml.Unmarshal(raw, &b); err != nil {
		return b, err
	}
	if err := ValidateHub(b); err != nil {
		return b, err
	}
	return b, nil
}
