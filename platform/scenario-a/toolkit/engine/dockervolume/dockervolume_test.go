// SPDX-License-Identifier: Apache-2.0

package dockervolume

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// requireDocker skips the test if the docker CLI is not reachable — these tests
// exercise real `docker run` against a throwaway volume, mirroring how the
// migrated steps behave in production (no host filesystem involved).
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

func TestWriteFile_ReadFile_RoundTrip(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	volume := "cbweb3_test_dockervolume_roundtrip"
	cleanupVolume(t, volume)

	content := []byte("nodeName: spoke-brl-cb\n")
	if err := WriteFile(ctx, volume, "config.yaml", content, "0644"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadFile(ctx, volume, "config.yaml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q; want %q", got, content)
	}
}

func TestWriteFile_NestedPath(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	volume := "cbweb3_test_dockervolume_nested"
	cleanupVolume(t, volume)

	if err := WriteFile(ctx, volume, "sub/dir/tls.key", []byte("fake-key"), "0600"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := ReadFile(ctx, volume, "sub/dir/tls.key")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "fake-key" {
		t.Errorf("content = %q; want fake-key", got)
	}
}

func TestFileExists_TrueAfterWrite(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	volume := "cbweb3_test_dockervolume_exists"
	cleanupVolume(t, volume)

	exists, err := FileExists(ctx, volume, "genesis.json")
	if err != nil {
		t.Fatalf("FileExists (before write): %v", err)
	}
	if exists {
		t.Error("expected false before write (volume auto-created empty)")
	}

	if err := WriteFile(ctx, volume, "genesis.json", []byte("{}"), ""); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	exists, err = FileExists(ctx, volume, "genesis.json")
	if err != nil {
		t.Fatalf("FileExists (after write): %v", err)
	}
	if !exists {
		t.Error("expected true after write")
	}
}

func TestReadFile_NotFound(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	volume := "cbweb3_test_dockervolume_notfound"
	cleanupVolume(t, volume)

	_, err := ReadFile(ctx, volume, "nope.txt")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v; want ErrNotFound", err)
	}
}
