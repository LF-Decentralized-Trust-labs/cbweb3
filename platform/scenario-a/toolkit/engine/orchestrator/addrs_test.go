// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDeployedAddrs_FileNotExist(t *testing.T) {
	addrs, err := parseDeployedAddrs("/nonexistent/path/.deployed-addrs.env")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if addrs.RegistryContractAddress != "" {
		t.Errorf("expected empty RegistryContractAddress, got %q", addrs.RegistryContractAddress)
	}
}

func TestParseDeployedAddrs_WellFormed(t *testing.T) {
	content := `REGISTRY_CONTRACT_ADDRESS=0xAAA
ZETO_FACTORY_ADDRESS=0xBBB
PENTE_FACTORY_ADDRESS=0xCCC
ZETO_TOKEN_ADDRESS=0xDDD
PENTE_CONTEXT_GROUP_ID=0xEEE
PENTE_CONTEXT_ADDRESS=0xFFF
FX_AGREEMENT_DEPLOYED_AT=some-uuid
`
	path := writeEnvFile(t, content)
	addrs, err := parseDeployedAddrs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addrs.RegistryContractAddress != "0xAAA" {
		t.Errorf("RegistryContractAddress = %q; want 0xAAA", addrs.RegistryContractAddress)
	}
	if addrs.ZetoFactoryAddress != "0xBBB" {
		t.Errorf("ZetoFactoryAddress = %q; want 0xBBB", addrs.ZetoFactoryAddress)
	}
	if addrs.PenteFactoryAddress != "0xCCC" {
		t.Errorf("PenteFactoryAddress = %q; want 0xCCC", addrs.PenteFactoryAddress)
	}
	if addrs.ZetoTokenAddress != "0xDDD" {
		t.Errorf("ZetoTokenAddress = %q; want 0xDDD", addrs.ZetoTokenAddress)
	}
	if addrs.PenteContextGroupID != "0xEEE" {
		t.Errorf("PenteContextGroupID = %q; want 0xEEE", addrs.PenteContextGroupID)
	}
	if addrs.FXAgreementDeployedAt != "some-uuid" {
		t.Errorf("FXAgreementDeployedAt = %q; want some-uuid", addrs.FXAgreementDeployedAt)
	}
}

func TestParseDeployedAddrs_MissingKey(t *testing.T) {
	content := "REGISTRY_CONTRACT_ADDRESS=0xAAA\n"
	path := writeEnvFile(t, content)
	addrs, err := parseDeployedAddrs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addrs.RegistryContractAddress != "0xAAA" {
		t.Errorf("RegistryContractAddress = %q; want 0xAAA", addrs.RegistryContractAddress)
	}
	if addrs.ZetoFactoryAddress != "" {
		t.Errorf("ZetoFactoryAddress should be empty, got %q", addrs.ZetoFactoryAddress)
	}
}

func TestParseDeployedAddrs_KeyWithEmptyValue(t *testing.T) {
	content := "REGISTRY_CONTRACT_ADDRESS=\nZETO_FACTORY_ADDRESS=0xBBB\n"
	path := writeEnvFile(t, content)
	addrs, err := parseDeployedAddrs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addrs.RegistryContractAddress != "" {
		t.Errorf("RegistryContractAddress should be empty, got %q", addrs.RegistryContractAddress)
	}
	if addrs.ZetoFactoryAddress != "0xBBB" {
		t.Errorf("ZetoFactoryAddress = %q; want 0xBBB", addrs.ZetoFactoryAddress)
	}
}

func TestParseDeployedAddrs_CommentsAndBlanks(t *testing.T) {
	content := `# This is a comment
REGISTRY_CONTRACT_ADDRESS=0xAAA

# Another comment
ZETO_FACTORY_ADDRESS=0xBBB
`
	path := writeEnvFile(t, content)
	addrs, err := parseDeployedAddrs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addrs.RegistryContractAddress != "0xAAA" {
		t.Errorf("RegistryContractAddress = %q; want 0xAAA", addrs.RegistryContractAddress)
	}
	if addrs.ZetoFactoryAddress != "0xBBB" {
		t.Errorf("ZetoFactoryAddress = %q; want 0xBBB", addrs.ZetoFactoryAddress)
	}
}

// writeEnvFile writes content to a temp file and returns its path.
func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".deployed-addrs.env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}
	return path
}
