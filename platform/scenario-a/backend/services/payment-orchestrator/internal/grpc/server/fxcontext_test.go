// SPDX-License-Identifier: Apache-2.0

package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFXContextStore_Resolve(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fx-contexts.json")
	body := `[
	  {"spoke_id":"spoke-brl","group_id":"0xG1","contract_address":"0xFXA1","bank_identity":"funded_operator@spoke-brl-bank-itau","cb_identity":"funded_operator@spoke-brl-cb"},
	  {"spoke_id":"spoke-cop","group_id":"0xG2","contract_address":"0xFXA2","bank_identity":"funded_operator@spoke-cop-bank-bancolombia","cb_identity":"funded_operator@spoke-cop-cb"}
	]`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newFXContextStore(path)

	// Matches by the bank identity among the record's parties.
	c, ok := s.resolve("funded_operator@spoke-cop-bank-bancolombia", "other")
	if !ok || c.GroupID != "0xG2" || c.ContractAddress != "0xFXA2" {
		t.Fatalf("resolve bancolombia = %+v ok=%v", c, ok)
	}
	// First matching identity wins; unknown identities are skipped.
	c, ok = s.resolve("nope@x", "funded_operator@spoke-brl-bank-itau")
	if !ok || c.GroupID != "0xG1" {
		t.Fatalf("resolve itau = %+v ok=%v", c, ok)
	}
	// No match.
	if _, ok := s.resolve("unknown@z"); ok {
		t.Error("expected no match for unknown identity")
	}
	// Empty path / missing file → no match, no panic.
	if _, ok := newFXContextStore("").resolve("x"); ok {
		t.Error("empty path should not resolve")
	}
	if _, ok := newFXContextStore(filepath.Join(dir, "missing.json")).resolve("x"); ok {
		t.Error("missing file should not resolve")
	}
}
