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
	m.Spec.Environment = ""   // optional
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
