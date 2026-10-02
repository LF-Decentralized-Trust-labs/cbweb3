// SPDX-License-Identifier: Apache-2.0

package addrs

import (
	"os"
	"path/filepath"
	"testing"
)

// A deployed-addrs file written before the Scenario A AMM was retired still carries
// AMM_ADDRESS. Parsing must ignore the key, not reject the file: an operator's existing
// state must keep loading after an upgrade.
func TestParseDeployedAddrs_IgnoresRetiredAMMAddress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".deployed-addrs.env")
	content := "HTLC_ADDRESS=0xhtlc\nAMM_ADDRESS=0xdeadbeef\nFIAT_TOKEN_ADDRESS=0xfiat\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	got, err := ParseDeployedAddrs(path)
	if err != nil {
		t.Fatalf("a file holding the retired key must parse, got error: %v", err)
	}
	if got.HTLCAddress != "0xhtlc" || got.FiatTokenAddress != "0xfiat" {
		t.Errorf("surrounding keys were dropped: %+v", got)
	}
}
