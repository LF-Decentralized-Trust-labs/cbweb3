// SPDX-License-Identifier: Apache-2.0

package keyprovider_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// ============================================================
// US1 — Provisionar sem chaves em arquivos (Priority: P1)
// ============================================================

func TestLocalKeyProvider_ExportPrivateKeyHex_Seeded(t *testing.T) {
	p := keyprovider.NewLocalKeyProviderSeeded()

	hexKey, err := p.ExportPrivateKeyHex(keyprovider.LocalOperatorKeyID)
	if err != nil {
		t.Fatalf("ExportPrivateKeyHex: %v", err)
	}
	// The seeded operator key is the well-known public Besu dev account; its hex
	// must round-trip to a valid secp256k1 key.
	if _, err := gethcrypto.HexToECDSA(hexKey); err != nil {
		t.Errorf("exported hex is not a valid key: %v", err)
	}
}

func TestLocalKeyProvider_ExportPrivateKeyHex_Unknown(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	if _, err := p.ExportPrivateKeyHex("nonexistent"); !errors.Is(err, keyprovider.ErrKeyNotFound) {
		t.Errorf("want ErrKeyNotFound, got %v", err)
	}
}

func TestLocalKeyProvider_GenerateKey(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	pub, err := p.GenerateKey(ctx, "participant-a")
	if err != nil {
		t.Fatalf("GenerateKey: unexpected error: %v", err)
	}
	if len(pub) != 65 {
		t.Errorf("GenerateKey: want 65-byte pubkey, got %d bytes", len(pub))
	}
	if pub[0] != 0x04 {
		t.Errorf("GenerateKey: want uncompressed pubkey (0x04 prefix), got 0x%02x", pub[0])
	}
}

func TestLocalKeyProvider_GenerateKey_Idempotent(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	pub1, err := p.GenerateKey(ctx, "participant-a")
	if err != nil {
		t.Fatalf("first GenerateKey: %v", err)
	}
	pub2, err := p.GenerateKey(ctx, "participant-a")
	if err != nil {
		t.Fatalf("second GenerateKey: %v", err)
	}
	if !bytes.Equal(pub1, pub2) {
		t.Error("GenerateKey: second call returned different pubkey for the same id")
	}
}

func TestLocalKeyProvider_Sign_Verifiable(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	pub, err := p.GenerateKey(ctx, "signer")
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	digest := sha256.Sum256([]byte("test payload"))
	sig, err := p.Sign(ctx, "signer", digest[:])
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) != 65 {
		t.Errorf("Sign: want 65-byte signature, got %d bytes", len(sig))
	}
	if !gethcrypto.VerifySignature(pub, digest[:], sig[:64]) {
		t.Error("Sign: signature failed verification against public key")
	}
}

func TestLocalKeyProvider_GetPublicKey_AfterGenerate(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	generated, err := p.GenerateKey(ctx, "participant-b")
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	retrieved, err := p.GetPublicKey(ctx, "participant-b")
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}
	if !bytes.Equal(generated, retrieved) {
		t.Error("GetPublicKey: returned pubkey differs from GenerateKey result")
	}
}

func TestEVMAddress_Derivation(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	pub, err := p.GenerateKey(ctx, "evm-participant")
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	addr, err := keyprovider.EVMAddress(pub)
	if err != nil {
		t.Fatalf("EVMAddress: %v", err)
	}
	if !strings.HasPrefix(addr, "0x") {
		t.Errorf("EVMAddress: want 0x-prefixed address, got %q", addr)
	}
	if len(addr) != 42 {
		t.Errorf("EVMAddress: want 42-char address, got %d chars: %q", len(addr), addr)
	}

	ecKey, err := gethcrypto.UnmarshalPubkey(pub)
	if err != nil {
		t.Fatalf("UnmarshalPubkey: %v", err)
	}
	expected := gethcrypto.PubkeyToAddress(*ecKey).Hex()
	if addr != expected {
		t.Errorf("EVMAddress: got %q, want %q", addr, expected)
	}
}

// ============================================================
// US2 — Uso local sem dependência de serviço externo (Priority: P2)
// ============================================================

func TestLocalKeyProvider_GetPublicKey_NotFound(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	_, err := p.GetPublicKey(ctx, "nonexistent")
	if !errors.Is(err, keyprovider.ErrKeyNotFound) {
		t.Errorf("GetPublicKey(unknown id): want ErrKeyNotFound, got %v", err)
	}
}

func TestLocalKeyProvider_Sign_InvalidDigestLength(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	_, _ = p.GenerateKey(ctx, "signer")

	cases := []struct {
		name    string
		payload []byte
	}{
		{"empty", []byte{}},
		{"31 bytes", make([]byte, 31)},
		{"33 bytes", make([]byte, 33)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Sign(ctx, "signer", tc.payload)
			if !errors.Is(err, keyprovider.ErrInvalidDigest) {
				t.Errorf("Sign(%d-byte payload): want ErrInvalidDigest, got %v", len(tc.payload), err)
			}
		})
	}
}

func TestLocalKeyProvider_Sign_UnknownID(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	digest := sha256.Sum256([]byte("payload"))
	_, err := p.Sign(ctx, "ghost", digest[:])
	if !errors.Is(err, keyprovider.ErrKeyNotFound) {
		t.Errorf("Sign(unknown id): want ErrKeyNotFound, got %v", err)
	}
}

func TestLocalKeyProvider_GenerateKey_DifferentIDs(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()

	pub1, err := p.GenerateKey(ctx, "bank-a")
	if err != nil {
		t.Fatalf("GenerateKey bank-a: %v", err)
	}
	pub2, err := p.GenerateKey(ctx, "bank-b")
	if err != nil {
		t.Fatalf("GenerateKey bank-b: %v", err)
	}
	if bytes.Equal(pub1, pub2) {
		t.Error("GenerateKey: distinct IDs produced identical public keys")
	}
}

func TestLocalKeyProvider_GenerateKey_Concurrent(t *testing.T) {
	p := keyprovider.NewLocalKeyProvider()
	ctx := context.Background()
	const workers = 10

	results := make([][]byte, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			pub, err := p.GenerateKey(ctx, "shared-id")
			if err != nil {
				t.Errorf("worker %d: GenerateKey: %v", i, err)
				return
			}
			results[i] = pub
		}()
	}
	wg.Wait()

	first := results[0]
	for i, r := range results {
		if !bytes.Equal(first, r) {
			t.Errorf("worker %d: got different pubkey (concurrent GenerateKey not idempotent)", i)
		}
	}
}

// ============================================================
// US3 — Extensibilidade para KMS de produção (Priority: P3)
// ============================================================

func TestFactory_LocalEmulator(t *testing.T) {
	kp, err := keyprovider.New("kms://local-emulator")
	if err != nil {
		t.Fatalf("New(kms://local-emulator): %v", err)
	}
	if kp == nil {
		t.Fatal("New(kms://local-emulator): returned nil provider")
	}

	pub, err := kp.GenerateKey(context.Background(), "test")
	if err != nil {
		t.Fatalf("GenerateKey on local emulator: %v", err)
	}
	if len(pub) != 65 {
		t.Errorf("GenerateKey: want 65 bytes, got %d", len(pub))
	}
}

func TestFactory_ProdStub_GenerateKey_NotImplemented(t *testing.T) {
	kp, err := keyprovider.New("kms://vault://prod-kms")
	if err != nil {
		t.Fatalf("New(prod URI): %v", err)
	}
	_, err = kp.GenerateKey(context.Background(), "test")
	if !errors.Is(err, keyprovider.ErrNotImplemented) {
		t.Errorf("prod stub GenerateKey: want ErrNotImplemented, got %v", err)
	}
}

func TestFactory_ProdStub_Sign_NotImplemented(t *testing.T) {
	kp, err := keyprovider.New("kms://vault://prod-kms")
	if err != nil {
		t.Fatalf("New(prod URI): %v", err)
	}
	digest := sha256.Sum256([]byte("x"))
	_, err = kp.Sign(context.Background(), "test", digest[:])
	if !errors.Is(err, keyprovider.ErrNotImplemented) {
		t.Errorf("prod stub Sign: want ErrNotImplemented, got %v", err)
	}
}

func TestFactory_ProdStub_GetPublicKey_NotImplemented(t *testing.T) {
	kp, err := keyprovider.New("kms://vault://prod-kms")
	if err != nil {
		t.Fatalf("New(prod URI): %v", err)
	}
	_, err = kp.GetPublicKey(context.Background(), "test")
	if !errors.Is(err, keyprovider.ErrNotImplemented) {
		t.Errorf("prod stub GetPublicKey: want ErrNotImplemented, got %v", err)
	}
}

func TestFactory_InvalidURI_NoScheme(t *testing.T) {
	_, err := keyprovider.New("vault://prod-kms")
	if err == nil {
		t.Error("New(URI without kms://): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "kms://") {
		t.Errorf("New(invalid URI): error should mention kms://, got: %v", err)
	}
}

func TestFactory_InvalidURI_Empty(t *testing.T) {
	_, err := keyprovider.New("")
	if err == nil {
		t.Error("New(empty URI): expected error, got nil")
	}
}
