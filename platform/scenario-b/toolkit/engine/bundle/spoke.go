package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SpokeBundleVersion is the current spoke bundle schema version.
const SpokeBundleVersion = "cbweb3b/spoke-bundle/v1"

// RequiredSpokeContracts are the spoke contracts expected in the bundle
// (deployed by CBWeb3Spoke.s.sol).
var RequiredSpokeContracts = []string{"identityRegistry", "tCeBM", "spokeBridge", "fCeBM"}

// ValidateSpoke checks required fields and rejects any private-key material.
func ValidateSpoke(b SpokeBundle) error {
	if b.Version == "" {
		return errors.New("spoke bundle: version required")
	}
	if b.SpokeID == "" {
		return errors.New("spoke bundle: spokeId required")
	}
	if b.ChainID == 0 {
		return errors.New("spoke bundle: chainId required")
	}
	if b.Enode == "" {
		return errors.New("spoke bundle: enode required")
	}
	if b.Genesis == "" {
		return errors.New("spoke bundle: genesis required")
	}
	for _, c := range RequiredSpokeContracts {
		if b.Contracts[c] == "" {
			return fmt.Errorf("spoke bundle: contract %q address missing", c)
		}
	}
	if strings.Contains(b.Genesis, "PRIVATE KEY") {
		return errors.New("spoke bundle: genesis must not contain private key material (no-secrets)")
	}
	raw, err := yaml.Marshal(struct {
		Version, SpokeID, Enode, SpokeRPC, SpokeWS string
		Contracts                                  map[string]string
	}{b.Version, b.SpokeID, b.Enode, b.SpokeRPC, b.SpokeWS, b.Contracts})
	if err != nil {
		return err
	}
	if strings.Contains(string(raw), "PRIVATE KEY") {
		return errors.New("spoke bundle: must not contain private key material (no-secrets)")
	}
	return nil
}

// EmitSpoke writes the bundle to <outDir>/bundles/<spokeId>.bundle.yaml (atomic).
// The spoke id is used verbatim (ids are already like "spoke-brl"), so a joining
// bank's joinBundleRef points straight at <spokeId>.bundle.yaml.
func EmitSpoke(b SpokeBundle, outDir string) (string, error) {
	if b.Version == "" {
		b.Version = SpokeBundleVersion
	}
	if err := ValidateSpoke(b); err != nil {
		return "", err
	}
	dir := filepath.Join(outDir, "bundles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, b.SpokeID+".bundle.yaml")
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

// LoadSpoke reads and validates a spoke bundle.
func LoadSpoke(path string) (SpokeBundle, error) {
	var b SpokeBundle
	raw, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	if err := yaml.Unmarshal(raw, &b); err != nil {
		return b, err
	}
	if err := ValidateSpoke(b); err != nil {
		return b, err
	}
	return b, nil
}
