package relayauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pkiDir is the project PKI relative to this package (backend/config/pki).
const pkiDir = "../../../../config/pki"

// The real project PKI material (EC key + X.509 cert from `make pki.gen-all`) must
// round-trip: a signature with central-bank-a.key verifies against central-bank-a.crt.
func TestPKIKeyMaterialRoundTrips(t *testing.T) {
	if _, err := os.Stat(filepath.Join(pkiDir, "central-bank-a.key")); err != nil {
		t.Skip("project PKI not generated (run make pki.gen-all); skipping PKI round-trip")
	}
	signer, err := LoadSigner(pkiDir, "central-bank-a")
	if err != nil {
		t.Fatalf("load signer from PKI: %v", err)
	}
	reg, err := LoadRegistryFromPKIDir(pkiDir, []string{"central-bank-a", "central-bank-b"})
	if err != nil {
		t.Fatalf("load registry from PKI: %v", err)
	}

	body := []byte(`{"correlation_id":"pki-1"}`)
	now := time.Now()
	h, err := signer.HeadersFor(tMethod, tPath, body, now)
	if err != nil {
		t.Fatalf("sign with PKI key: %v", err)
	}
	if err := reg.VerifyRequest(h[HeaderKeyID], h[HeaderTimestamp], h[HeaderSignature], tMethod, tPath, body, now); err != nil {
		t.Fatalf("expected PKI signature to verify against the pinned cert, got: %v", err)
	}
}

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return k
}

const (
	tMethod = "POST"
	tPath   = "/internal/amm/cross-currency-bridge-in"
)

// A signature from the registered key over the exact request verifies.
func TestVerifyRequest_HappyPath(t *testing.T) {
	key := mustKey(t)
	reg := NewRegistry()
	reg.Add("central-bank-a", &key.PublicKey)

	body := []byte(`{"correlation_id":"c1","amount":"100"}`)
	s := NewSigner("central-bank-a", key)
	now := time.Now()
	h, err := s.HeadersFor(tMethod, tPath, body, now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if err := reg.VerifyRequest(h[HeaderKeyID], h[HeaderTimestamp], h[HeaderSignature], tMethod, tPath, body, now); err != nil {
		t.Fatalf("expected verification to pass, got: %v", err)
	}
}

// A tampered body must fail — the signature is bound to the body hash.
func TestVerifyRequest_TamperedBodyRejected(t *testing.T) {
	key := mustKey(t)
	reg := NewRegistry()
	reg.Add("central-bank-a", &key.PublicKey)

	body := []byte(`{"amount":"100"}`)
	s := NewSigner("central-bank-a", key)
	now := time.Now()
	h, _ := s.HeadersFor(tMethod, tPath, body, now)

	tampered := []byte(`{"amount":"999999"}`)
	if err := reg.VerifyRequest(h[HeaderKeyID], h[HeaderTimestamp], h[HeaderSignature], tMethod, tPath, tampered, now); err == nil {
		t.Fatal("expected verification to fail for tampered body, got nil")
	}
}

// A signature for one path must not authenticate a different path.
func TestVerifyRequest_PathBindingRejected(t *testing.T) {
	key := mustKey(t)
	reg := NewRegistry()
	reg.Add("central-bank-a", &key.PublicKey)

	body := []byte(`{}`)
	s := NewSigner("central-bank-a", key)
	now := time.Now()
	h, _ := s.HeadersFor(tMethod, tPath, body, now)

	if err := reg.VerifyRequest(h[HeaderKeyID], h[HeaderTimestamp], h[HeaderSignature], tMethod, "/internal/amm/execute-matched-commit", body, now); err == nil {
		t.Fatal("expected verification to fail for a different path, got nil")
	}
}

// A captured signature replayed outside the skew window is rejected.
func TestVerifyRequest_ExpiredTimestampRejected(t *testing.T) {
	key := mustKey(t)
	reg := NewRegistry().WithMaxSkew(5 * time.Minute)
	reg.Add("central-bank-a", &key.PublicKey)

	body := []byte(`{}`)
	signedAt := time.Now().Add(-10 * time.Minute)
	s := NewSigner("central-bank-a", key)
	h, _ := s.HeadersFor(tMethod, tPath, body, signedAt)

	if err := reg.VerifyRequest(h[HeaderKeyID], h[HeaderTimestamp], h[HeaderSignature], tMethod, tPath, body, time.Now()); err == nil {
		t.Fatal("expected verification to fail for an expired timestamp, got nil")
	}
}

// A key-id not pinned in the registry cannot authenticate, even with a valid self-signature.
func TestVerifyRequest_UnknownKeyIDRejected(t *testing.T) {
	key := mustKey(t)
	reg := NewRegistry() // empty — nothing pinned

	body := []byte(`{}`)
	s := NewSigner("attacker", key)
	now := time.Now()
	h, _ := s.HeadersFor(tMethod, tPath, body, now)

	if err := reg.VerifyRequest(h[HeaderKeyID], h[HeaderTimestamp], h[HeaderSignature], tMethod, tPath, body, now); err == nil {
		t.Fatal("expected verification to fail for an unknown key-id, got nil")
	}
}

// A signature made with a different private key than the one pinned for the key-id fails.
func TestVerifyRequest_WrongKeyRejected(t *testing.T) {
	pinned := mustKey(t)
	other := mustKey(t)
	reg := NewRegistry()
	reg.Add("central-bank-a", &pinned.PublicKey) // pin CB-A's real key

	body := []byte(`{}`)
	s := NewSigner("central-bank-a", other) // attacker claims CB-A but signs with another key
	now := time.Now()
	h, _ := s.HeadersFor(tMethod, tPath, body, now)

	if err := reg.VerifyRequest(h[HeaderKeyID], h[HeaderTimestamp], h[HeaderSignature], tMethod, tPath, body, now); err == nil {
		t.Fatal("expected verification to fail when signed with the wrong key, got nil")
	}
}

// Missing headers are rejected (fail closed, not panic).
func TestVerifyRequest_MissingHeadersRejected(t *testing.T) {
	reg := NewRegistry()
	if err := reg.VerifyRequest("", "", "", tMethod, tPath, []byte(`{}`), time.Now()); err == nil {
		t.Fatal("expected verification to fail for missing headers, got nil")
	}
}
