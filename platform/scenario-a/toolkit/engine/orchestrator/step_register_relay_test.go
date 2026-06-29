// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// captureRelayRegistrar records Register calls for assertion.
type captureRelayRegistrar struct {
	registered  bool
	lastInfo    SpokeInfo
	registerErr error
}

func (r *captureRelayRegistrar) Register(_ context.Context, info SpokeInfo) error {
	r.registered = true
	r.lastInfo = info
	return r.registerErr
}
func (r *captureRelayRegistrar) IsRegistered(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func TestRegisterRelayStep_Check_False_NotRegistered(t *testing.T) {
	dir := t.TempDir()
	reg := &captureRelayRegistrar{}
	step := newRegisterRelayStep("spoke-test", dir, "http://localhost:8645", reg, 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when spoke is not registered")
	}
}

// alreadyRegisteredRelayRegistrar simulates a relay that already knows this spoke.
type alreadyRegisteredRelayRegistrar struct{}

func (alreadyRegisteredRelayRegistrar) Register(_ context.Context, _ SpokeInfo) error {
	return nil
}
func (alreadyRegisteredRelayRegistrar) IsRegistered(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func TestRegisterRelayStep_Check_True_AlreadyRegistered(t *testing.T) {
	dir := t.TempDir()
	step := newRegisterRelayStep("spoke-test", dir, "http://localhost:8645", alreadyRegisteredRelayRegistrar{}, 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when spoke is already registered")
	}
}

// unavailableRelayRegistrar simulates a relay that is unreachable.
type unavailableRelayRegistrar struct{}

func (unavailableRelayRegistrar) Register(_ context.Context, _ SpokeInfo) error {
	return ErrRelayUnavailable
}
func (unavailableRelayRegistrar) IsRegistered(_ context.Context, _ string) (bool, error) {
	return false, ErrRelayUnavailable
}

func TestRegisterRelayStep_Check_False_RelayUnavailable(t *testing.T) {
	dir := t.TempDir()
	step := newRegisterRelayStep("spoke-test", dir, "http://localhost:8645", unavailableRelayRegistrar{}, 0)
	// IsRegistered returns ErrRelayUnavailable → check returns false, nil (soft fallback).
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("check should not return error on relay unavailability; got: %v", err)
	}
	if done {
		t.Error("check should return false when relay is unavailable")
	}
}

func TestRegisterRelayStep_Run_PopulatesSpokeID(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	reg := &captureRelayRegistrar{}
	step := newRegisterRelayStep("spoke-test", dir, "http://localhost:8645", reg, 0)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reg.registered {
		t.Fatal("Register was not called")
	}
	if reg.lastInfo.SpokeID != "spoke-test" {
		t.Errorf("SpokeID = %q; want spoke-test", reg.lastInfo.SpokeID)
	}
}

func TestRegisterRelayStep_Run_PopulatesBesuRPCURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	reg := &captureRelayRegistrar{}
	step := newRegisterRelayStep("spoke-test", dir, "http://besu-rpc:8645", reg, 0)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reg.lastInfo.BesuRPCURL != "http://besu-rpc:8645" {
		t.Errorf("BesuRPCURL = %q; want http://besu-rpc:8645", reg.lastInfo.BesuRPCURL)
	}
}

func TestRegisterRelayStep_Run_HTLCAddressFromDeployedAddrs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREGISTRY\n"), 0o644)
	reg := &captureRelayRegistrar{}
	step := newRegisterRelayStep("spoke-test", dir, "http://localhost:8645", reg, 0)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reg.lastInfo.HTLCAddress != "0xREGISTRY" {
		t.Errorf("HTLCAddress = %q; want 0xREGISTRY", reg.lastInfo.HTLCAddress)
	}
}

func TestRegisterRelayStep_Run_PropagatesRegisterError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	reg := &captureRelayRegistrar{registerErr: errors.New("relay rejected")}
	step := newRegisterRelayStep("spoke-test", dir, "http://localhost:8645", reg, 0)
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should propagate error from Register")
	}
}
