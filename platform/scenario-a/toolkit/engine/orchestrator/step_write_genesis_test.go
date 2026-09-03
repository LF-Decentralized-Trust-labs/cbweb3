// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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
	requireDocker(t)
	content, b64, hash := genesisFixture()
	step := &writeGenesisStep{spokeID: "spoke-test", bankID: "bank-write-genesis-new", genesisB64: b64, genesisHash: hash}
	cleanupVolume(t, step.genesisVolume())

	done, err := step.Check(context.Background())
	if err != nil || done {
		t.Fatalf("Check before Run: done=%v err=%v, want false,nil", done, err)
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := readVolumeFile(context.Background(), step.genesisVolume(), "genesis.json")
	if err != nil {
		t.Fatalf("genesis.json not written to volume: %v", err)
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
	requireDocker(t)
	content, b64, hash := genesisFixture()
	step := &writeGenesisStep{spokeID: "spoke-test", bankID: "bank-write-genesis-match", genesisB64: b64, genesisHash: hash}
	cleanupVolume(t, step.genesisVolume())
	if err := writeVolumeFile(context.Background(), step.genesisVolume(), "genesis.json", content, "0644"); err != nil {
		t.Fatal(err)
	}

	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: unexpected error: %v", err)
	}
	if !done {
		t.Error("Check should be true when existing genesis matches bundle hash")
	}
}

func TestWriteGenesisStep_ExistingHashMismatch(t *testing.T) {
	requireDocker(t)
	_, b64, hash := genesisFixture()
	step := &writeGenesisStep{spokeID: "spoke-test", bankID: "bank-write-genesis-mismatch", genesisB64: b64, genesisHash: hash}
	cleanupVolume(t, step.genesisVolume())
	// Write a DIFFERENT genesis into the volume.
	divergent := []byte(`{"config":{"chainId":999}}`)
	if err := writeVolumeFile(context.Background(), step.genesisVolume(), "genesis.json", divergent, "0644"); err != nil {
		t.Fatal(err)
	}

	_, err := step.Check(context.Background())
	if err == nil {
		t.Fatal("Check should error when existing genesis hash diverges (genesis-once invariant)")
	}
	// Ensure the divergent file is NOT overwritten.
	got, _ := readVolumeFile(context.Background(), step.genesisVolume(), "genesis.json")
	if string(got) != string(divergent) {
		t.Error("divergent genesis must not be overwritten")
	}
}
