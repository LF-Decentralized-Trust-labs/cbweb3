// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// validJoinBundle returns a minimal bundle that passes ValidateForJoin.
func validJoinBundle() *JoinBundle {
	return &JoinBundle{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata:   BundleMetadata{Name: "spoke-brl"},
		Spec: BundleSpec{
			SpokeID:  "spoke-brl",
			ChainID:  1337,
			Currency: "BRL",
			Bootnode: BootnodeSpec{
				Enode:          "enode://abc@host:31303",
				AdvertisedHost: "host",
				P2PPort:        31303,
			},
			Genesis: GenesisSpec{Hash: "sha256:abc", Content: "eyJ9"},
			Contracts: ContractsSpec{
				RegistryAddress: "0x1234",
			},
			Validators: []ValidatorSpec{
				{Address: "0xCB1A", RPCURL: "http://cb:8645"},
			},
			CBEndpoint: "http://cb:8080/api/v1/credential-request",
		},
	}
}

func TestValidateForJoin_Valid(t *testing.T) {
	if err := ValidateForJoin(validJoinBundle()); err != nil {
		t.Fatalf("expected valid bundle, got error: %v", err)
	}
}

func TestValidateForJoin_EmptyValidators(t *testing.T) {
	b := validJoinBundle()
	b.Spec.Validators = nil
	err := ValidateForJoin(b)
	if err == nil {
		t.Fatal("expected error for empty validators, got nil")
	}
	if !errors.Is(err, ErrBundleInvalid) {
		t.Errorf("expected ErrBundleInvalid, got %v", err)
	}
}

func TestValidateForJoin_EmptyCBEndpoint(t *testing.T) {
	b := validJoinBundle()
	b.Spec.CBEndpoint = ""
	err := ValidateForJoin(b)
	if err == nil {
		t.Fatal("expected error for empty cbEndpoint, got nil")
	}
	if !errors.Is(err, ErrBundleInvalid) {
		t.Errorf("expected ErrBundleInvalid, got %v", err)
	}
}

func TestValidateForJoin_ValidatorMissingFields(t *testing.T) {
	b := validJoinBundle()
	b.Spec.Validators = []ValidatorSpec{{Address: "", RPCURL: ""}}
	err := ValidateForJoin(b)
	if err == nil {
		t.Fatal("expected error for validator with empty fields, got nil")
	}
}

func TestLoadBundle_NotFound(t *testing.T) {
	_, err := LoadBundle(filepath.Join(t.TempDir(), "missing.yaml"))
	if !errors.Is(err, ErrBundleNotFound) {
		t.Errorf("expected ErrBundleNotFound, got %v", err)
	}
}

func TestLoadBundle_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spoke-brl.bundle.yaml")
	if err := writeBundleAtomic(dir, "spoke-brl", validJoinBundle()); err != nil {
		t.Fatalf("writeBundleAtomic: %v", err)
	}
	// writeBundleAtomic writes to dir/bundles/<spoke>.bundle.yaml
	loaded, err := LoadBundle(filepath.Join(dir, "bundles", "spoke-brl.bundle.yaml"))
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	if len(loaded.Spec.Validators) != 1 || loaded.Spec.Validators[0].Address != "0xCB1A" {
		t.Errorf("validators not round-tripped: %+v", loaded.Spec.Validators)
	}
	if loaded.Spec.CBEndpoint != "http://cb:8080/api/v1/credential-request" {
		t.Errorf("cbEndpoint not round-tripped: %q", loaded.Spec.CBEndpoint)
	}
	_ = path
	_ = os.Remove
}
