// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"gopkg.in/yaml.v3"
)

// --- helpers ---

// staticEnodeProvider implements EnodeProvider for tests.
type staticEnodeProvider struct {
	enode string
	err   error
}

func (s *staticEnodeProvider) NodeInfo(_ context.Context) (string, error) {
	return s.enode, s.err
}

// ctxAwareProvider checks ctx before returning.
type ctxAwareProvider struct {
	enode string
}

func (p *ctxAwareProvider) NodeInfo(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return p.enode, nil
	}
}

// makeManifest builds a minimal valid "found" manifest.
func makeManifest(spokeID, advertisedHost string, p2pPort int) *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: "cbweb3/v1",
		Kind:       "ParticipantDeployment",
		Metadata:   manifest.Metadata{Name: "test-bank"},
		Spec: manifest.Spec{
			Scenario: "a",
			Mode:     "found",
			Spoke:    manifest.Spoke{ID: spokeID, ChainID: 1337, Currency: "BRL"},
			Node: manifest.Node{
				AdvertisedHost: advertisedHost,
				P2P:            &manifest.Port{Port: p2pPort},
			},
			Relay: &manifest.Relay{Endpoint: "http://relay:4000"},
		},
	}
}

// populateDataDir writes minimal valid artifacts into dir.
func populateDataDir(t *testing.T, dir string) {
	t.Helper()
	genesisDir := filepath.Join(dir, "genesis")
	tlsDir := filepath.Join(dir, "tls")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatalf("mkdir genesis: %v", err)
	}
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		t.Fatalf("mkdir tls: %v", err)
	}
	if err := os.WriteFile(filepath.Join(genesisDir, "genesis.json"), []byte(`{"config":{},"alloc":{}}`), 0o644); err != nil {
		t.Fatalf("write genesis: %v", err)
	}
	caCert := "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA\n-----END CERTIFICATE-----\n"
	if err := os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte(caCert), 0o644); err != nil {
		t.Fatalf("write CA cert: %v", err)
	}
	addrsEnv := `REGISTRY_CONTRACT_ADDRESS=0xAAA
ZETO_FACTORY_ADDRESS=0xBBB
PENTE_FACTORY_ADDRESS=0xCCC
ZETO_TOKEN_ADDRESS=0xDDD
PENTE_CONTEXT_GROUP_ID=0xEEE
PENTE_CONTEXT_ADDRESS=0xFFF
FX_AGREEMENT_DEPLOYED_AT=0xGGG
`
	if err := os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(addrsEnv), 0o644); err != nil {
		t.Fatalf("write addrs: %v", err)
	}
}

// --- readGenesis tests (T010) ---

func TestReadGenesis_Success(t *testing.T) {
	dir := t.TempDir()
	genesisContent := `{"config":{"chainId":1337},"alloc":{}}`
	genesisDir := filepath.Join(dir, "genesis")
	os.MkdirAll(genesisDir, 0o755)
	os.WriteFile(filepath.Join(genesisDir, "genesis.json"), []byte(genesisContent), 0o644)

	spec, err := readGenesis(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SC-005: verify exact SHA-256 hash value, not just format
	sum := sha256.Sum256([]byte(genesisContent))
	expectedHash := fmt.Sprintf("sha256:%x", sum)
	if spec.Hash != expectedHash {
		t.Errorf("hash = %q; want %q", spec.Hash, expectedHash)
	}

	// FR-005: verify content decodes back to original genesis bytes
	decoded, err := base64.StdEncoding.DecodeString(spec.Content)
	if err != nil {
		t.Fatalf("content is not valid base64: %v", err)
	}
	if string(decoded) != genesisContent {
		t.Errorf("decoded content = %q; want %q", string(decoded), genesisContent)
	}
}

func TestReadGenesis_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := readGenesis(context.Background(), dir)
	if !errors.Is(err, ErrGenesisNotFound) {
		t.Errorf("expected ErrGenesisNotFound, got %v", err)
	}
}

// --- readCACert tests (T012) ---

func TestReadCACert_ValidPEM(t *testing.T) {
	dir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	caCert := "-----BEGIN CERTIFICATE-----\nMIIBIjANBg==\n-----END CERTIFICATE-----\n"
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte(caCert), 0o644)

	spec, err := readCACert(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.CACertPEM == "" {
		t.Error("CACertPEM should not be empty")
	}
}

func TestReadCACert_ContainsPrivateKey(t *testing.T) {
	dir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	evil := "-----BEGIN CERTIFICATE-----\nMIIBIjANBg==\n-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----\nMIIEvgIB\n-----END PRIVATE KEY-----\n"
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte(evil), 0o644)

	_, err := readCACert(dir)
	if !errors.Is(err, ErrCACertNotFound) {
		t.Errorf("expected ErrCACertNotFound for private key material, got %v", err)
	}
	if !strings.Contains(err.Error(), "private key material") {
		t.Errorf("error should mention private key material, got %q", err.Error())
	}
}

func TestReadCACert_NoPEMBlock(t *testing.T) {
	dir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte("not pem content\n"), 0o644)

	_, err := readCACert(dir)
	if !errors.Is(err, ErrCACertNotFound) {
		t.Errorf("expected ErrCACertNotFound for no PEM block, got %v", err)
	}
}

func TestReadCACert_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := readCACert(dir)
	if !errors.Is(err, ErrCACertNotFound) {
		t.Errorf("expected ErrCACertNotFound for missing file, got %v", err)
	}
}

// --- readContracts tests (T014) ---

func TestReadContracts_AllKeys(t *testing.T) {
	dir := t.TempDir()
	env := `REGISTRY_CONTRACT_ADDRESS=0xA
ZETO_FACTORY_ADDRESS=0xB
PENTE_FACTORY_ADDRESS=0xC
ZETO_TOKEN_ADDRESS=0xD
PENTE_CONTEXT_GROUP_ID=0xE
PENTE_CONTEXT_ADDRESS=0xF
FX_AGREEMENT_DEPLOYED_AT=0x1
FIAT_TOKEN_ADDRESS=0xF1A7
HTLC_ADDRESS=0x47C
`
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(env), 0o644)

	spec, err := readContracts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.RegistryAddress != "0xA" {
		t.Errorf("RegistryAddress = %q; want 0xA", spec.RegistryAddress)
	}
	if spec.FXAgreementAddress != "0x1" {
		t.Errorf("FXAgreementAddress = %q; want 0x1", spec.FXAgreementAddress)
	}
	if spec.FiatTokenAddress != "0xF1A7" {
		t.Errorf("FiatTokenAddress = %q; want 0xF1A7", spec.FiatTokenAddress)
	}
	if spec.HTLCAddress != "0x47C" {
		t.Errorf("HTLCAddress = %q; want 0x47C", spec.HTLCAddress)
	}
}

func TestReadContracts_MissingKey(t *testing.T) {
	dir := t.TempDir()
	// Only one key, REGISTRY_CONTRACT_ADDRESS empty
	env := `REGISTRY_CONTRACT_ADDRESS=
ZETO_FACTORY_ADDRESS=0xB
PENTE_FACTORY_ADDRESS=0xC
ZETO_TOKEN_ADDRESS=0xD
PENTE_CONTEXT_GROUP_ID=0xE
PENTE_CONTEXT_ADDRESS=0xF
FX_AGREEMENT_DEPLOYED_AT=0x1
`
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(env), 0o644)

	_, err := readContracts(dir)
	if !errors.Is(err, ErrDeployedAddrsIncomplete) {
		t.Errorf("expected ErrDeployedAddrsIncomplete, got %v", err)
	}
	if !strings.Contains(err.Error(), "REGISTRY_CONTRACT_ADDRESS") {
		t.Errorf("error should mention key name, got %q", err.Error())
	}
}

func TestReadContracts_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	// ParseDeployedAddrs returns empty struct for missing file; readContracts should then fail on first empty key
	_, err := readContracts(dir)
	if !errors.Is(err, ErrDeployedAddrsIncomplete) {
		t.Errorf("expected ErrDeployedAddrsIncomplete for absent file, got %v", err)
	}
}

// --- EmitBundle end-to-end tests (T016) ---

func TestEmitBundle_Success(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	jb, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify returned struct
	if jb.APIVersion != "cbweb3/v1" {
		t.Errorf("apiVersion = %q; want cbweb3/v1", jb.APIVersion)
	}
	if jb.Kind != "JoinBundle" {
		t.Errorf("kind = %q; want JoinBundle", jb.Kind)
	}
	if jb.Spec.Bootnode.Enode != "enode://abc@bank.local:31303" {
		t.Errorf("enode = %q; want enode://abc@bank.local:31303", jb.Spec.Bootnode.Enode)
	}
	if !strings.HasPrefix(jb.Spec.Genesis.Hash, "sha256:") {
		t.Errorf("genesis.hash should start with sha256:, got %q", jb.Spec.Genesis.Hash)
	}

	// Verify file exists and is valid YAML
	bundlePath := filepath.Join(outputDir, "bundles", "spoke-brl.bundle.yaml")
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("bundle file not found: %v", err)
	}
	var parsed JoinBundle
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("bundle YAML parse error: %v", err)
	}
	if parsed.APIVersion != "cbweb3/v1" {
		t.Errorf("parsed apiVersion = %q; want cbweb3/v1", parsed.APIVersion)
	}
}

func TestEmitBundle_EnodeUsesAdvertisedHost(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-usd", "spoke.central-bank.example", 31304)
	ep := &staticEnodeProvider{enode: "enode://deadbeef@172.17.0.2:30303"}

	jb, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "enode://deadbeef@spoke.central-bank.example:31304"
	if jb.Spec.Bootnode.Enode != want {
		t.Errorf("enode = %q; want %q", jb.Spec.Bootnode.Enode, want)
	}
	if strings.Contains(jb.Spec.Bootnode.Enode, "172.17") {
		t.Error("enode should not contain internal container IP 172.17.x.x")
	}
}

func TestEmitBundle_GenesisHashMatchesSHA256(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	genesisContent := `{"config":{"chainId":1337},"alloc":{}}`
	genesisDir := filepath.Join(dir, "genesis")
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(genesisDir, 0o755)
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(genesisDir, "genesis.json"), []byte(genesisContent), 0o644)
	caCert := "-----BEGIN CERTIFICATE-----\nMIIBIjANBg==\n-----END CERTIFICATE-----\n"
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte(caCert), 0o644)
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(`REGISTRY_CONTRACT_ADDRESS=0xA
ZETO_FACTORY_ADDRESS=0xB
PENTE_FACTORY_ADDRESS=0xC
ZETO_TOKEN_ADDRESS=0xD
PENTE_CONTEXT_GROUP_ID=0xE
PENTE_CONTEXT_ADDRESS=0xF
FX_AGREEMENT_DEPLOYED_AT=0x1
`), 0o644)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	jb, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SC-005: verify exact SHA-256 hash value against the known genesis content
	sum := sha256.Sum256([]byte(genesisContent))
	expectedHash := fmt.Sprintf("sha256:%x", sum)
	if jb.Spec.Genesis.Hash != expectedHash {
		t.Errorf("genesis.hash = %q; want %q", jb.Spec.Genesis.Hash, expectedHash)
	}
}

func TestEmitBundle_Reemission_Succeeds(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}
	in := BundleInput{Manifest: m, DataDir: dir, OutputDir: outputDir, EnodeProvider: ep}

	_, err := EmitBundle(context.Background(), in)
	if err != nil {
		t.Fatalf("first emission error: %v", err)
	}
	_, err = EmitBundle(context.Background(), in)
	if err != nil {
		t.Fatalf("second emission error: %v", err)
	}
	bundlePath := filepath.Join(outputDir, "bundles", "spoke-brl.bundle.yaml")
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("bundle file not found after re-emission: %v", err)
	}
}

func TestEmitBundle_OutputDirCreatedAutomatically(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "nonexistent", "nested")
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bundlePath := filepath.Join(outputDir, "bundles", "spoke-brl.bundle.yaml")
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("bundle file not created in nested outputDir: %v", err)
	}
}

// --- Fail-fast tests (T019) ---

func TestEmitBundle_GenesisAbsent(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0o755)
	caCert := "-----BEGIN CERTIFICATE-----\nMIIBIjANBg==\n-----END CERTIFICATE-----\n"
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte(caCert), 0o644)
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xA\nZETO_FACTORY_ADDRESS=0xB\nPENTE_FACTORY_ADDRESS=0xC\nZETO_TOKEN_ADDRESS=0xD\nPENTE_CONTEXT_GROUP_ID=0xE\nPENTE_CONTEXT_ADDRESS=0xF\nFX_AGREEMENT_DEPLOYED_AT=0x1\n"), 0o644)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if !errors.Is(err, ErrGenesisNotFound) {
		t.Errorf("expected ErrGenesisNotFound, got %v", err)
	}
	assertNoBundleFile(t, outputDir)
}

func TestEmitBundle_CACertAbsent(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	os.MkdirAll(genesisDir, 0o755)
	os.WriteFile(filepath.Join(genesisDir, "genesis.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xA\nZETO_FACTORY_ADDRESS=0xB\nPENTE_FACTORY_ADDRESS=0xC\nZETO_TOKEN_ADDRESS=0xD\nPENTE_CONTEXT_GROUP_ID=0xE\nPENTE_CONTEXT_ADDRESS=0xF\nFX_AGREEMENT_DEPLOYED_AT=0x1\n"), 0o644)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if !errors.Is(err, ErrCACertNotFound) {
		t.Errorf("expected ErrCACertNotFound, got %v", err)
	}
	assertNoBundleFile(t, outputDir)
}

func TestEmitBundle_CACertWithPrivateKey(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(genesisDir, 0o755)
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(genesisDir, "genesis.json"), []byte(`{}`), 0o644)
	evil := "-----BEGIN CERTIFICATE-----\nMIIBIjANBg==\n-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----\nMIIEvg==\n-----END PRIVATE KEY-----\n"
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte(evil), 0o644)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if !errors.Is(err, ErrCACertNotFound) {
		t.Errorf("expected ErrCACertNotFound for private key material, got %v", err)
	}
	if !strings.Contains(err.Error(), "private key material") {
		t.Errorf("error should mention private key material, got %q", err.Error())
	}
	assertNoBundleFile(t, outputDir)
}

func TestEmitBundle_DeployedAddrsEmptyKey(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(genesisDir, 0o755)
	os.MkdirAll(tlsDir, 0o755)
	os.WriteFile(filepath.Join(genesisDir, "genesis.json"), []byte(`{}`), 0o644)
	caCert := "-----BEGIN CERTIFICATE-----\nMIIBIjANBg==\n-----END CERTIFICATE-----\n"
	os.WriteFile(filepath.Join(tlsDir, "central-bank.crt"), []byte(caCert), 0o644)
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=\nZETO_FACTORY_ADDRESS=0xB\n"), 0o644)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if !errors.Is(err, ErrDeployedAddrsIncomplete) {
		t.Errorf("expected ErrDeployedAddrsIncomplete, got %v", err)
	}
	if !strings.Contains(err.Error(), "REGISTRY_CONTRACT_ADDRESS") {
		t.Errorf("error should mention key name, got %q", err.Error())
	}
	assertNoBundleFile(t, outputDir)
}

func TestEmitBundle_EnodeProviderError(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{err: ErrEnodeUnavailable}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable, got %v", err)
	}
	assertNoBundleFile(t, outputDir)
}

func TestEmitBundle_MalformedEnode(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "not-an-enode"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if !errors.Is(err, ErrEnodeUnavailable) {
		t.Errorf("expected ErrEnodeUnavailable for malformed enode, got %v", err)
	}
	assertNoBundleFile(t, outputDir)
}

// --- Invalid BundleInput tests (T020) ---

func TestEmitBundle_NilManifest(t *testing.T) {
	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      nil,
		DataDir:       "/some/dir",
		OutputDir:     t.TempDir(),
		EnodeProvider: &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil Manifest, got %v", err)
	}
}

func TestEmitBundle_EmptyDataDir(t *testing.T) {
	m := makeManifest("spoke-brl", "bank.local", 31303)
	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       "",
		OutputDir:     t.TempDir(),
		EnodeProvider: &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty DataDir, got %v", err)
	}
}

func TestEmitBundle_NilEnodeProvider(t *testing.T) {
	m := makeManifest("spoke-brl", "bank.local", 31303)
	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       "/some/dir",
		OutputDir:     t.TempDir(),
		EnodeProvider: nil,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil EnodeProvider, got %v", err)
	}
}

func TestEmitBundle_NilP2P(t *testing.T) {
	m := makeManifest("spoke-brl", "bank.local", 31303)
	m.Spec.Node.P2P = nil
	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       "/some/dir",
		OutputDir:     t.TempDir(),
		EnodeProvider: &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil P2P, got %v", err)
	}
}

func TestEmitBundle_ZeroP2PPort(t *testing.T) {
	m := makeManifest("spoke-brl", "bank.local", 0)
	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       "/some/dir",
		OutputDir:     t.TempDir(),
		EnodeProvider: &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for p2p.port == 0, got %v", err)
	}
}

// --- Context cancellation test (T022) ---

func TestEmitBundle_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &ctxAwareProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(ctx, BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	assertNoBundleFile(t, outputDir)
}

// --- Mode guard tests (T024) ---

func TestEmitBundle_ModeJoin_Rejected(t *testing.T) {
	m := makeManifest("spoke-brl", "bank.local", 31303)
	m.Spec.Mode = "join"
	outputDir := t.TempDir()

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       "/some/dir",
		OutputDir:     outputDir,
		EnodeProvider: &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"},
	})
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode for mode: join, got %v", err)
	}
	if !strings.Contains(err.Error(), "join") {
		t.Errorf("error should mention 'join', got %q", err.Error())
	}
	assertNoBundleFile(t, outputDir)
}

func TestEmitBundle_ModeEmpty_Rejected(t *testing.T) {
	m := makeManifest("spoke-brl", "bank.local", 31303)
	m.Spec.Mode = ""
	outputDir := t.TempDir()

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       "/some/dir",
		OutputDir:     outputDir,
		EnodeProvider: &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"},
	})
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode for empty mode, got %v", err)
	}
	assertNoBundleFile(t, outputDir)
}

func TestEmitBundle_ModeFound_Passes(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Errorf("mode: found should pass guard, got %v", err)
	}
}

// --- Security invariant tests (T030) ---

func TestEmitBundle_NoPrivateKeyInYAML(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	_, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bundlePath := filepath.Join(outputDir, "bundles", "spoke-brl.bundle.yaml")
	data, _ := os.ReadFile(bundlePath)
	yamlStr := string(data)

	if strings.Contains(yamlStr, "PRIVATE KEY") {
		t.Error("SC-006 violation: bundle YAML contains 'PRIVATE KEY'")
	}
	if !strings.Contains(yamlStr, "-----BEGIN CERTIFICATE-----") {
		t.Error("SC-002: bundle should contain BEGIN CERTIFICATE")
	}
	if strings.Contains(jb_enode(yamlStr), "0.0.0.0") {
		t.Error("SC-004: enode should not contain 0.0.0.0 (internal address)")
	}
}

// --- Relay nil test (T033) ---

func TestEmitBundle_RelayNil_FieldOmitted(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	m.Spec.Relay = nil // no relay
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}

	jb, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      m,
		DataDir:       dir,
		OutputDir:     outputDir,
		EnodeProvider: ep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jb.Spec.Relay != nil {
		t.Error("Relay should be nil when manifest.spec.relay is nil")
	}

	// Verify YAML omits relay field
	bundlePath := filepath.Join(outputDir, "bundles", "spoke-brl.bundle.yaml")
	data, _ := os.ReadFile(bundlePath)
	var parsed JoinBundle
	yaml.Unmarshal(data, &parsed)
	if parsed.Spec.Relay != nil {
		t.Error("parsed YAML should have nil Relay when manifest.spec.relay is nil (omitempty)")
	}
}

// --- SC-008 field-by-field re-emission test (T035) ---

func TestEmitBundle_Reemission_FieldByField(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()
	populateDataDir(t, dir)

	m := makeManifest("spoke-brl", "bank.local", 31303)
	ep := &staticEnodeProvider{enode: "enode://abc@0.0.0.0:30303"}
	in := BundleInput{Manifest: m, DataDir: dir, OutputDir: outputDir, EnodeProvider: ep}

	jb1, err := EmitBundle(context.Background(), in)
	if err != nil {
		t.Fatalf("first emission error: %v", err)
	}
	jb2, err := EmitBundle(context.Background(), in)
	if err != nil {
		t.Fatalf("second emission error: %v", err)
	}

	// Zero out generatedAt before comparison
	jb1.Metadata.GeneratedAt = ""
	jb2.Metadata.GeneratedAt = ""

	if jb1.APIVersion != jb2.APIVersion {
		t.Errorf("APIVersion differs: %q vs %q", jb1.APIVersion, jb2.APIVersion)
	}
	if jb1.Kind != jb2.Kind {
		t.Errorf("Kind differs: %q vs %q", jb1.Kind, jb2.Kind)
	}
	if jb1.Metadata.Name != jb2.Metadata.Name {
		t.Errorf("Metadata.Name differs: %q vs %q", jb1.Metadata.Name, jb2.Metadata.Name)
	}
	if jb1.Spec.Bootnode.Enode != jb2.Spec.Bootnode.Enode {
		t.Errorf("Enode differs: %q vs %q", jb1.Spec.Bootnode.Enode, jb2.Spec.Bootnode.Enode)
	}
	if jb1.Spec.Genesis.Hash != jb2.Spec.Genesis.Hash {
		t.Errorf("Genesis.Hash differs: %q vs %q", jb1.Spec.Genesis.Hash, jb2.Spec.Genesis.Hash)
	}
	if jb1.Spec.Genesis.Content != jb2.Spec.Genesis.Content {
		t.Errorf("Genesis.Content differs")
	}
	if jb1.Spec.Contracts.RegistryAddress != jb2.Spec.Contracts.RegistryAddress {
		t.Errorf("RegistryAddress differs")
	}
	if jb1.Spec.Trust.CACertPEM != jb2.Spec.Trust.CACertPEM {
		t.Errorf("CACertPEM differs")
	}
}

// --- helpers ---

func assertNoBundleFile(t *testing.T, outputDir string) {
	t.Helper()
	bundlesDir := filepath.Join(outputDir, "bundles")
	entries, _ := os.ReadDir(bundlesDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bundle.yaml") {
			t.Errorf("bundle file should not exist after error, found: %s", e.Name())
		}
	}
}

// jb_enode extracts the enode line from YAML text (simple grep for test purposes).
func jb_enode(yaml string) string {
	for _, line := range strings.Split(yaml, "\n") {
		if strings.Contains(line, "enode:") {
			return line
		}
	}
	return ""
}
