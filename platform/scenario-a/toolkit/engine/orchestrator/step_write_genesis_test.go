// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func genesisFixture() (content []byte, b64, hash string) {
	content = []byte(`{"config":{"chainId":1337}}`)
	b64 = base64.StdEncoding.EncodeToString(content)
	sum := sha256.Sum256(content)
	hash = fmt.Sprintf("sha256:%x", sum)
	return
}

func TestWriteGenesisStep_NewFile(t *testing.T) {
	dir := t.TempDir()
	content, b64, hash := genesisFixture()
	step := newWriteGenesisStep(dir, b64, hash)

	done, err := step.Check(context.Background())
	if err != nil || done {
		t.Fatalf("Check before Run: done=%v err=%v, want false,nil", done, err)
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "genesis", "genesis.json"))
	if err != nil {
		t.Fatalf("genesis.json not written: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("genesis content mismatch")
	}
	done, err = step.Check(context.Background())
	if err != nil || !done {
		t.Errorf("Check after Run: done=%v err=%v, want true,nil", done, err)
	}
}

func TestWriteGenesisStep_ExistingHashMatch(t *testing.T) {
	dir := t.TempDir()
	content, b64, hash := genesisFixture()
	gdir := filepath.Join(dir, "genesis")
	os.MkdirAll(gdir, 0o755)
	os.WriteFile(filepath.Join(gdir, "genesis.json"), content, 0o644)

	step := newWriteGenesisStep(dir, b64, hash)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: unexpected error: %v", err)
	}
	if !done {
		t.Error("Check should be true when existing genesis matches bundle hash")
	}
}

func TestWriteGenesisStep_ExistingHashMismatch(t *testing.T) {
	dir := t.TempDir()
	_, b64, hash := genesisFixture()
	gdir := filepath.Join(dir, "genesis")
	os.MkdirAll(gdir, 0o755)
	// Write a DIFFERENT genesis.
	os.WriteFile(filepath.Join(gdir, "genesis.json"), []byte(`{"config":{"chainId":999}}`), 0o644)

	step := newWriteGenesisStep(dir, b64, hash)
	_, err := step.Check(context.Background())
	if err == nil {
		t.Fatal("Check should error when existing genesis hash diverges (genesis-once invariant)")
	}
	// Ensure the divergent file is NOT overwritten.
	got, _ := os.ReadFile(filepath.Join(gdir, "genesis.json"))
	if string(got) != `{"config":{"chainId":999}}` {
		t.Error("divergent genesis must not be overwritten")
	}
}
