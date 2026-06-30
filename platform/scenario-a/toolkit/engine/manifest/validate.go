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

	// spec.node.dataDir — required for both modes; the engine uses this as SPOKE_DATA_DIR
	// (genesis, TLS, provisioning state, lock). mode:join also relies on it.
	if (m.Spec.Mode == "found" || m.Spec.Mode == "join") && m.Spec.Node.DataDir == "" {
		errs = append(errs, fmt.Errorf(
			"spec.node.dataDir: required field is missing for mode:%s; "+
				"set it to the absolute path where the provisioning engine will store spoke runtime data",
			m.Spec.Mode,
		))
	}

	// spec.joinBundleRef — required when mode is "join"; the bundle drives the join flow.
	if m.Spec.Mode == "join" && m.Spec.JoinBundleRef == "" {
		errs = append(errs, errors.New(
			"spec.joinBundleRef: required field is missing for mode:join; " +
				"set it to the path of the join bundle emitted by the founding central bank (TK-6)",
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

	// spec.adminUsers — mandatory per-role operator accounts (portal login).
	errs = append(errs, validateAdminUsers(m)...)

	return errors.Join(errs...)
}

// requiredAdminRolesByEntity lists the Keycloak realm roles an entity must
// provision an admin user for, keyed on spec.role. It mirrors the realms/clients
// the engine provisions: a central bank hosts governance + treasury + the shared
// NOC realm; a commercial bank hosts its bank realm.
var requiredAdminRolesByEntity = map[string][]string{
	"central-bank":    {"ROLE_GOVERNANCE", "ROLE_TREASURY", "ROLE_NOC_ADMIN"},
	"commercial-bank": {"ROLE_BANK"},
}

// validateAdminUsers enforces that spec.adminUsers is present, every entry has
// role/username/password, and there is exactly one admin user per role the
// entity hosts (login is per role).
func validateAdminUsers(m *Manifest) []error {
	var errs []error

	if len(m.Spec.AdminUsers) == 0 {
		errs = append(errs, errors.New(
			"spec.adminUsers: required field is missing; "+
				"declare one operator account per role the entity hosts "+
				"(central-bank: ROLE_GOVERNANCE, ROLE_TREASURY, ROLE_NOC_ADMIN; commercial-bank: ROLE_BANK)",
		))
		return errs
	}

	seen := map[string]bool{}
	for i, u := range m.Spec.AdminUsers {
		if u.Role == "" {
			errs = append(errs, fmt.Errorf("spec.adminUsers[%d].role: required field is missing", i))
		}
		if u.Username == "" {
			errs = append(errs, fmt.Errorf("spec.adminUsers[%d].username: required field is missing", i))
		}
		if u.Password == "" {
			errs = append(errs, fmt.Errorf("spec.adminUsers[%d].password: required field is missing", i))
		}
		if u.Role != "" {
			if seen[u.Role] {
				errs = append(errs, fmt.Errorf("spec.adminUsers: duplicate admin user for role %q", u.Role))
			}
			seen[u.Role] = true
		}
	}

	// Each role the entity hosts must have an admin user (login is per role).
	if required, ok := requiredAdminRolesByEntity[m.Spec.Role]; ok {
		for _, role := range required {
			if !seen[role] {
				errs = append(errs, fmt.Errorf(
					"spec.adminUsers: missing required admin user for role %q (role %s hosts: %v)",
					role, m.Spec.Role, required,
				))
			}
		}
	}

	return errs
}
