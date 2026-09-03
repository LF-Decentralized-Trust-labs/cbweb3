// SPDX-License-Identifier: Apache-2.0

package relayauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
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

// A registry that is non-empty but missing the callers' ids is more dangerous than an empty one.
//
// Empty means "no signatures here" and the middleware falls back to the shared secret. Non-empty
// means "verify strictly", so a SIGNED request whose id is not pinned is rejected with 401 — no
// downgrade, which is correct (downgrading would make the signature worthless) but fatal if the
// peers' certificates were never distributed.
//
// The exact shape that produces it: pointing PKI_DIR at a directory that holds only this entity's
// own certificate. IDs() exists so a gateway can see that at boot instead of discovering it as a
// wave of 401s on the settlement path.
func TestRegistry_IDsIsSortedAndComplete(t *testing.T) {
	reg := NewRegistry()
	if got := reg.IDs(); len(got) != 0 {
		t.Fatalf("empty registry must report no ids, got %v", got)
	}
	for _, id := range []string{"central-bank-brazil", "bank-itau", "bank-bradesco"} {
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("keygen: %v", err)
		}
		reg.Add(id, &priv.PublicKey)
	}
	got := reg.IDs()
	want := []string{"bank-bradesco", "bank-itau", "central-bank-brazil"}
	if len(got) != len(want) {
		t.Fatalf("IDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("IDs() = %v, want %v (sorted, so a boot log is stable)", got, want)
		}
	}
}

// --- union of pin sources ---
//
// A central bank learns its banks' identities from onboarding: it signs their CSR and stores the
// issued certificate, which certifies the very key they sign with (verified live — the public key of
// bank-itau.key and of its participant certificate hash identically). So the participants table is
// the authoritative source for every peer the CB onboards.
//
// Files remain a source for peers that are NOT participants — the Cacti relay is one, and it will
// never be onboarded.
//
// The load-bearing rule is the interaction: a participant's authority comes from the table ONLY.
// Otherwise deactivating a bank in compliance would not stop it authenticating, because a stale file
// pin would resurrect it — and revocation is the one thing this source buys that files cannot.

func testCertPEM(t *testing.T, cn string) (string, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), &priv.PublicKey
}

func mustPin(t *testing.T, reg *Registry, id, certPEM string) {
	t.Helper()
	k, err := LoadCertPublicKeyPEM([]byte(certPEM))
	if err != nil {
		t.Fatalf("parse %s: %v", id, err)
	}
	reg.Add(id, k)
}

func TestBuildRegistry_ActiveParticipantIsPinned(t *testing.T) {
	certPEM, pub := testCertPEM(t, "bank-itau")
	reg := BuildRegistry(nil, []ParticipantPin{{ID: "bank-itau", CertPEM: certPEM, Active: true}})
	got, ok := reg.keys["bank-itau"]
	if !ok {
		t.Fatalf("active participant not pinned: %v", reg.IDs())
	}
	if !got.Equal(pub) {
		t.Fatal("pinned the wrong public key")
	}
}

// Revocation: an inactive participant is not pinned, AND a leftover file pin for it is dropped.
// Without the second half, deactivating a bank would revoke nothing.
func TestBuildRegistry_InactiveParticipantOverridesAFilePin(t *testing.T) {
	filePEM, _ := testCertPEM(t, "bank-itau")
	files := NewRegistry()
	mustPin(t, files, "bank-itau", filePEM)

	dbPEM, _ := testCertPEM(t, "bank-itau")
	reg := BuildRegistry(files, []ParticipantPin{{ID: "bank-itau", CertPEM: dbPEM, Active: false}})
	if _, ok := reg.keys["bank-itau"]; ok {
		t.Fatal("a deactivated participant is still pinned — a stale file pin resurrected it, so revocation does nothing")
	}
}

// A peer that is not a participant keeps its file pin: the Cacti relay is exactly that case.
func TestBuildRegistry_KeepsFilePinsForNonParticipants(t *testing.T) {
	relayPEM, relayPub := testCertPEM(t, "cacti-relay")
	files := NewRegistry()
	mustPin(t, files, "cacti-relay", relayPEM)

	bankPEM, _ := testCertPEM(t, "bank-itau")
	reg := BuildRegistry(files, []ParticipantPin{{ID: "bank-itau", CertPEM: bankPEM, Active: true}})
	pinned, ok := reg.keys["cacti-relay"]
	if !ok {
		t.Fatalf("the relay's file pin was dropped: %v", reg.IDs())
	}
	if !pinned.Equal(relayPub) {
		t.Fatal("the relay's pin changed")
	}
	if len(reg.IDs()) != 2 {
		t.Fatalf("expected both peers pinned, got %v", reg.IDs())
	}
}

// An active participant overrides a file pin for the same id: the table is authoritative, so a stale
// file cannot keep an old key alive after a re-issue.
func TestBuildRegistry_ActiveParticipantOverridesAStaleFilePin(t *testing.T) {
	stalePEM, _ := testCertPEM(t, "bank-itau")
	files := NewRegistry()
	mustPin(t, files, "bank-itau", stalePEM)

	freshPEM, freshPub := testCertPEM(t, "bank-itau")
	reg := BuildRegistry(files, []ParticipantPin{{ID: "bank-itau", CertPEM: freshPEM, Active: true}})
	if !reg.keys["bank-itau"].Equal(freshPub) {
		t.Fatal("the stale file pin won over the participants table")
	}
}

// One malformed certificate must not take the whole registry down: a single bad row would otherwise
// stop every other peer from authenticating.
func TestBuildRegistry_SkipsAnUnparseableParticipantCert(t *testing.T) {
	goodPEM, _ := testCertPEM(t, "bank-bradesco")
	reg := BuildRegistry(nil, []ParticipantPin{
		{ID: "bank-itau", CertPEM: "not a certificate", Active: true},
		{ID: "bank-bradesco", CertPEM: goodPEM, Active: true},
	})
	if _, ok := reg.keys["bank-bradesco"]; !ok {
		t.Fatal("a healthy peer was lost because another row was malformed")
	}
	if _, ok := reg.keys["bank-itau"]; ok {
		t.Fatal("an unparseable certificate was pinned")
	}
}

// A CA certificate must never be pinned as a peer identity.
//
// The name-based skip (*-ca.crt) misses the case seen live: the CB's CA is written as
// central-bank.crt, so the glob pinned it under the key-id "central-bank". Not exploitable from
// outside — only the CA key could produce a matching signature — but a certificate authority is not
// a service identity, and pinning it makes the trust anchor say something it does not mean.
func TestLoadRegistryGlob_SkipsCACertificates(t *testing.T) {
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "central-bank-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	// Named without the -ca suffix, exactly as the toolkit writes it.
	if err := os.WriteFile(filepath.Join(dir, "central-bank.crt"),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o644); err != nil {
		t.Fatal(err)
	}

	leafPEM, _ := testCertPEM(t, "cacti-relay")
	if err := os.WriteFile(filepath.Join(dir, "cacti-relay.crt"), []byte(leafPEM), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadRegistryGlob(dir)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if _, pinned := reg.keys["central-bank"]; pinned {
		t.Fatal("pinned a CA certificate as a peer identity")
	}
	if _, pinned := reg.keys["cacti-relay"]; !pinned {
		t.Fatalf("the leaf certificate was not pinned: %v", reg.IDs())
	}
}

// --- refresh on an unknown key-id ---
//
// A central bank's peers are its onboarded banks, and a bank signs its internal calls from the moment
// it has a key. With a periodic-only refresh there is a window — up to the refresh interval — where a
// legitimately onboarded bank is not yet pinned and every call it makes is rejected with 401. The
// sample deployment hits exactly that: it onboards a bank and immediately makes a deposit.
//
// So an unknown key-id triggers ONE refresh before the request is rejected. That is not a weakening:
// a refresh can only load certificates the CB itself issued, so an attacker presenting an unknown id
// gains nothing from it. It is rate-limited because otherwise unknown ids would be a free way to make
// the gateway hammer its own database.

func TestStore_EnsureFreshRefreshesOnlyForAnUnknownKeyID(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	known := NewRegistry()
	known.Add("bank-itau", &priv.PublicKey)

	store := NewStore(known)
	calls := 0
	store.SetRefresher(func() { calls++ }, 0)

	store.EnsureFresh("bank-itau")
	if calls != 0 {
		t.Fatalf("a known key-id must not trigger a refresh, got %d", calls)
	}
	store.EnsureFresh("bank-bradesco")
	if calls != 1 {
		t.Fatalf("an unknown key-id must trigger exactly one refresh, got %d", calls)
	}
}

// Rate limit: repeated unknown ids must not turn into repeated database reads.
func TestStore_EnsureFreshIsRateLimited(t *testing.T) {
	store := NewStore(NewRegistry())
	calls := 0
	store.SetRefresher(func() { calls++ }, time.Hour)

	for i := 0; i < 5; i++ {
		store.EnsureFresh("unknown-peer")
	}
	if calls != 1 {
		t.Fatalf("expected the refresh to be rate-limited to one call, got %d", calls)
	}
}

// An empty key-id is an unsigned request; there is nothing to look up and nothing to refresh for.
func TestStore_EnsureFreshIgnoresAnEmptyKeyID(t *testing.T) {
	store := NewStore(NewRegistry())
	calls := 0
	store.SetRefresher(func() { calls++ }, 0)
	store.EnsureFresh("")
	if calls != 0 {
		t.Fatalf("an unsigned request must not trigger a refresh, got %d", calls)
	}
}

// Without a refresher configured the call must be a harmless no-op, not a panic.
func TestStore_EnsureFreshWithoutARefresher(t *testing.T) {
	store := NewStore(NewRegistry())
	if got := store.EnsureFresh("whoever"); got == nil {
		t.Fatal("EnsureFresh must still return the registry in force")
	}
}
