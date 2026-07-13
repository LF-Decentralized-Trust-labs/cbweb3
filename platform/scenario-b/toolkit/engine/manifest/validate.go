// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Recognized enum values (also mirrored by the published JSON-Schema).
const (
	wantAPIVersion = "cbweb3b/v1"
	wantKind       = "ParticipantDeployment"
	wantScenario   = "b"
	wantEnv        = "local"

	ModeFoundHub   = "found-hub"
	ModeFoundSpoke = "found-spoke"
	ModeJoin       = "join"
)

// Modes lists the recognized spec.mode values.
var Modes = []string{ModeFoundHub, ModeFoundSpoke, ModeJoin}

// Roles lists the recognized spec.topology.role values.
var Roles = []string{"hub", "central-bank", "commercial-bank"}

// RequiredByMode is the per-mode set of spec fields that MUST be present.
// It is the source of truth for the required half of the per-mode matrix and is
// compared against the published JSON-Schema by the parity test (SC-004).
var RequiredByMode = map[string][]string{
	ModeFoundHub:   {"hub"},
	ModeFoundSpoke: {"spoke", "hubBundleRef"},
	ModeJoin:       {"spoke", "joinBundleRef", "bankId"},
}

// ForbiddenByMode is the per-mode set of spec fields that MUST NOT be present.
// It is the source of truth for the forbidden half of the per-mode matrix and
// is compared against the published JSON-Schema by the parity test (SC-004).
var ForbiddenByMode = map[string][]string{
	ModeFoundHub:   {"spoke", "hubBundleRef", "joinBundleRef", "bankId", "pair"},
	ModeFoundSpoke: {"hub", "joinBundleRef", "bankId"},
	ModeJoin:       {"hub", "hubBundleRef", "pair", "cbEndpoint"},
}

var (
	keyProviderRe = regexp.MustCompile(`^kms://`)
	certSourceRe  = regexp.MustCompile(`^(self-signed(://.*)?|ca://.+)$`)
	// hexKeyRe matches a bare or 0x-prefixed 64-hex-character private key.
	hexKeyRe = regexp.MustCompile(`(?i)\b(0x)?[0-9a-f]{64}\b`)
)

// Validate checks a single manifest and returns all findings (errors +
// warnings), collected in one pass (FR-011). A manifest is valid when the
// returned Result has no errors; warnings do not make it invalid.
func Validate(pd *ParticipantDeployment) Result {
	var r Result
	if pd == nil {
		r.AddError("manifest", "nil manifest")
		return r
	}

	// FR-001: apiVersion / kind. These gate everything else conceptually, but
	// we still collect the remaining findings so the operator sees them at once.
	if pd.APIVersion == "" {
		r.AddError("apiVersion", "required field is missing")
	} else if pd.APIVersion != wantAPIVersion {
		r.AddError("apiVersion", fmt.Sprintf("invalid value %q; must be %q", pd.APIVersion, wantAPIVersion))
	}
	if pd.Kind == "" {
		r.AddError("kind", "required field is missing")
	} else if pd.Kind != wantKind {
		r.AddError("kind", fmt.Sprintf("invalid value %q; must be %q", pd.Kind, wantKind))
	}

	// metadata.name
	if pd.Metadata.Name == "" {
		r.AddError("metadata.name", "required field is missing")
	}

	spec := pd.Spec

	// FR-002 + scenario: scenario / mode / role discriminators.
	if spec.Scenario == "" {
		r.AddError("spec.scenario", "required field is missing")
	} else if spec.Scenario != wantScenario {
		r.AddError("spec.scenario", fmt.Sprintf("invalid value %q; must be %q", spec.Scenario, wantScenario))
	}

	if spec.Mode == "" {
		r.AddError("spec.mode", "required field is missing")
	} else if !contains(Modes, spec.Mode) {
		r.AddError("spec.mode", fmt.Sprintf("invalid value %q; accepted values are: %s", spec.Mode, strings.Join(Modes, ", ")))
	}

	if spec.Topology.Role == "" {
		r.AddError("spec.topology.role", "required field is missing")
	} else if !contains(Roles, spec.Topology.Role) {
		r.AddError("spec.topology.role", fmt.Sprintf("invalid value %q; accepted values are: %s", spec.Topology.Role, strings.Join(Roles, ", ")))
	}

	// FR-005: environment must be local in this phase.
	if spec.Environment == "" {
		r.AddError("spec.environment", "required field is missing")
	} else if spec.Environment != wantEnv {
		r.AddError("spec.environment", fmt.Sprintf("invalid value %q; only %q is supported in this phase (staging/prod are rejected)", spec.Environment, wantEnv))
	}

	// FR-004: node addressing.
	validateNode(spec.Node, &r)

	// Always-required scalar/object fields (structural, mirrored by schema).
	if spec.Image == "" {
		r.AddError("spec.image", "required field is missing")
	}
	// FR-007: keyProvider / certSource URIs.
	if spec.KeyProvider == "" {
		r.AddError("spec.keyProvider", "required field is missing")
	} else if !keyProviderRe.MatchString(spec.KeyProvider) {
		r.AddError("spec.keyProvider", fmt.Sprintf("invalid value %q; must match kms://…", spec.KeyProvider))
	}
	if spec.CertSource == "" {
		r.AddError("spec.certSource", "required field is missing")
	} else if !certSourceRe.MatchString(spec.CertSource) {
		r.AddError("spec.certSource", fmt.Sprintf("invalid value %q; must be self-signed, self-signed://… or ca://…", spec.CertSource))
	}
	validateRelay(spec.Relay, &r)
	if spec.FrontendHost == "" {
		r.AddError("spec.frontendHost", "required field is missing")
	}
	validateAdminUsers(spec.AdminUsers, &r)

	// Launcher (optional): enable | disable when present.
	if spec.Launcher != "" && spec.Launcher != "enable" && spec.Launcher != "disable" {
		r.AddError("spec.launcher", fmt.Sprintf("invalid value %q; accepted values are: enable, disable", spec.Launcher))
	}
	// LauncherPort (optional): a valid TCP port when set.
	if spec.LauncherPort != 0 && (spec.LauncherPort < 1 || spec.LauncherPort > 65535) {
		r.AddError("spec.launcherPort", fmt.Sprintf("invalid value %d; must be a TCP port (1-65535)", spec.LauncherPort))
	}

	// FR-003: per-mode required + forbidden presence.
	if contains(Modes, spec.Mode) {
		validateModeMatrix(pd, &r)
	}

	// FR-012: pair (found-spoke only), when present.
	validatePair(spec.Pair, &r)

	// FR-013: join with node.validator: true → warning (not error).
	if spec.Mode == ModeJoin && spec.Node != nil && spec.Node.Validator != nil && *spec.Node.Validator {
		r.AddWarning("spec.node.validator",
			"validator: true diverges from the join model (non-validating full node); "+
				"accepted, but promotion to validator is deferred (vote-qbft)")
	}

	// FR-006: no secret material anywhere in the manifest.
	scanSecrets("", reflect.ValueOf(pd), &r)

	return r
}

// validateNode checks node addressing (FR-004).
func validateNode(n *Node, r *Result) {
	if n == nil {
		r.AddError("spec.node", "required field is missing")
		return
	}
	if n.AdvertisedHost == "" {
		r.AddError("spec.node.advertisedHost",
			"required field is missing; it must be set explicitly and is never inferred from co-location")
	}
	validatePort("spec.node.rpc", n.RPC, r)
	validatePort("spec.node.ws", n.WS, r)
	validatePort("spec.node.p2p", n.P2P, r)
	if n.DataDir == "" {
		r.AddError("spec.node.dataDir",
			"required field is missing; set the host path where runtime data will be stored (relative paths resolve against the current working directory)")
	}
}

func validatePort(field string, p *Port, r *Result) {
	if p == nil {
		r.AddError(field, "required field is missing")
		return
	}
	if p.Port <= 0 {
		r.AddError(field+".port", fmt.Sprintf("invalid value %d; must be a positive integer", p.Port))
	}
}

func validateRelay(rel *Relay, r *Result) {
	if rel == nil {
		r.AddError("spec.relay", "required field is missing")
		return
	}
	if rel.Endpoint == "" {
		r.AddError("spec.relay.endpoint", "required field is missing")
	}
}

func validateAdminUsers(users []AdminUser, r *Result) {
	if len(users) == 0 {
		r.AddError("spec.adminUsers", "required field is missing; declare at least one operator account")
		return
	}
	for i, u := range users {
		if u.Role == "" {
			r.AddError(fmt.Sprintf("spec.adminUsers[%d].role", i), "required field is missing")
		}
		if u.Username == "" {
			r.AddError(fmt.Sprintf("spec.adminUsers[%d].username", i), "required field is missing")
		}
		if u.Password == "" {
			r.AddError(fmt.Sprintf("spec.adminUsers[%d].password", i), "required field is missing")
		}
	}
}

// validateModeMatrix enforces the per-mode required (FR-003) and forbidden
// (FR-003) presence rules using the RequiredByMode / ForbiddenByMode tables.
func validateModeMatrix(pd *ParticipantDeployment, r *Result) {
	mode := pd.Spec.Mode
	for _, field := range RequiredByMode[mode] {
		if !specFieldPresent(pd, field) {
			r.AddError("spec."+field, fmt.Sprintf("required field is missing for mode:%s", mode))
		}
	}
	for _, field := range ForbiddenByMode[mode] {
		if specFieldPresent(pd, field) {
			r.AddError("spec."+field, fmt.Sprintf("field is not allowed for mode:%s", mode))
		}
	}
}

// specFieldPresent reports whether a per-mode-controlled spec field is present.
// Object fields (hub/spoke/pair) use pointer non-nil; scalar fields use a
// non-empty value.
func specFieldPresent(pd *ParticipantDeployment, field string) bool {
	s := pd.Spec
	switch field {
	case "hub":
		return s.Hub != nil
	case "spoke":
		return s.Spoke != nil
	case "pair":
		return s.Pair != nil
	case "hubBundleRef":
		return s.HubBundleRef != ""
	case "joinBundleRef":
		return s.JoinBundleRef != ""
	case "bankId":
		return s.BankID != ""
	case "cbEndpoint":
		return s.CBEndpoint != ""
	default:
		return false
	}
}

// validatePair enforces FR-012 when the sovereign pair is present.
func validatePair(p *Pair, r *Result) {
	if p == nil {
		return
	}
	if p.ProposerCB == "" {
		r.AddError("spec.pair.proposerCB", "required field is missing")
	}
	if p.ConfirmerCB == "" {
		r.AddError("spec.pair.confirmerCB", "required field is missing")
	}
	if p.SymbolA == "" {
		r.AddError("spec.pair.symbolA", "required field is missing")
	}
	if p.SymbolB == "" {
		r.AddError("spec.pair.symbolB", "required field is missing")
	}
	if p.ProposerCB != "" && p.ProposerCB == p.ConfirmerCB {
		r.AddError("spec.pair.confirmerCB", fmt.Sprintf("confirmerCB must differ from proposerCB (both %q)", p.ProposerCB))
	}
	if p.SymbolA != "" && p.SymbolA == p.SymbolB {
		r.AddError("spec.pair.symbolB", fmt.Sprintf("symbolB must differ from symbolA (both %q)", p.SymbolA))
	}
}

// scanSecrets walks every string value reachable from v and rejects any that
// carries private-key material (FR-006). The field path is built from yaml
// tags so findings name the offending field.
func scanSecrets(path string, v reflect.Value, r *Result) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return
		}
		scanSecrets(path, v.Elem(), r)
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name := yamlFieldName(f)
			if name == "-" {
				continue
			}
			scanSecrets(joinPath(path, name), v.Field(i), r)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			scanSecrets(fmt.Sprintf("%s[%d]", path, i), v.Index(i), r)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			scanSecrets(fmt.Sprintf("%s.%v", path, k.Interface()), v.MapIndex(k), r)
		}
	case reflect.String:
		if hasSecret(v.String()) {
			field := path
			if field == "" {
				field = "manifest"
			}
			r.AddError(field, "must not contain private-key material; key/cert material is referenced via keyProvider/certSource, never inline")
		}
	}
}

func hasSecret(s string) bool {
	if strings.Contains(strings.ToUpper(s), "PRIVATE KEY") {
		return true
	}
	return hexKeyRe.MatchString(s)
}

func yamlFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" {
		return f.Name
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		return f.Name
	}
	return name
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
