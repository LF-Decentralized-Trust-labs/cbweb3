// SPDX-License-Identifier: Apache-2.0

package keyprovider

import (
	"context"
	"testing"
)

// SC-007: kms://local-emulator resolves to the local provider.
func TestFactoryLocalEmulator(t *testing.T) {
	kp, err := New("kms://local-emulator")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := kp.(LocalKeyExporter); !ok {
		t.Fatal("local emulator must implement LocalKeyExporter")
	}
	if _, err := kp.GenerateKey(context.Background(), "x"); err != nil {
		t.Fatalf("local GenerateKey: %v", err)
	}
}

func TestFactoryLocalEmulatorWithSeed(t *testing.T) {
	kp, err := New("kms://local-emulator?seed=abc")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := kp.GenerateKey(context.Background(), "x"); err != nil {
		t.Fatalf("seeded local GenerateKey: %v", err)
	}
}

// SC-003 / SC-007: any other kms:// resolves to the prod stub.
func TestFactoryProdStub(t *testing.T) {
	kp, err := New("kms://aws-kms-prod")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := kp.(LocalKeyExporter); ok {
		t.Fatal("prod stub must NOT implement LocalKeyExporter")
	}
	if _, err := kp.GenerateKey(context.Background(), "x"); err != ErrNotImplemented {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

// SC-007: unsupported URI → configuration error.
func TestFactoryUnsupportedURI(t *testing.T) {
	if _, err := New("vault://foo"); err != ErrUnsupportedURI {
		t.Fatalf("expected ErrUnsupportedURI, got %v", err)
	}
}
