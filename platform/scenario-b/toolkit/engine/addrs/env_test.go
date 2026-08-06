// SPDX-License-Identifier: Apache-2.0

package addrs

import (
	"path/filepath"
	"testing"
)

func TestReadAddr(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if err := AppendAddr(p, "W_TOKEN_ADDRESS", "0xABC"); err != nil {
		t.Fatal(err)
	}
	if err := AppendAddr(p, "OTHER", "0xDEF"); err != nil {
		t.Fatal(err)
	}
	if got := ReadAddr(p, "W_TOKEN_ADDRESS"); got != "0xABC" {
		t.Fatalf("ReadAddr = %q, want 0xABC", got)
	}
	// An upsert must be visible to the reader, not the superseded value.
	if err := AppendAddr(p, "W_TOKEN_ADDRESS", "0x123"); err != nil {
		t.Fatal(err)
	}
	if got := ReadAddr(p, "W_TOKEN_ADDRESS"); got != "0x123" {
		t.Fatalf("ReadAddr after upsert = %q, want 0x123", got)
	}
	if got := ReadAddr(p, "MISSING"); got != "" {
		t.Fatalf("absent key must read as empty, got %q", got)
	}
	if got := ReadAddr(filepath.Join(dir, "nope.env"), "ANY"); got != "" {
		t.Fatalf("absent file must read as empty, got %q", got)
	}
}
