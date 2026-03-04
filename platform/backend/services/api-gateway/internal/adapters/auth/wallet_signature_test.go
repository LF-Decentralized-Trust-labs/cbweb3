// This file tests Ethereum signature verification used in wallet binding.
package auth

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestVerifyWalletSignature(t *testing.T) {
	t.Parallel()

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	wallet := crypto.PubkeyToAddress(key.PublicKey).Hex()
	subject := "bank-a"

	message := fmt.Sprintf("CBWEB3_WALLET_BIND:%s:%s", subject, strings.ToLower(wallet))
	hash := accounts.TextHash([]byte(message))
	signature, err := crypto.Sign(hash, key)
	if err != nil {
		t.Fatalf("failed to sign hash: %v", err)
	}

	if err := VerifyWalletSignature(subject, wallet, "0x"+hex.EncodeToString(signature)); err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}
}

func TestVerifyWalletSignatureFail(t *testing.T) {
	t.Parallel()

	if err := VerifyWalletSignature("bank-a", "0x1234", "0xdeadbeef"); err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestVerifyWalletSignatureMismatch(t *testing.T) {
	t.Parallel()

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	other, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	wallet := crypto.PubkeyToAddress(other.PublicKey).Hex()
	subject := "bank-a"
	message := fmt.Sprintf("CBWEB3_WALLET_BIND:%s:%s", subject, strings.ToLower(wallet))
	hash := accounts.TextHash([]byte(message))
	signature, err := crypto.Sign(hash, key)
	if err != nil {
		t.Fatalf("failed to sign hash: %v", err)
	}
	if err := VerifyWalletSignature(subject, wallet, "0x"+hex.EncodeToString(signature)); err == nil {
		t.Fatal("expected mismatch signature error")
	}
}

