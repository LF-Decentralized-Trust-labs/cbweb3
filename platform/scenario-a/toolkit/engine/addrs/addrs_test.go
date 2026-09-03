// SPDX-License-Identifier: Apache-2.0

package addrs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDeployedAddrs_FiatAndHTLC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".deployed-addrs.env")
	content := "REGISTRY_CONTRACT_ADDRESS=0xREG\n" +
		"FIAT_TOKEN_ADDRESS=0xF1A7\n" +
		"HTLC_ADDRESS=0x47C\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ParseDeployedAddrs(path)
	if err != nil {
		t.Fatalf("ParseDeployedAddrs: %v", err)
	}
	if got.FiatTokenAddress != "0xF1A7" {
		t.Errorf("FiatTokenAddress = %q, want 0xF1A7", got.FiatTokenAddress)
	}
	if got.HTLCAddress != "0x47C" {
		t.Errorf("HTLCAddress = %q, want 0x47C", got.HTLCAddress)
	}
}

func TestParseDeployedAddrs_MissingFiatAndHTLC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".deployed-addrs.env")
	if err := os.WriteFile(path, []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ParseDeployedAddrs(path)
	if err != nil {
		t.Fatalf("ParseDeployedAddrs: %v", err)
	}
	if got.FiatTokenAddress != "" || got.HTLCAddress != "" {
		t.Errorf("expected empty Fiat/HTLC, got %q / %q", got.FiatTokenAddress, got.HTLCAddress)
	}
}
