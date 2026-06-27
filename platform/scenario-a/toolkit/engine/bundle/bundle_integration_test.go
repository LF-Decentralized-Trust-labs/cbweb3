//go:build integration

// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"gopkg.in/yaml.v3"
)

// TestEmitBundle_Integration verifies SC-002 through SC-009 end-to-end
// using a realistic SPOKE_DATA_DIR with a generated TLS CA cert and
// a mock Besu JSON-RPC server via httptest.
func TestEmitBundle_Integration(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()

	// 1. Generate CA cert via crypto/x509
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// 2. Populate data dir
	genesisContent := `{"config":{"chainId":1337,"byzantiumBlock":0},"difficulty":"0x1","gasLimit":"0x1fffffffffffff","alloc":{}}`
	genesisDir := filepath.Join(dir, "genesis")
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(genesisDir, 0o755)
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(genesisDir, "genesis.json"), []byte(genesisContent), 0o644)
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), certPEM, 0o644)

	addrsEnv := `REGISTRY_CONTRACT_ADDRESS=0xREGISTRY
ZETO_FACTORY_ADDRESS=0xZETO_FACTORY
PENTE_FACTORY_ADDRESS=0xPENTE_FACTORY
ZETO_TOKEN_ADDRESS=0xZETO_TOKEN
PENTE_CONTEXT_GROUP_ID=0xPENTE_GROUP
PENTE_CONTEXT_ADDRESS=0xPENTE_ADDR
FX_AGREEMENT_DEPLOYED_AT=0xFX_AGREEMENT
`
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(addrsEnv), 0o644)

	// 3. Mock Besu server
	besuEnode := "enode://deadbeefcafe@0.0.0.0:30303"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]string{"enode": besuEnode},
		})
	}))
	defer srv.Close()

	// 4. Build manifest and run EmitBundle
	m := &manifest.Manifest{
		APIVersion: "cbweb3/v1",
		Kind:       "ParticipantDeployment",
		Metadata:   manifest.Metadata{Name: "integration-bank"},
		Spec: manifest.Spec{
			Scenario: "a",
			Mode:     "found",
			Spoke:    manifest.Spoke{ID: "spoke-intg", ChainID: 1337, Currency: "BRL"},
			Node: manifest.Node{
				AdvertisedHost: "integration.bank.example",
				P2P:            &manifest.Port{Port: 31303},
			},
			Relay: &manifest.Relay{Endpoint: "http://relay:4000"},
		},
	}

	ep := NewBesuEnodeProvider(srv.URL, nil)
	ctx := context.Background()

	jb, err := EmitBundle(ctx, BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("EmitBundle: %v", err)
	}

	// 5. Verify returned struct (SC-002, SC-003, SC-004, SC-005)
	if jb.APIVersion != "cbweb3/v1" {
		t.Errorf("SC-002: apiVersion = %q; want cbweb3/v1", jb.APIVersion)
	}
	if jb.Kind != "JoinBundle" {
		t.Errorf("SC-002: kind = %q; want JoinBundle", jb.Kind)
	}
	wantEnode := "enode://deadbeefcafe@integration.bank.example:31303"
	if jb.Spec.Bootnode.Enode != wantEnode {
		t.Errorf("SC-004: enode = %q; want %q", jb.Spec.Bootnode.Enode, wantEnode)
	}
	if !strings.HasPrefix(jb.Spec.Genesis.Hash, "sha256:") || len(jb.Spec.Genesis.Hash) != len("sha256:")+64 {
		t.Errorf("SC-003: genesis.hash format invalid: %q", jb.Spec.Genesis.Hash)
	}
	if jb.Spec.Contracts.RegistryAddress == "" {
		t.Error("SC-005: registryAddress should not be empty")
	}
	if jb.Spec.Relay == nil || jb.Spec.Relay.Endpoint == "" {
		t.Error("SC-005: relay.endpoint should be set")
	}

	// 6. Verify file on disk (SC-001, SC-006, SC-008, SC-009)
	bundlePath := filepath.Join(outputDir, "bundles", "spoke-intg.bundle.yaml")
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("SC-001: bundle file not found: %v", err)
	}

	// SC-006: no private key material
	if strings.Contains(string(data), "PRIVATE KEY") {
		t.Error("SC-006: bundle YAML contains private key material")
	}

	// SC-007: no internal addresses in enode
	if strings.Contains(string(data), "0.0.0.0") {
		t.Error("SC-007: bundle should not contain 0.0.0.0 (Besu container address)")
	}

	// Parse and verify structure
	var parsed JoinBundle
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("SC-009: bundle is not valid YAML: %v", err)
	}
	if parsed.APIVersion != "cbweb3/v1" {
		t.Errorf("SC-009: parsed apiVersion = %q", parsed.APIVersion)
	}
	if !strings.Contains(parsed.Spec.Trust.CACertPEM, "-----BEGIN CERTIFICATE-----") {
		t.Error("SC-002: caCertPEM should contain BEGIN CERTIFICATE header")
	}

	// SC-008: Re-emission produces identical structural content
	jb2, err := EmitBundle(ctx, BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("SC-008: second EmitBundle: %v", err)
	}
	jb.Metadata.GeneratedAt = ""
	jb2.Metadata.GeneratedAt = ""
	if jb.Spec.Bootnode.Enode != jb2.Spec.Bootnode.Enode ||
		jb.Spec.Genesis.Hash != jb2.Spec.Genesis.Hash ||
		jb.Spec.Contracts.RegistryAddress != jb2.Spec.Contracts.RegistryAddress {
		t.Error("SC-008: re-emission produced different structural content")
	}
}
