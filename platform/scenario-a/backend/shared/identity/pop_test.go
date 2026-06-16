// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestVerifyPoP_Success(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	nonce := []byte("test-nonce-issued-by-ca")
	nonceHex := hex.EncodeToString(nonce)

	digest := sha256.Sum256(nonce)
	sig, err := crypto.Sign(digest[:], key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	signatureHex := hex.EncodeToString(sig)
	expectedAddr := crypto.PubkeyToAddress(key.PublicKey).Hex()

	if err := VerifyPoP(nonceHex, signatureHex, expectedAddr); err != nil {
		t.Fatalf("VerifyPoP: expected nil, got %v", err)
	}
}

func TestVerifyPoP_WrongAddress(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	other, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey other: %v", err)
	}

	nonce := []byte("another-nonce")
	nonceHex := hex.EncodeToString(nonce)

	digest := sha256.Sum256(nonce)
	sig, err := crypto.Sign(digest[:], key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	signatureHex := hex.EncodeToString(sig)
	wrongAddr := crypto.PubkeyToAddress(other.PublicKey).Hex()

	if err := VerifyPoP(nonceHex, signatureHex, wrongAddr); err == nil {
		t.Fatal("VerifyPoP: expected error for wrong address, got nil")
	}
}

func TestVerifyPoP_InvalidSignature(t *testing.T) {
	nonceHex := hex.EncodeToString([]byte("nonce"))
	expectedAddr := "0x0000000000000000000000000000000000000001"

	// Valid hex but wrong length (64 bytes, not 65).
	garbageSig := hex.EncodeToString(make([]byte, 64))

	if err := VerifyPoP(nonceHex, garbageSig, expectedAddr); err == nil {
		t.Fatal("VerifyPoP: expected error for invalid signature, got nil")
	}
}

func TestDeriveAddress_Uncompressed(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	want := crypto.PubkeyToAddress(key.PublicKey).Hex()
	pubBytes := crypto.FromECDSAPub(&key.PublicKey)
	pubHex := hex.EncodeToString(pubBytes)

	got, err := DeriveAddress(pubHex)
	if err != nil {
		t.Fatalf("DeriveAddress: %v", err)
	}
	if got != want {
		t.Errorf("DeriveAddress: got %s, want %s", got, want)
	}
}

func TestDeriveAddress_Compressed(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	want := crypto.PubkeyToAddress(key.PublicKey).Hex()
	compressed := crypto.CompressPubkey(&key.PublicKey)
	pubHex := hex.EncodeToString(compressed)

	got, err := DeriveAddress(pubHex)
	if err != nil {
		t.Fatalf("DeriveAddress: %v", err)
	}
	if got != want {
		t.Errorf("DeriveAddress: got %s, want %s", got, want)
	}
}

func TestValidatePubKeyMatchesAddress_Mismatch(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	other, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey other: %v", err)
	}

	pubHex := hex.EncodeToString(crypto.FromECDSAPub(&key.PublicKey))
	wrongAddr := crypto.PubkeyToAddress(other.PublicKey).Hex()

	if err := ValidatePubKeyMatchesAddress(pubHex, wrongAddr); err == nil {
		t.Fatal("ValidatePubKeyMatchesAddress: expected error on mismatch, got nil")
	}
}
