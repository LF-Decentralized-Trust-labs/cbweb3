// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGenCSRStep_ProducesKeyAndCSR(t *testing.T) {
	dir := t.TempDir()
	step := newGenCSRStep("commercial-bank-alpha", "Alpha Bank", dir)

	done, err := step.Check(context.Background())
	if err != nil || done {
		t.Fatalf("Check before Run: done=%v err=%v, want false,nil", done, err)
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	keyPath := filepath.Join(dir, "pki", "commercial-bank-alpha.key")
	csrPath := filepath.Join(dir, "pki", "commercial-bank-alpha.csr")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf(".key not created: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf(".key mode = %o, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(csrPath); err != nil {
		t.Errorf(".csr not created: %v", err)
	}

	done, err = step.Check(context.Background())
	if err != nil || !done {
		t.Errorf("Check after Run: done=%v err=%v, want true,nil", done, err)
	}
}
