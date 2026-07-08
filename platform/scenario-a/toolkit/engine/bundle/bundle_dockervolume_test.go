// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/dockervolume"
)

// requireDocker skips the test if the docker CLI is not reachable. Only the
// GenesisVolume path needs Docker; every other bundle_test.go test is pure Go
// against the DataDir fallback, unaffected by this file.
func requireDocker(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("docker not available; skipping volume-backed test")
	}
}

func cleanupVolume(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		_ = exec.Command("docker", "volume", "rm", "-f", name).Run()
	})
}

func TestReadGenesis_FromVolume_Success(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	volume := "cbweb3_test_bundle_genesis_success"
	cleanupVolume(t, volume)

	genesisContent := `{"config":{"chainId":1337},"alloc":{}}`
	if err := dockervolume.WriteFile(ctx, volume, "genesis.json", []byte(genesisContent), "0644"); err != nil {
		t.Fatalf("seed volume: %v", err)
	}

	spec, err := readGenesis(ctx, "/dataDir/should/not/be/used", volume)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sum := sha256.Sum256([]byte(genesisContent))
	expectedHash := fmt.Sprintf("sha256:%x", sum)
	if spec.Hash != expectedHash {
		t.Errorf("hash = %q; want %q", spec.Hash, expectedHash)
	}
	decoded, err := base64.StdEncoding.DecodeString(spec.Content)
	if err != nil {
		t.Fatalf("content is not valid base64: %v", err)
	}
	if string(decoded) != genesisContent {
		t.Errorf("decoded content = %q; want %q", string(decoded), genesisContent)
	}
}

func TestReadGenesis_FromVolume_Missing(t *testing.T) {
	requireDocker(t)
	volume := "cbweb3_test_bundle_genesis_missing"
	cleanupVolume(t, volume)

	_, err := readGenesis(context.Background(), "/dataDir/unused", volume)
	if !errors.Is(err, ErrGenesisNotFound) {
		t.Errorf("expected ErrGenesisNotFound, got %v", err)
	}
}

func TestReadCACert_FromVolume_Success(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	volume := "cbweb3_test_bundle_tls_success"
	cleanupVolume(t, volume)

	caCert := "-----BEGIN CERTIFICATE-----\nMIIBIjANBg==\n-----END CERTIFICATE-----\n"
	if err := dockervolume.WriteFile(ctx, volume, "central-bank.crt", []byte(caCert), "0644"); err != nil {
		t.Fatalf("seed volume: %v", err)
	}

	spec, err := readCACert(ctx, "/dataDir/should/not/be/used", volume)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.CACertPEM == "" {
		t.Error("CACertPEM should not be empty")
	}
}

func TestReadCACert_FromVolume_Missing(t *testing.T) {
	requireDocker(t)
	volume := "cbweb3_test_bundle_tls_missing"
	cleanupVolume(t, volume)

	_, err := readCACert(context.Background(), "/dataDir/unused", volume)
	if !errors.Is(err, ErrCACertNotFound) {
		t.Errorf("expected ErrCACertNotFound, got %v", err)
	}
}
