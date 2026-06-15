// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestVerifyPoP_DecodeErrors(t *testing.T) {
	t.Run("bad_nonce_hex", func(t *testing.T) {
		if err := VerifyPoP("zz", hex.EncodeToString(make([]byte, 65)), "0x0"); err == nil {
			t.Fatal("expected error for bad nonce hex")
		}
	})
	t.Run("bad_signature_hex", func(t *testing.T) {
		if err := VerifyPoP("00", "zz", "0x0"); err == nil {
			t.Fatal("expected error for bad signature hex")
		}
	})
	t.Run("recover_error", func(t *testing.T) {
		// 65 bytes of zeros is not a recoverable signature.
		if err := VerifyPoP("00", hex.EncodeToString(make([]byte, 65)), "0x0"); err == nil {
			t.Fatal("expected error for unrecoverable signature")
		}
	})
}

func TestDeriveAddress_Errors(t *testing.T) {
	t.Run("bad_hex", func(t *testing.T) {
		if _, err := DeriveAddress("zz"); err == nil {
			t.Fatal("expected error for bad hex")
		}
	})
	t.Run("wrong_length", func(t *testing.T) {
		if _, err := DeriveAddress(hex.EncodeToString([]byte{0x01, 0x02, 0x03})); err == nil {
			t.Fatal("expected error for invalid length key")
		}
	})
	t.Run("bad_compressed", func(t *testing.T) {
		// 33 bytes that are not a valid compressed point.
		if _, err := DeriveAddress(hex.EncodeToString(make([]byte, 33))); err == nil {
			t.Fatal("expected error for invalid compressed key")
		}
	})
}

func TestValidatePubKeyMatchesAddress_Success(t *testing.T) {
	key, _ := crypto.GenerateKey()
	pubHex := hex.EncodeToString(crypto.FromECDSAPub(&key.PublicKey))
	addr := crypto.PubkeyToAddress(key.PublicKey).Hex()
	if err := ValidatePubKeyMatchesAddress(pubHex, addr); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
}

func TestValidatePubKeyMatchesAddress_DeriveError(t *testing.T) {
	if err := ValidatePubKeyMatchesAddress("zz", "0x0"); err == nil {
		t.Fatal("expected derive error to propagate")
	}
}
