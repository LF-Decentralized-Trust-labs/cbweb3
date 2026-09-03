// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// NOCBundleVersion is the current NOC bundle schema version.
const NOCBundleVersion = "cbweb3/noc-bundle/v1"

// NOCComponentTypes are the component types the NOC agent knows how to probe.
// (Scenario A monitors Besu + Paladin nodes and the Cacti relay.)
var NOCComponentTypes = []string{"BESU", "PALADIN", "CACTI_RELAY"}

// nocUUIDRe matches the canonical 8-4-4-4-12 hex UUID form. The NOC backend does
// a strict uuid.Parse on the registered/pushed spoke id, so the bundle rejects a
// SpokeUUID that would fail there.
var nocUUIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// NOCComponent describes a single monitorable component (a Besu/Paladin node, a
// Cacti relay) that the NOC agent probes. Public addressing only — never keys.
type NOCComponent struct {
	Name          string `yaml:"name"`
	Type          string `yaml:"type"` // BESU | PALADIN | CACTI_RELAY
	Endpoint      string `yaml:"endpoint"`
	ContainerName string `yaml:"containerName,omitempty"`
}

// NOCBundle is the versioned public artifact describing a spoke's monitoring
// topology. Emitted by a central bank's found and consumed by an observe-mode
// NOC deployment to register the spoke and drive its agent. No secrets.
type NOCBundle struct {
	Version      string         `yaml:"version"`
	SpokeID      string         `yaml:"spokeId"`
	SpokeUUID    string         `yaml:"spokeUuid"`
	Name         string         `yaml:"name"`
	CurrencyCode string         `yaml:"currencyCode"`
	Jurisdiction string         `yaml:"jurisdiction"`
	Components   []NOCComponent `yaml:"components"`
}

// ValidateNOC checks required fields and rejects any private-key material.
func ValidateNOC(b NOCBundle) error {
	if b.Version == "" {
		return errors.New("noc bundle: version required")
	}
	if b.SpokeID == "" {
		return errors.New("noc bundle: spokeId required")
	}
	if b.SpokeUUID == "" {
		return errors.New("noc bundle: spokeUuid required")
	}
	if !nocUUIDRe.MatchString(b.SpokeUUID) {
		return fmt.Errorf("noc bundle: spokeUuid %q is not a canonical UUID", b.SpokeUUID)
	}
	if b.Name == "" {
		return errors.New("noc bundle: name required")
	}
	if b.CurrencyCode == "" {
		return errors.New("noc bundle: currencyCode required")
	}
	if b.Jurisdiction == "" {
		return errors.New("noc bundle: jurisdiction required")
	}
	if len(b.Components) == 0 {
		return errors.New("noc bundle: at least one component required")
	}
	for i, c := range b.Components {
		if c.Name == "" {
			return fmt.Errorf("noc bundle: component[%d].name required", i)
		}
		if !containsNOC(NOCComponentTypes, c.Type) {
			return fmt.Errorf("noc bundle: component[%d] %q has invalid type %q; must be one of %s",
				i, c.Name, c.Type, strings.Join(NOCComponentTypes, ", "))
		}
		if c.Endpoint == "" {
			return fmt.Errorf("noc bundle: component[%d] %q endpoint required", i, c.Name)
		}
	}
	raw, err := yaml.Marshal(b)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToUpper(string(raw)), "PRIVATE KEY") {
		return errors.New("noc bundle: must not contain private key material (no-secrets)")
	}
	return nil
}

// EmitNOC writes the bundle to <outDir>/bundles/<spokeId>.noc.bundle.yaml
// (atomic). The spoke id is used verbatim, so an observe deployment's
// nocBundleRef points straight at <spokeId>.noc.bundle.yaml.
func EmitNOC(b NOCBundle, outDir string) (string, error) {
	if b.Version == "" {
		b.Version = NOCBundleVersion
	}
	if err := ValidateNOC(b); err != nil {
		return "", err
	}
	dir := filepath.Join(outDir, "bundles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, b.SpokeID+".noc.bundle.yaml")
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

// LoadNOC reads and validates a NOC bundle.
func LoadNOC(path string) (NOCBundle, error) {
	var b NOCBundle
	raw, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	if err := yaml.Unmarshal(raw, &b); err != nil {
		return b, err
	}
	if err := ValidateNOC(b); err != nil {
		return b, err
	}
	return b, nil
}

func containsNOC(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
