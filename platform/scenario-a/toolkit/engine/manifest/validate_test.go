// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// validManifestYAML is the minimal valid manifest from the provisioning spec §5.1.
const validManifestYAML = `apiVersion: cbweb3/v1
kind: ParticipantDeployment
metadata:
  name: central-bank-brazil
spec:
  scenario: a
  environment: local
  role: central-bank
  mode: found
  spoke:
    id: spoke-brl
    chainId: 1337
    currency: BRL
  node:
    rpc:
      port: 8645
    ws:
      port: 8655
    p2p:
      port: 31303
    advertisedHost: cbweb3-spoke-brl-besu.central-bank-brazil
    dataDir: /data/spokes/spoke-brl
  image: build
  keyProvider: kms://local-emulator
  certSource: self-signed
  relay:
    endpoint: http://cbweb3-cacti:4000
  adminUsers:
    - role: ROLE_GOVERNANCE
      username: admin@brasil.governance.gov
      password: governance-local
    - role: ROLE_TREASURY
      username: admin@brasil.treasury.gov
      password: treasury-local
    - role: ROLE_SUPERVISOR
      username: admin@brasil.supervisor.gov
      password: supervisor-local
    - role: ROLE_NOC_ADMIN
      username: admin@brasil.noc.gov
      password: noc-local
`

// validManifest returns a fully valid Manifest struct for use in tests.
func validManifest() *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: "cbweb3/v1",
		Kind:       "ParticipantDeployment",
		Metadata:   manifest.Metadata{Name: "central-bank-brazil"},
		Spec: manifest.Spec{
			Scenario:    "a",
			Environment: "local",
			Role:        "central-bank",
			Mode:        "found",
			Spoke: manifest.Spoke{
				ID:       "spoke-brl",
				ChainID:  1337,
				Currency: "BRL",
			},
			Node: manifest.Node{
				AdvertisedHost: "cbweb3-spoke-brl-besu.central-bank-brazil",
				DataDir:        "/data/spokes/spoke-brl",
			},
			Image:       "build",
			KeyProvider: "kms://local-emulator",
			CertSource:  "self-signed",
			AdminUsers: []manifest.AdminUser{
				{Role: "ROLE_GOVERNANCE", Username: "admin@brasil.governance.gov", Password: "governance-local"},
				{Role: "ROLE_TREASURY", Username: "admin@brasil.treasury.gov", Password: "treasury-local"},
				{Role: "ROLE_SUPERVISOR", Username: "admin@brasil.supervisor.gov", Password: "supervisor-local"},
				{Role: "ROLE_NOC_ADMIN", Username: "admin@brasil.noc.gov", Password: "noc-local"},
			},
		},
	}
}

// writeManifestFile writes content to a temp file and returns its path.
func writeManifestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeManifestFile: %v", err)
	}
	return path
}

// ── US1: Valid manifest accepted ─────────────────────────────────────────────

func TestLoad_ValidManifest(t *testing.T) {
	path := writeManifestFile(t, validManifestYAML)

	m, err := manifest.Load(path)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("Load returned nil manifest")
	}
	if m.APIVersion != "cbweb3/v1" {
		t.Errorf("APIVersion = %q; want %q", m.APIVersion, "cbweb3/v1")
	}
	if m.Kind != "ParticipantDeployment" {
		t.Errorf("Kind = %q; want %q", m.Kind, "ParticipantDeployment")
	}
	if m.Metadata.Name != "central-bank-brazil" {
		t.Errorf("Metadata.Name = %q; want %q", m.Metadata.Name, "central-bank-brazil")
	}
	if m.Spec.Role != "central-bank" {
		t.Errorf("Spec.Role = %q; want %q", m.Spec.Role, "central-bank")
	}
	if m.Spec.Spoke.ChainID != 1337 {
		t.Errorf("Spec.Spoke.ChainID = %d; want %d", m.Spec.Spoke.ChainID, 1337)
	}
	if m.Spec.Node.AdvertisedHost != "cbweb3-spoke-brl-besu.central-bank-brazil" {
		t.Errorf("Spec.Node.AdvertisedHost = %q", m.Spec.Node.AdvertisedHost)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := manifest.Load("/nonexistent/path/manifest.yaml")
	if err == nil {
		t.Fatal("Load: expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeManifestFile(t, "apiVersion: [unclosed bracket\n")
	_, err := manifest.Load(path)
	if err == nil {
		t.Fatal("Load: expected error for invalid YAML, got nil")
	}
}

func TestValidate_ValidManifest(t *testing.T) {
	m := validManifest()
	if err := manifest.Validate(m); err != nil {
		t.Errorf("Validate returned unexpected error for valid manifest: %v", err)
	}
}

func TestValidate_ValidManifest_OptionalFieldsOmitted(t *testing.T) {
	m := validManifest()
	m.Spec.Relay = nil        // optional
	m.Spec.JoinBundleRef = "" // optional
	m.Spec.Node.RPC = nil     // optional
	m.Spec.Node.WS = nil      // optional
	m.Spec.Node.P2P = nil     // optional

	if err := manifest.Validate(m); err != nil {
		t.Errorf("Validate returned unexpected error with optional fields omitted: %v", err)
	}
}

// ── US2: Missing required fields ─────────────────────────────────────────────

func TestValidate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name       string
		modify     func(*manifest.Manifest)
		wantInErrs []string
	}{
		{
			name:       "missing apiVersion",
			modify:     func(m *manifest.Manifest) { m.APIVersion = "" },
			wantInErrs: []string{"apiVersion"},
		},
		{
			name:       "missing kind",
			modify:     func(m *manifest.Manifest) { m.Kind = "" },
			wantInErrs: []string{"kind"},
		},
		{
			name:       "missing metadata.name",
			modify:     func(m *manifest.Manifest) { m.Metadata.Name = "" },
			wantInErrs: []string{"metadata.name"},
		},
		{
			name:       "missing spec.scenario",
			modify:     func(m *manifest.Manifest) { m.Spec.Scenario = "" },
			wantInErrs: []string{"spec.scenario"},
		},
		{
			name:       "missing spec.role",
			modify:     func(m *manifest.Manifest) { m.Spec.Role = "" },
			wantInErrs: []string{"spec.role"},
		},
		{
			name:       "missing spec.mode",
			modify:     func(m *manifest.Manifest) { m.Spec.Mode = "" },
			wantInErrs: []string{"spec.mode"},
		},
		{
			// Required since R1-10.7: it decides the Keycloak realms' sslRequired,
			// and an absent value fails login with "HTTPS required" far from the
			// cause. Scenario-b has always required it.
			name:       "missing spec.environment",
			modify:     func(m *manifest.Manifest) { m.Spec.Environment = "" },
			wantInErrs: []string{"spec.environment"},
		},
		{
			name:       "missing spec.spoke.id",
			modify:     func(m *manifest.Manifest) { m.Spec.Spoke.ID = "" },
			wantInErrs: []string{"spec.spoke.id"},
		},
		{
			name:       "missing spec.spoke.chainId",
			modify:     func(m *manifest.Manifest) { m.Spec.Spoke.ChainID = 0 },
			wantInErrs: []string{"spec.spoke.chainId"},
		},
		{
			name:       "missing spec.spoke.currency",
			modify:     func(m *manifest.Manifest) { m.Spec.Spoke.Currency = "" },
			wantInErrs: []string{"spec.spoke.currency"},
		},
		{
			name:       "missing spec.node.advertisedHost",
			modify:     func(m *manifest.Manifest) { m.Spec.Node.AdvertisedHost = "" },
			wantInErrs: []string{"spec.node.advertisedHost"},
		},
		{
			// advertisedHost must include the "never inferred" message (FR-003)
			name:       "advertisedHost empty string includes never-inferred message",
			modify:     func(m *manifest.Manifest) { m.Spec.Node.AdvertisedHost = "" },
			wantInErrs: []string{"never inferred"},
		},
		{
			name:       "missing spec.image",
			modify:     func(m *manifest.Manifest) { m.Spec.Image = "" },
			wantInErrs: []string{"spec.image"},
		},
		{
			name:       "missing spec.keyProvider",
			modify:     func(m *manifest.Manifest) { m.Spec.KeyProvider = "" },
			wantInErrs: []string{"spec.keyProvider"},
		},
		{
			name:       "missing spec.certSource",
			modify:     func(m *manifest.Manifest) { m.Spec.CertSource = "" },
			wantInErrs: []string{"spec.certSource"},
		},
		{
			// spec.node.dataDir required when mode is "found" (FR-008)
			name:       "missing spec.node.dataDir for mode:found",
			modify:     func(m *manifest.Manifest) { m.Spec.Node.DataDir = "" },
			wantInErrs: []string{"spec.node.dataDir"},
		},
		{
			// all required fields missing → all errors reported in one call (FR-004, SC-003)
			name: "all required fields missing reports all errors",
			modify: func(m *manifest.Manifest) {
				m.APIVersion = ""
				m.Kind = ""
				m.Metadata.Name = ""
				m.Spec.Scenario = ""
				m.Spec.Role = ""
				m.Spec.Mode = ""
				m.Spec.Spoke.ID = ""
				m.Spec.Spoke.ChainID = 0
				m.Spec.Spoke.Currency = ""
				m.Spec.Node.AdvertisedHost = ""
				m.Spec.Image = ""
				m.Spec.KeyProvider = ""
				m.Spec.CertSource = ""
			},
			wantInErrs: []string{
				"apiVersion",
				"kind",
				"metadata.name",
				"spec.scenario",
				"spec.role",
				"spec.mode",
				"spec.spoke.id",
				"spec.spoke.chainId",
				"spec.spoke.currency",
				"spec.node.advertisedHost",
				"spec.image",
				"spec.keyProvider",
				"spec.certSource",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.modify(m)

			err := manifest.Validate(m)
			if err == nil {
				t.Fatal("Validate: expected error, got nil")
			}
			msg := err.Error()
			for _, want := range tc.wantInErrs {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not contain %q", msg, want)
				}
			}
		})
	}
}

// ── dataDir mode-specific rule (feature 032 T050) ────────────────────────────

// Feature 032 (mode:join) makes dataDir required for join as well, since the
// join engine uses it as SPOKE_DATA_DIR (genesis, TLS, provisioning state, lock).
func TestValidate_DataDir_RequiredForJoin(t *testing.T) {
	m := validManifest()
	m.Spec.Mode = "join"
	m.Spec.JoinBundleRef = "./bundles/spoke-brl.bundle.yaml"
	m.Spec.Node.DataDir = "" // omit dataDir — must now trigger an error for mode:join
	err := manifest.Validate(m)
	if err == nil || !strings.Contains(err.Error(), "spec.node.dataDir") {
		t.Errorf("dataDir must be required for mode:join, got: %v", err)
	}
}

// Feature 032 T049: joinBundleRef is required for mode:join.
func TestValidate_JoinBundleRef_RequiredForJoin(t *testing.T) {
	m := validManifest()
	m.Spec.Mode = "join"
	m.Spec.Node.DataDir = "/opt/cbweb3/data/bank"
	m.Spec.JoinBundleRef = "" // omit — must trigger an error for mode:join
	err := manifest.Validate(m)
	if err == nil || !strings.Contains(err.Error(), "spec.joinBundleRef") {
		t.Errorf("joinBundleRef must be required for mode:join, got: %v", err)
	}
}

// ── US3: Invalid enum values ──────────────────────────────────────────────────

func TestValidate_InvalidEnumValues(t *testing.T) {
	tests := []struct {
		name         string
		modify       func(*manifest.Manifest)
		wantField    string
		wantBadValue string
		wantAccepted string
		wantExtra    string // optional: additional string that must appear in the error
	}{
		{
			name:         "invalid spec.role",
			modify:       func(m *manifest.Manifest) { m.Spec.Role = "central" },
			wantField:    "spec.role",
			wantBadValue: "central",
			wantAccepted: "central-bank",
		},
		{
			name:         "invalid spec.mode",
			modify:       func(m *manifest.Manifest) { m.Spec.Mode = "init" },
			wantField:    "spec.mode",
			wantBadValue: "init",
			wantAccepted: "found",
		},
		{
			name:         "invalid spec.scenario",
			modify:       func(m *manifest.Manifest) { m.Spec.Scenario = "b" },
			wantField:    "spec.scenario",
			wantBadValue: "b",
			wantAccepted: "a",
			wantExtra:    "out of scope",
		},
		{
			name:         "invalid spec.environment",
			modify:       func(m *manifest.Manifest) { m.Spec.Environment = "dev" },
			wantField:    "spec.environment",
			wantBadValue: "dev",
			wantAccepted: "local",
		},
		{
			name:         "invalid apiVersion",
			modify:       func(m *manifest.Manifest) { m.APIVersion = "cbweb3/v2" },
			wantField:    "apiVersion",
			wantBadValue: "cbweb3/v2",
			wantAccepted: "cbweb3/v1",
		},
		{
			name:         "invalid kind",
			modify:       func(m *manifest.Manifest) { m.Kind = "ServiceDeployment" },
			wantField:    "kind",
			wantBadValue: "ServiceDeployment",
			wantAccepted: "ParticipantDeployment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.modify(m)

			err := manifest.Validate(m)
			if err == nil {
				t.Fatal("Validate: expected error, got nil")
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.wantField) {
				t.Errorf("error %q does not contain field %q", msg, tc.wantField)
			}
			if !strings.Contains(msg, tc.wantBadValue) {
				t.Errorf("error %q does not contain bad value %q", msg, tc.wantBadValue)
			}
			if !strings.Contains(msg, tc.wantAccepted) {
				t.Errorf("error %q does not contain accepted value %q", msg, tc.wantAccepted)
			}
			if !strings.Contains(msg, "accepted values are") {
				t.Errorf("error %q does not contain %q", msg, "accepted values are")
			}
			if tc.wantExtra != "" && !strings.Contains(msg, tc.wantExtra) {
				t.Errorf("error %q does not contain %q", msg, tc.wantExtra)
			}
		})
	}
}

// ── spec.adminUsers (mandatory, per-role) ────────────────────────────────────

func TestValidate_AdminUsers(t *testing.T) {
	t.Run("missing adminUsers is rejected", func(t *testing.T) {
		m := validManifest()
		m.Spec.AdminUsers = nil
		err := manifest.Validate(m)
		if err == nil || !strings.Contains(err.Error(), "spec.adminUsers") {
			t.Errorf("expected spec.adminUsers error, got %v", err)
		}
	})

	t.Run("missing required role is rejected", func(t *testing.T) {
		m := validManifest()
		// Drop the treasury admin → central-bank must still require ROLE_TREASURY.
		m.Spec.AdminUsers = []manifest.AdminUser{
			{Role: "ROLE_GOVERNANCE", Username: "admin@brasil.governance.gov", Password: "p"},
			{Role: "ROLE_NOC_ADMIN", Username: "admin@brasil.noc.gov", Password: "p"},
		}
		err := manifest.Validate(m)
		if err == nil || !strings.Contains(err.Error(), "ROLE_TREASURY") {
			t.Errorf("expected missing ROLE_TREASURY error, got %v", err)
		}
	})

	t.Run("entry missing password is rejected", func(t *testing.T) {
		m := validManifest()
		m.Spec.AdminUsers[0].Password = ""
		err := manifest.Validate(m)
		if err == nil || !strings.Contains(err.Error(), "password") {
			t.Errorf("expected password error, got %v", err)
		}
	})

	t.Run("commercial-bank requires ROLE_BANK", func(t *testing.T) {
		m := validManifest()
		m.Spec.Role = "commercial-bank"
		m.Spec.AdminUsers = []manifest.AdminUser{
			{Role: "ROLE_GOVERNANCE", Username: "x@y.z", Password: "p"},
		}
		err := manifest.Validate(m)
		if err == nil || !strings.Contains(err.Error(), "ROLE_BANK") {
			t.Errorf("expected missing ROLE_BANK error, got %v", err)
		}
	})

	t.Run("multiple accounts may share a role", func(t *testing.T) {
		m := validManifest()
		// A whole central-bank team, each granted the same set of CB roles, plus a
		// supervisor-only regulator — all required roles are still covered.
		m.Spec.AdminUsers = []manifest.AdminUser{
			{Role: "ROLE_GOVERNANCE", Username: "a@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_TREASURY", Username: "a@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_SUPERVISOR", Username: "a@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_NOC_ADMIN", Username: "a@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_GOVERNANCE", Username: "b@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_TREASURY", Username: "b@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_SUPERVISOR", Username: "b@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_NOC_ADMIN", Username: "b@bccr.fi.cr", Password: "p"},
			{Role: "ROLE_SUPERVISOR", Username: "reg@sugeval.fi.cr", Password: "p"},
		}
		if err := manifest.Validate(m); err != nil {
			t.Errorf("multiple accounts per role should be allowed, got %v", err)
		}
	})

	t.Run("same username with conflicting passwords is rejected", func(t *testing.T) {
		m := validManifest()
		m.Spec.AdminUsers = append(m.Spec.AdminUsers,
			manifest.AdminUser{Role: "ROLE_TREASURY", Username: m.Spec.AdminUsers[0].Username, Password: "a-different-password"},
		)
		err := manifest.Validate(m)
		if err == nil || !strings.Contains(err.Error(), "conflicting password") {
			t.Errorf("expected conflicting-password error, got %v", err)
		}
	})
}

// ── ResolveDataDir: relative node.dataDir is resolved against CWD ──────────────

func TestResolveDataDir_RelativeBecomesAbsolute(t *testing.T) {
	m := validManifest()
	m.Spec.Node.DataDir = "cbweb3-data/central-bank-brazil"
	if err := manifest.ResolveDataDir(m); err != nil {
		t.Fatalf("ResolveDataDir: %v", err)
	}
	if !filepath.IsAbs(m.Spec.Node.DataDir) {
		t.Errorf("dataDir should be absolute after resolution, got %q", m.Spec.Node.DataDir)
	}
	cwd, _ := os.Getwd()
	want := filepath.Join(cwd, "cbweb3-data", "central-bank-brazil")
	if m.Spec.Node.DataDir != want {
		t.Errorf("dataDir = %q, want %q", m.Spec.Node.DataDir, want)
	}
}

func TestResolveDataDir_AbsoluteUnchanged(t *testing.T) {
	m := validManifest()
	m.Spec.Node.DataDir = "/opt/cbweb3/data/central-bank-brazil"
	if err := manifest.ResolveDataDir(m); err != nil {
		t.Fatalf("ResolveDataDir: %v", err)
	}
	if m.Spec.Node.DataDir != "/opt/cbweb3/data/central-bank-brazil" {
		t.Errorf("absolute dataDir must be left unchanged, got %q", m.Spec.Node.DataDir)
	}
}

func TestResolveDataDir_EmptyIsNoOp(t *testing.T) {
	m := validManifest()
	m.Spec.Node.DataDir = ""
	if err := manifest.ResolveDataDir(m); err != nil {
		t.Fatalf("ResolveDataDir: %v", err)
	}
	if m.Spec.Node.DataDir != "" {
		t.Errorf("empty dataDir must stay empty, got %q", m.Spec.Node.DataDir)
	}
}

// ── R1-10.3: per-spoke fCeBM metadata overrides ───────────────────────────────

// The overrides are optional; declared, they are accepted when the symbol keeps the
// "<prefix>_<ISO>" shape ending in this spoke's own currency.
func TestValidate_FiatTokenOverrides_Accepted(t *testing.T) {
	m := validManifest()
	m.Spec.Spoke.FiatTokenName = "Real Digital"
	m.Spec.Spoke.FiatTokenSymbol = "fRD_BRL"
	if err := manifest.Validate(m); err != nil {
		t.Fatalf("expected valid manifest, got: %v", err)
	}
}

// Omitting them stays valid — the step derives "Fiat <ISO>" / "fCeBM_<ISO>".
func TestValidate_FiatTokenOverrides_Omitted(t *testing.T) {
	m := validManifest()
	if err := manifest.Validate(m); err != nil {
		t.Fatalf("expected valid manifest without overrides, got: %v", err)
	}
}

func TestValidate_FiatTokenSymbol_Rejected(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
	}{
		{"no currency segment", "fBRL"},
		{"trailing underscore", "fCeBM_"},
		{"whitespace", "fCeBM BRL"},
		{"currency mismatch", "fCeBM_COP"}, // validManifest settles in BRL
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			m.Spec.Spoke.FiatTokenSymbol = tc.symbol
			err := manifest.Validate(m)
			if err == nil || !strings.Contains(err.Error(), "spec.spoke.fiatTokenSymbol") {
				t.Errorf("symbol %q must be rejected, got: %v", tc.symbol, err)
			}
		})
	}
}

// A blank name is rejected: omit the field to derive it from the currency.
func TestValidate_FiatTokenName_BlankRejected(t *testing.T) {
	m := validManifest()
	m.Spec.Spoke.FiatTokenName = "   "
	err := manifest.Validate(m)
	if err == nil || !strings.Contains(err.Error(), "spec.spoke.fiatTokenName") {
		t.Errorf("blank fiatTokenName must be rejected, got: %v", err)
	}
}
