// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func hexEncode(b []byte) string { return hex.EncodeToString(b) }

func newLegacyTx() *types.Transaction {
	return types.NewTransaction(0, common.HexToAddress("0x0000000000000000000000000000000000000001"), big.NewInt(0), 21000, big.NewInt(1), nil)
}

// ── RoleToSolidityEnum ────────────────────────────────────────────────────────

func TestRoleToSolidityEnum(t *testing.T) {
	t.Parallel()
	cases := []struct {
		role string
		want uint8
	}{
		{"ROLE_COMMERCIAL_BANK", RoleCommercialBank},
		{"role_commercial_bank", RoleCommercialBank},
		{"  ROLE_TREASURY  ", RoleTreasury},
		{"ROLE_GOVERNANCE", RoleGovernance},
		{"ROLE_CENTRAL_BANK", RoleCentralBank},
		{"ROLE_UNKNOWN", RoleNone},
		{"", RoleNone},
		// Bare form (no ROLE_ prefix) as sent by the onboarding flow — must map to a
		// non-NONE role so IdentityRegistry.canTransact returns true after onboarding.
		{"commercial_bank", RoleCommercialBank},
		{"COMMERCIAL_BANK", RoleCommercialBank},
		{"central_bank", RoleCentralBank},
		{"treasury", RoleTreasury},
		{"governance", RoleGovernance},
		{"unknown", RoleNone},
	}
	for _, tc := range cases {
		if got := RoleToSolidityEnum(tc.role); got != tc.want {
			t.Errorf("RoleToSolidityEnum(%q) = %d, want %d", tc.role, got, tc.want)
		}
	}
}

// ── NoopRegistryClient ────────────────────────────────────────────────────────

func TestNoopRegistryClient(t *testing.T) {
	t.Parallel()
	var c NoopRegistryClient
	ctx := context.Background()
	zeroHash := "0x0000000000000000000000000000000000000000000000000000000000000000"

	if h, err := c.RegisterParticipant(ctx, "0xabc", "Bank", "ROLE_COMMERCIAL_BANK", [32]byte{}, [32]byte{}); err != nil || h != zeroHash {
		t.Errorf("RegisterParticipant = (%q, %v)", h, err)
	}
	if h, err := c.UpdateStatus(ctx, "0xabc", KycStatusVerified); err != nil || h != zeroHash {
		t.Errorf("UpdateStatus = (%q, %v)", h, err)
	}
	if h, err := c.SetCertFingerprint(ctx, "0xabc", [32]byte{}); err != nil || h != zeroHash {
		t.Errorf("SetCertFingerprint = (%q, %v)", h, err)
	}
	if ok, err := c.CanTransact(ctx, "0xabc"); err != nil || !ok {
		t.Errorf("CanTransact = (%v, %v); want true", ok, err)
	}
	if ok, err := c.IsWhitelisted(ctx, "0xabc"); err != nil || !ok {
		t.Errorf("IsWhitelisted = (%v, %v); want true", ok, err)
	}
	p, err := c.GetParticipant(ctx, "0xabc")
	if err != nil || p.Role != RoleNone || p.Status != KycStatusVerified {
		t.Errorf("GetParticipant = (%+v, %v)", p, err)
	}
	if fp, err := c.GetCertFingerprint(ctx, "0xabc"); err != nil || fp != ([32]byte{}) {
		t.Errorf("GetCertFingerprint = (%v, %v)", fp, err)
	}
}

// ── StaticKeySigner ───────────────────────────────────────────────────────────

func TestNewStaticKeySigner(t *testing.T) {
	t.Parallel()

	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	keyHex := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	_ = keyHex
	privHex := hexEncode(gethcrypto.FromECDSA(key))

	t.Run("valid_with_0x_prefix", func(t *testing.T) {
		signer, err := NewStaticKeySigner("0x" + privHex)
		if err != nil {
			t.Fatalf("NewStaticKeySigner: %v", err)
		}
		addr, err := signer.SignerAddress(context.Background())
		if err != nil {
			t.Fatalf("SignerAddress: %v", err)
		}
		want := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
		if addr != want {
			t.Errorf("addr = %q, want %q", addr, want)
		}
	})

	t.Run("valid_without_prefix", func(t *testing.T) {
		if _, err := NewStaticKeySigner(privHex); err != nil {
			t.Fatalf("NewStaticKeySigner: %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := NewStaticKeySigner("   "); err == nil {
			t.Error("expected error for empty key")
		}
	})

	t.Run("bad_hex", func(t *testing.T) {
		if _, err := NewStaticKeySigner("zzzz"); err == nil {
			t.Error("expected error for bad hex")
		}
	})

	t.Run("not_a_valid_ec_key", func(t *testing.T) {
		// 32 bytes of zero is not a valid secp256k1 private key.
		if _, err := NewStaticKeySigner("00"); err == nil {
			t.Error("expected error for invalid EC key")
		}
	})
}

func TestStaticKeySigner_SignTx(t *testing.T) {
	t.Parallel()
	key, _ := gethcrypto.GenerateKey()
	signer, err := NewStaticKeySigner(hexEncode(gethcrypto.FromECDSA(key)))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	tx := newLegacyTx()
	signed, err := signer.SignTx(context.Background(), tx, big.NewInt(1337))
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}
	v, r, s := signed.RawSignatureValues()
	if v == nil || r == nil || s == nil || (r.Sign() == 0 && s.Sign() == 0) {
		t.Error("expected non-empty signature values")
	}
}

// ── NewBesuClient validation (no live node needed for these branches) ─────────

func TestNewBesuClient_Validation(t *testing.T) {
	t.Parallel()
	if _, err := NewBesuClient(BesuConfig{RegistryAddress: "0xabc"}, nil); err == nil {
		t.Error("expected error for missing RPC URL")
	}
	if _, err := NewBesuClient(BesuConfig{RPCURL: "http://x"}, nil); err == nil {
		t.Error("expected error for missing registry address")
	}
}

func TestBesuClient_TransactOpts_NoSigner(t *testing.T) {
	t.Parallel()
	// A BesuClient with no signer must reject transactOpts with ErrNoSigner,
	// without touching the network.
	b := &BesuClient{}
	if _, err := b.transactOpts(context.Background()); err != ErrNoSigner {
		t.Errorf("expected ErrNoSigner, got %v", err)
	}
}
