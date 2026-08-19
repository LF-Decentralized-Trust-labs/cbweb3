// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// ResolveDataDir rewrites m.Spec.Node.DataDir to an absolute path, resolving a
// relative value against the current working directory. Callers must invoke this
// before building engine inputs: spec.node.dataDir is mounted into containers as
// SPOKE_DATA_DIR (a Docker bind-mount source, which must be absolute or it would
// resolve against the compose-file directory), and the bundle OutputDir is derived
// from filepath.Dir(dataDir). A no-op when the path is empty or already absolute.
func ResolveDataDir(m *Manifest) error {
	if m == nil || m.Spec.Node.DataDir == "" || filepath.IsAbs(m.Spec.Node.DataDir) {
		return nil
	}
	abs, err := filepath.Abs(m.Spec.Node.DataDir)
	if err != nil {
		return fmt.Errorf("spec.node.dataDir: resolve %q to absolute path: %w", m.Spec.Node.DataDir, err)
	}
	m.Spec.Node.DataDir = abs
	return nil
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
		case "central-bank", "commercial-bank", "noc":
		default:
			errs = append(errs, fmt.Errorf("spec.role: invalid value %q; accepted values are: central-bank, commercial-bank, noc", m.Spec.Role))
		}
	}

	// spec.mode
	if m.Spec.Mode == "" {
		errs = append(errs, errors.New("spec.mode: required field is missing"))
	} else {
		switch m.Spec.Mode {
		case "found", "join", "observe":
		default:
			errs = append(errs, fmt.Errorf("spec.mode: invalid value %q; accepted values are: found, join, observe", m.Spec.Mode))
		}
	}

	// spec.environment (required, and constrained). It was optional, and an absent
	// value is now load-bearing: it decides the Keycloak realms' sslRequired, where
	// the safe default ("external", TLS demanded) makes a forgotten field surface as
	// a login failing with "HTTPS required" — far from the cause. Requiring it turns
	// that into a validation error naming the field, and matches scenario-b, which
	// has always required it.
	if m.Spec.Environment == "" {
		errs = append(errs, errors.New("spec.environment: required field is missing"))
	} else {
		switch m.Spec.Environment {
		case "local", "staging", "prod":
		default:
			errs = append(errs, fmt.Errorf("spec.environment: invalid value %q; accepted values are: local, staging, prod", m.Spec.Environment))
		}
	}

	// Node-provisioning fields (spoke, node, image, keys, adminUsers) apply to the
	// modes that stand up an on-chain node (found/join). The observe mode deploys
	// the NOC control plane (no Besu node, no keys, users live in the CB realm), so
	// they are skipped for it.
	if m.Spec.Mode != "observe" {

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
		errs = append(errs, validateFiatTokenMetadata(m.Spec.Spoke)...)

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
					"set it to the path where the provisioning engine will store spoke runtime data "+
					"(relative paths are resolved against the current working directory)",
				m.Spec.Mode,
			))
		}

		// spec.joinBundleRef — required when mode is "join"; the bundle drives the join flow.
		if m.Spec.Mode == "join" && m.Spec.JoinBundleRef == "" {
			errs = append(errs, errors.New(
				"spec.joinBundleRef: required field is missing for mode:join; "+
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

	} // end node-mode checks

	// spec.nocBundleRef — required in observe, forbidden otherwise. The NOC users
	// live in the CB realm (provisioned by the CB's found), so observe declares no
	// adminUsers/node/keys.
	if m.Spec.Mode == "observe" {
		if m.Spec.NOCBundleRef == "" {
			errs = append(errs, errors.New("spec.nocBundleRef: required field is missing for mode:observe"))
		}
	} else if m.Spec.NOCBundleRef != "" {
		errs = append(errs, fmt.Errorf("spec.nocBundleRef: field is not allowed for mode:%s", m.Spec.Mode))
	}
	errs = append(errs, validateNOC(m)...)

	// spec.launcher — optional per-entity launcher toggle.
	switch m.Spec.Launcher {
	case "", "enable", "disable":
	default:
		errs = append(errs, fmt.Errorf("spec.launcher: invalid value %q; accepted values are: enable, disable", m.Spec.Launcher))
	}
	// spec.launcherPort — optional; a valid TCP port when set.
	if m.Spec.LauncherPort != 0 && (m.Spec.LauncherPort < 1 || m.Spec.LauncherPort > 65535) {
		errs = append(errs, fmt.Errorf("spec.launcherPort: invalid value %d; must be a TCP port (1-65535)", m.Spec.LauncherPort))
	}
	// spec.proxy — optional per-host reverse-proxy toggle.
	switch m.Spec.Proxy {
	case "", "enable", "disable":
	default:
		errs = append(errs, fmt.Errorf("spec.proxy: invalid value %q; accepted values are: enable, disable", m.Spec.Proxy))
	}

	return errors.Join(errs...)
}

// validateFiatTokenMetadata checks the optional per-spoke fCeBM metadata overrides.
// They are presentation-only: spec.spoke.currency remains the semantic key (it is
// what the portals display via FIAT_SYMBOL), so an override must not smuggle in a
// different currency.
//
// The "<prefix>_<ISO>" symbol shape is a platform convention, not decoration: every
// spoke token is named that way (fCeBM_BRL, fCeBM_COP), and tooling that reads a
// currency out of a symbol takes the segment after the last underscore. Keeping the
// shape enforced here means a custom symbol can never desynchronise from the
// currency it settles in.
func validateFiatTokenMetadata(s Spoke) []error {
	var errs []error
	if s.FiatTokenName != "" && strings.TrimSpace(s.FiatTokenName) == "" {
		errs = append(errs, errors.New(
			"spec.spoke.fiatTokenName: must not be blank when set; omit the field to derive it from spec.spoke.currency"))
	}
	sym := s.FiatTokenSymbol
	if sym == "" {
		return errs // absent → derived from the currency
	}
	if strings.TrimSpace(sym) != sym || strings.ContainsAny(sym, " \t") {
		return append(errs, fmt.Errorf(
			"spec.spoke.fiatTokenSymbol: invalid value %q; an ERC-20 symbol must not contain whitespace", sym))
	}
	idx := strings.LastIndex(sym, "_")
	if idx < 0 || idx == len(sym)-1 {
		want := s.Currency
		if want == "" {
			want = "BRL"
		}
		return append(errs, fmt.Errorf(
			"spec.spoke.fiatTokenSymbol: invalid value %q; the symbol must end in \"_<currency>\" (e.g. fCeBM_%s)", sym, want))
	}
	if code := sym[idx+1:]; s.Currency != "" && !strings.EqualFold(code, s.Currency) {
		errs = append(errs, fmt.Errorf(
			"spec.spoke.fiatTokenSymbol: invalid value %q; its currency segment %q must match spec.spoke.currency %q",
			sym, code, s.Currency))
	}
	return errs
}

// nocComponentTypes are the component types the noc-agent knows how to probe.
var nocComponentTypes = []string{"BESU", "PALADIN", "CACTI_RELAY"}

// validateNOC checks the optional NOC block (present in observe to tune the NOC
// deployment, optionally in found/join to configure the agent). All fields are
// optional; only obviously-invalid values are rejected.
func validateNOC(m *Manifest) []error {
	var errs []error
	if m.Spec.NOC == nil {
		return errs
	}
	if m.Spec.NOC.PushIntervalSeconds < 0 {
		errs = append(errs, fmt.Errorf("spec.noc.pushIntervalSeconds: invalid value %d; must be non-negative", m.Spec.NOC.PushIntervalSeconds))
	}
	for i, c := range m.Spec.NOC.Components {
		ok := false
		for _, t := range nocComponentTypes {
			if c == t {
				ok = true
				break
			}
		}
		if !ok {
			errs = append(errs, fmt.Errorf("spec.noc.components[%d]: invalid value %q; accepted values are: %s", i, c, strings.Join(nocComponentTypes, ", ")))
		}
	}
	return errs
}

// requiredAdminRolesByEntity lists the Keycloak realm roles an entity must
// provision an admin user for, keyed on spec.role. It mirrors the realms/clients
// the engine provisions: a central bank hosts governance + treasury + the shared
// NOC realm; a commercial bank hosts its bank realm.
var requiredAdminRolesByEntity = map[string][]string{
	"central-bank":    {"ROLE_GOVERNANCE", "ROLE_TREASURY", "ROLE_SUPERVISOR", "ROLE_NOC_ADMIN"},
	"commercial-bank": {"ROLE_BANK"},
}

// validateAdminUsers enforces that spec.adminUsers is present, every entry has
// role/username/password, and every role the entity hosts has at least one
// admin user. Multiple accounts MAY share a role (e.g. a whole central-bank
// team each granted ROLE_GOVERNANCE), and one account MAY appear across several
// role entries under the same username — the Keycloak plan groups those entries
// by username into a single realm-import user carrying all its roles (see
// orchestrator.adminUsersForRealmRoles). The only hard rule is that a username
// used more than once must keep a consistent password (the plan takes the first
// occurrence's password, so a mismatch would silently drop credentials).
func validateAdminUsers(m *Manifest) []error {
	var errs []error

	if len(m.Spec.AdminUsers) == 0 {
		errs = append(errs, errors.New(
			"spec.adminUsers: required field is missing; "+
				"declare one operator account per role the entity hosts "+
				"(central-bank: ROLE_GOVERNANCE, ROLE_TREASURY, ROLE_SUPERVISOR, ROLE_NOC_ADMIN; commercial-bank: ROLE_BANK)",
		))
		return errs
	}

	seen := map[string]bool{}
	passwordByUser := map[string]string{}
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
			seen[u.Role] = true
		}
		// A username repeated across role entries must carry the same password:
		// the Keycloak plan keeps the first occurrence, so a conflict would
		// silently drop one credential and confuse the operator.
		if u.Username != "" && u.Password != "" {
			if prev, ok := passwordByUser[u.Username]; ok && prev != u.Password {
				errs = append(errs, fmt.Errorf(
					"spec.adminUsers[%d]: conflicting password for username %q (each entry sharing a username must repeat the same password)", i, u.Username))
			} else {
				passwordByUser[u.Username] = u.Password
			}
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
