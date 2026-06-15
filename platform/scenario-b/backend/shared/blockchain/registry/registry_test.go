// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// hexEncodeKey returns the hex-encoded 32-byte secp256k1 private key.
func hexEncodeKey(key *ecdsa.PrivateKey) string {
	return hex.EncodeToString(gethcrypto.FromECDSA(key))
}

func TestRoleToSolidityEnum(t *testing.T) {
	cases := map[string]uint8{
		"ROLE_COMMERCIAL_BANK": RoleCommercialBank,
		"role_commercial_bank": RoleCommercialBank, // case-insensitive
		"  ROLE_TREASURY  ":    RoleTreasury,       // trimmed
		"ROLE_GOVERNANCE":      RoleGovernance,
		"ROLE_CENTRAL_BANK":    RoleCentralBank,
		"ROLE_UNKNOWN":         RoleNone,
		"":                     RoleNone,
	}
	for in, want := range cases {
		if got := RoleToSolidityEnum(in); got != want {
			t.Errorf("RoleToSolidityEnum(%q) = %d, want %d", in, got, want)
		}
	}
}

// NoopRegistryClient must satisfy the full read+write interface set and return
// safe defaults so local/dev environments work without a Besu node.
func TestNoopRegistryClient(t *testing.T) {
	ctx := context.Background()
	var c NoopRegistryClient

	zeroHash := "0x0000000000000000000000000000000000000000000000000000000000000000"

	if h, err := c.RegisterParticipant(ctx, "0x1", "n", "r", [32]byte{}); err != nil || h != zeroHash {
		t.Errorf("RegisterParticipant: %q %v", h, err)
	}
	if h, err := c.UpdateStatus(ctx, "0x1", 2); err != nil || h != zeroHash {
		t.Errorf("UpdateStatus: %q %v", h, err)
	}
	if h, err := c.SetCertFingerprint(ctx, "0x1", [32]byte{}); err != nil || h != zeroHash {
		t.Errorf("SetCertFingerprint: %q %v", h, err)
	}
	if ok, err := c.CanTransact(ctx, "0x1"); err != nil || !ok {
		t.Errorf("CanTransact: %v %v", ok, err)
	}
	if ok, err := c.IsWhitelisted(ctx, "0x1"); err != nil || !ok {
		t.Errorf("IsWhitelisted: %v %v", ok, err)
	}
	p, err := c.GetParticipant(ctx, "0x1")
	if err != nil || p.Status != KycStatusVerified || p.Role != RoleNone || p.LastUpdate.Sign() != 0 {
		t.Errorf("GetParticipant: %+v %v", p, err)
	}
	if fp, err := c.GetCertFingerprint(ctx, "0x1"); err != nil || fp != ([32]byte{}) {
		t.Errorf("GetCertFingerprint: %v %v", fp, err)
	}

	// Compile-time assertion that Noop implements both interfaces.
	var _ RegistryWriter = c
	var _ RegistryReader = c
}

// ---------------------------------------------------------------------------
// StaticKeySigner
// ---------------------------------------------------------------------------

func TestNewStaticKeySigner(t *testing.T) {
	key, _ := gethcrypto.GenerateKey()
	privHex := hexEncodeKey(key)

	t.Run("success_with_0x_prefix", func(t *testing.T) {
		s, err := NewStaticKeySigner("0x" + privHex)
		if err != nil {
			t.Fatalf("NewStaticKeySigner: %v", err)
		}
		addr, err := s.SignerAddress(context.Background())
		if err != nil {
			t.Fatalf("SignerAddress: %v", err)
		}
		want := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
		if addr != want {
			t.Fatalf("addr=%q want %q", addr, want)
		}
	})

	t.Run("success_without_prefix", func(t *testing.T) {
		if _, err := NewStaticKeySigner(privHex); err != nil {
			t.Fatalf("NewStaticKeySigner: %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := NewStaticKeySigner("   "); err == nil {
			t.Fatal("expected error for empty key")
		}
	})

	t.Run("bad_hex", func(t *testing.T) {
		if _, err := NewStaticKeySigner("zzzz"); err == nil {
			t.Fatal("expected error for bad hex")
		}
	})

	t.Run("invalid_key_bytes", func(t *testing.T) {
		// Valid hex but not a valid secp256k1 scalar (all zeros).
		if _, err := NewStaticKeySigner("00"); err == nil {
			t.Fatal("expected error for invalid key bytes")
		}
	})
}

func TestStaticKeySigner_SignTx(t *testing.T) {
	key, _ := gethcrypto.GenerateKey()
	s, err := NewStaticKeySigner(hexEncodeKey(key))
	if err != nil {
		t.Fatalf("NewStaticKeySigner: %v", err)
	}
	chainID := big.NewInt(1337)
	tx := types.NewTransaction(0, gethcrypto.PubkeyToAddress(key.PublicKey), big.NewInt(0), 21000, big.NewInt(1), nil)

	signed, err := s.SignTx(context.Background(), tx, chainID)
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}
	// Recover the sender via the EIP155 signer and confirm it matches.
	from, err := types.Sender(types.NewEIP155Signer(chainID), signed)
	if err != nil {
		t.Fatalf("Sender: %v", err)
	}
	if from != gethcrypto.PubkeyToAddress(key.PublicKey) {
		t.Fatalf("recovered sender mismatch: %s", from.Hex())
	}
}

// ErrNoSigner is exported for callers; ensure it carries a message.
func TestErrNoSigner(t *testing.T) {
	if ErrNoSigner == nil || ErrNoSigner.Error() == "" {
		t.Fatal("ErrNoSigner must be a non-empty error")
	}
}

// NewBesuClient's config validation runs before any network dial and is cheap
// to cover; the dial/bind paths require a live Besu node (not hermetic).
func TestNewBesuClient_ConfigValidation(t *testing.T) {
	t.Run("missing_rpc_url", func(t *testing.T) {
		if _, err := NewBesuClient(BesuConfig{RegistryAddress: "0x1"}, nil); err == nil {
			t.Fatal("expected error for missing RPC URL")
		}
	})
	t.Run("missing_registry_address", func(t *testing.T) {
		if _, err := NewBesuClient(BesuConfig{RPCURL: "http://localhost:8545"}, nil); err == nil {
			t.Fatal("expected error for missing registry address")
		}
	})
}
