// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// seedFoundDataDir writes the minimal artifacts EmitBundle reads.
func seedFoundDataDir(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, "genesis"), 0o755)
	os.MkdirAll(filepath.Join(dir, "tls"), 0o755)
	os.WriteFile(filepath.Join(dir, "genesis", "genesis.json"), []byte(`{"config":{}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "tls", "central-bank.crt"),
		[]byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"), 0o644)
	addrsEnv := "REGISTRY_CONTRACT_ADDRESS=0xR\nZETO_FACTORY_ADDRESS=0xZF\nPENTE_FACTORY_ADDRESS=0xPF\n" +
		"ZETO_TOKEN_ADDRESS=0xZT\nPENTE_CONTEXT_GROUP_ID=0xPG\nPENTE_CONTEXT_ADDRESS=0xPA\nFX_AGREEMENT_DEPLOYED_AT=0xFX\n"
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(addrsEnv), 0o644)
}

func foundManifest() *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: "cbweb3/v1",
		Kind:       "ParticipantDeployment",
		Metadata:   manifest.Metadata{Name: "cb"},
		Spec: manifest.Spec{
			Scenario: "a",
			Mode:     "found",
			Spoke:    manifest.Spoke{ID: "spoke-brl", ChainID: 1337, Currency: "BRL"},
			Node: manifest.Node{
				AdvertisedHost: "cb.host",
				RPC:            &manifest.Port{Port: 8645},
				P2P:            &manifest.Port{Port: 31303},
			},
		},
	}
}

// T043: explicit Validators and CBEndpoint are embedded in the emitted bundle.
func TestEmitBundle_EmbedsExplicitValidatorsAndCBEndpoint(t *testing.T) {
	dir, out := t.TempDir(), t.TempDir()
	seedFoundDataDir(t, dir)

	jb, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      foundManifest(),
		DataDir:       dir,
		OutputDir:     out,
		EnodeProvider: &staticEnodeProvider{enode: "enode://abc@h:1"},
		Validators:    []ValidatorSpec{{Address: "0xCB1A", RPCURL: "http://cb.host:8645"}},
		CBEndpoint:    "http://cb.host:8080/api/v1/credential-request",
	})
	if err != nil {
		t.Fatalf("EmitBundle: %v", err)
	}
	if len(jb.Spec.Validators) != 1 || jb.Spec.Validators[0].Address != "0xCB1A" {
		t.Errorf("validators not embedded: %+v", jb.Spec.Validators)
	}
	if jb.Spec.CBEndpoint != "http://cb.host:8080/api/v1/credential-request" {
		t.Errorf("cbEndpoint not embedded: %q", jb.Spec.CBEndpoint)
	}
}

// T052: a bundle emitted by the found path (with cbEndpoint in the manifest and
// a real enode) satisfies ValidateForJoin — closing the found→join chain.
func TestEmitBundle_EmittedBundleIsJoinValid(t *testing.T) {
	dir, out := t.TempDir(), t.TempDir()
	seedFoundDataDir(t, dir)

	key, _ := crypto.GenerateKey()
	pub := crypto.FromECDSAPub(&key.PublicKey)
	enode := "enode://" + hex.EncodeToString(pub[1:]) + "@10.0.0.9:30303"

	jb, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      foundManifest(),
		DataDir:       dir,
		OutputDir:     out,
		EnodeProvider: &staticEnodeProvider{enode: enode},
		CBEndpoint:    "http://api-gateway-cb:8080/api/v1/credential-request",
	})
	if err != nil {
		t.Fatalf("EmitBundle: %v", err)
	}
	if err := ValidateForJoin(jb); err != nil {
		t.Errorf("emitted bundle must pass ValidateForJoin, got: %v", err)
	}
}

// T042: when no validators are supplied, EmitBundle derives the founding CB
// validator from its own enode.
func TestEmitBundle_DerivesValidatorFromEnode(t *testing.T) {
	dir, out := t.TempDir(), t.TempDir()
	seedFoundDataDir(t, dir)

	key, _ := crypto.GenerateKey()
	pub := crypto.FromECDSAPub(&key.PublicKey)
	enode := "enode://" + hex.EncodeToString(pub[1:]) + "@10.0.0.9:30303"
	wantAddr := crypto.PubkeyToAddress(key.PublicKey).Hex()

	jb, err := EmitBundle(context.Background(), BundleInput{
		Manifest:      foundManifest(),
		DataDir:       dir,
		OutputDir:     out,
		EnodeProvider: &staticEnodeProvider{enode: enode},
	})
	if err != nil {
		t.Fatalf("EmitBundle: %v", err)
	}
	if len(jb.Spec.Validators) != 1 {
		t.Fatalf("expected 1 derived validator, got %d", len(jb.Spec.Validators))
	}
	if jb.Spec.Validators[0].Address != wantAddr {
		t.Errorf("derived validator address = %q, want %q", jb.Spec.Validators[0].Address, wantAddr)
	}
	if jb.Spec.Validators[0].RPCURL != "http://cb.host:8645" {
		t.Errorf("derived validator rpcUrl = %q, want http://cb.host:8645", jb.Spec.Validators[0].RPCURL)
	}
}
