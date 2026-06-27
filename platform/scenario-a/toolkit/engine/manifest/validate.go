// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads the YAML file at path, parses it into a *Manifest, and returns it.
// It does not validate field values or required-field presence — use Validate for that.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", path, err)
	}
	return &m, nil
}

// Validate checks all required fields and enum constraints of m.
// It collects all violations before returning so the operator never needs
// to re-run to discover additional errors.
// Returns nil if the manifest is valid.
func Validate(m *Manifest) error {
	if m == nil {
		return errors.New("manifest: nil pointer")
	}

	var errs []error

	// apiVersion
	if m.APIVersion == "" {
		errs = append(errs, errors.New("apiVersion: required field is missing"))
	} else if m.APIVersion != "cbweb3/v1" {
		errs = append(errs, fmt.Errorf("apiVersion: invalid value %q; accepted values are: cbweb3/v1", m.APIVersion))
	}

	// kind
	if m.Kind == "" {
		errs = append(errs, errors.New("kind: required field is missing"))
	} else if m.Kind != "ParticipantDeployment" {
		errs = append(errs, fmt.Errorf("kind: invalid value %q; accepted values are: ParticipantDeployment", m.Kind))
	}

	// metadata.name
	if m.Metadata.Name == "" {
		errs = append(errs, errors.New("metadata.name: required field is missing"))
	}

	// spec.scenario
	if m.Spec.Scenario == "" {
		errs = append(errs, errors.New("spec.scenario: required field is missing"))
	} else if m.Spec.Scenario != "a" {
		errs = append(errs, fmt.Errorf("spec.scenario: invalid value %q; accepted values are: a (Scenario B is out of scope for this toolkit)", m.Spec.Scenario))
	}

	// spec.role
	if m.Spec.Role == "" {
		errs = append(errs, errors.New("spec.role: required field is missing"))
	} else {
		switch m.Spec.Role {
		case "central-bank", "commercial-bank":
		default:
			errs = append(errs, fmt.Errorf("spec.role: invalid value %q; accepted values are: central-bank, commercial-bank", m.Spec.Role))
		}
	}

	// spec.mode
	if m.Spec.Mode == "" {
		errs = append(errs, errors.New("spec.mode: required field is missing"))
	} else {
		switch m.Spec.Mode {
		case "found", "join":
		default:
			errs = append(errs, fmt.Errorf("spec.mode: invalid value %q; accepted values are: found, join", m.Spec.Mode))
		}
	}

	// spec.environment (optional, but constrained when present)
	if m.Spec.Environment != "" {
		switch m.Spec.Environment {
		case "local", "staging", "prod":
		default:
			errs = append(errs, fmt.Errorf("spec.environment: invalid value %q; accepted values are: local, staging, prod", m.Spec.Environment))
		}
	}

	// spec.spoke
	if m.Spec.Spoke.ID == "" {
		errs = append(errs, errors.New("spec.spoke.id: required field is missing"))
	}
	if m.Spec.Spoke.ChainID == 0 {
		errs = append(errs, errors.New("spec.spoke.chainId: required field is missing"))
	}
	if m.Spec.Spoke.Currency == "" {
		errs = append(errs, errors.New("spec.spoke.currency: required field is missing"))
	}

	// spec.node.advertisedHost — special message per FR-003
	if m.Spec.Node.AdvertisedHost == "" {
		errs = append(errs, errors.New(
			"spec.node.advertisedHost: required field is missing; "+
				"this field must be set explicitly and is never inferred from co-location",
		))
	}

	// spec.image
	if m.Spec.Image == "" {
		errs = append(errs, errors.New("spec.image: required field is missing"))
	}

	// spec.keyProvider
	if m.Spec.KeyProvider == "" {
		errs = append(errs, errors.New("spec.keyProvider: required field is missing"))
	}

	// spec.certSource
	if m.Spec.CertSource == "" {
		errs = append(errs, errors.New("spec.certSource: required field is missing"))
	}

	return errors.Join(errs...)
}
