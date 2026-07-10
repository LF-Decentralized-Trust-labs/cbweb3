package keyprovider

import (
	"bytes"
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// SC-001: generate → GetPublicKey → Sign → verify; GenerateKey idempotent.
func TestGenerateSignVerify(t *testing.T) {
	l := newLocal("")
	ctx := context.Background()

	pub, err := l.GenerateKey(ctx, "bank-a")
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if len(pub) != 65 || pub[0] != 0x04 {
		t.Fatalf("unexpected pubkey shape: len=%d prefix=%x", len(pub), pub[0])
	}
	pub2, _ := l.GenerateKey(ctx, "bank-a")
	if !bytes.Equal(pub, pub2) {
		t.Fatal("GenerateKey not idempotent")
	}
	got, err := l.GetPublicKey(ctx, "bank-a")
	if err != nil || !bytes.Equal(got, pub) {
		t.Fatalf("GetPublicKey mismatch: %v", err)
	}

	digest := crypto.Keccak256([]byte("settlement-tx"))
	sig, err := l.Sign(ctx, "bank-a", digest)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) != 65 {
		t.Fatalf("signature len = %d, want 65", len(sig))
	}
	recovered, err := crypto.SigToPub(digest, sig)
	if err != nil {
		t.Fatalf("SigToPub: %v", err)
	}
	if !bytes.Equal(crypto.FromECDSAPub(recovered), pub) {
		t.Fatal("recovered pubkey does not match")
	}
	if !crypto.VerifySignature(pub, digest, sig[:64]) {
		t.Fatal("VerifySignature failed")
	}
}

// SC-002: distinct ids → distinct EVM addresses.
func TestDistinctIdsDistinctAddresses(t *testing.T) {
	l := newLocal("")
	ctx := context.Background()
	pa, _ := l.GenerateKey(ctx, "central-bank-a")
	pb, _ := l.GenerateKey(ctx, "bank-a")
	aa, err := EVMAddress(pa)
	if err != nil {
		t.Fatal(err)
	}
	ab, err := EVMAddress(pb)
	if err != nil {
		t.Fatal(err)
	}
	if aa == ab {
		t.Fatalf("distinct ids produced same address: %s", aa)
	}
}

// FR-003: derivation is deterministic across instances (stable addresses).
func TestDeterministicAcrossInstances(t *testing.T) {
	ctx := context.Background()
	a, _ := newLocal("").GenerateKey(ctx, "bank-a")
	b, _ := newLocal("").GenerateKey(ctx, "bank-a")
	if !bytes.Equal(a, b) {
		t.Fatal("derivation not deterministic across instances")
	}
	// A different seed yields a different key for the same id.
	c, _ := newLocal("other-seed").GenerateKey(ctx, "bank-a")
	if bytes.Equal(a, c) {
		t.Fatal("different seed produced same key")
	}
}

// Edge case: GetPublicKey must not generate implicitly.
func TestGetPublicKeyNoImplicitGen(t *testing.T) {
	l := newLocal("")
	if _, err := l.GetPublicKey(context.Background(), "never-generated"); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

// FR-003: a well-known dev key is seeded under the well-known id.
func TestSeededDevKey(t *testing.T) {
	l := newLocal("")
	if _, err := l.GetPublicKey(context.Background(), SeededDevID); err != nil {
		t.Fatalf("seeded dev key missing: %v", err)
	}
}

func TestSignRejectsBadDigestAndUnknownId(t *testing.T) {
	l := newLocal("")
	ctx := context.Background()
	_, _ = l.GenerateKey(ctx, "bank-a")
	if _, err := l.Sign(ctx, "bank-a", []byte("short")); err != ErrInvalidDigest {
		t.Fatalf("expected ErrInvalidDigest, got %v", err)
	}
	if _, err := l.Sign(ctx, "unknown", crypto.Keccak256([]byte("x"))); err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}
