// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// errKeyProvider is a stub KeyProvider that always returns an error from GetPublicKey.
type errKeyProvider struct{}

func (errKeyProvider) GenerateKey(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("stub: generate not available")
}
func (errKeyProvider) Sign(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, errors.New("stub: sign not available")
}
func (errKeyProvider) GetPublicKey(_ context.Context, _ string) ([]byte, error) {
	return nil, kp.ErrKeyNotFound
}

// notFoundKeyProvider returns ErrKeyNotFound from GetPublicKey; GenerateKey succeeds but
// returns a dummy pubkey that will fail EVMAddress derivation.
type notFoundKeyProvider struct{}

func (notFoundKeyProvider) GenerateKey(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("stub: generate failed")
}
func (notFoundKeyProvider) Sign(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, errors.New("stub: sign failed")
}
func (notFoundKeyProvider) GetPublicKey(_ context.Context, _ string) ([]byte, error) {
	return nil, kp.ErrKeyNotFound
}

func TestOnboardRegistryStep_Check_False_NoRegistryAddr(t *testing.T) {
	dir := t.TempDir()
	// No .deployed-addrs.env → registry address empty → returns false without connecting.
	step := newOnboardRegistryStep("spoke-test", "central-bank-test", dir, "http://localhost:8645", errKeyProvider{}, "", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when registry address is not present")
	}
}

func TestOnboardRegistryStep_Check_False_KeyNotFound(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	// GetPublicKey returns ErrKeyNotFound → check returns false, nil (key not generated yet).
	step := newOnboardRegistryStep("spoke-test", "central-bank-test", dir, "http://localhost:8645", errKeyProvider{}, "", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when key is not found")
	}
}

func TestOnboardRegistryStep_Run_ErrorsWhenGenerateKeyFails(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	// Both GetPublicKey (ErrKeyNotFound) and GenerateKey fail → Run should propagate error.
	step := newOnboardRegistryStep("spoke-test", "central-bank-test", dir, "http://localhost:8645", notFoundKeyProvider{}, "", 0)
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when both GetPublicKey and GenerateKey fail")
	}
}
