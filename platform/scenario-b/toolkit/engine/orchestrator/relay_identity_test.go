package orchestrator

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

// The central bank needs a service identity of its own: a key it signs WITH (relay auth when it is
// the sender, and the circuit-breaker institutional attestation) plus the certificate peers pin.
//
// It cannot come from onboarding — a CB does not onboard itself — so provisioning issues it, signed
// by the CA the CB already has in its PKI volume. Signed rather than self-signed because that is the
// shape a real CA will take over: same file, same place, different issuer.
//
// The properties below are what make it usable, and each has a failure mode that is silent:
// a CA-flagged leaf is refused by the pin loader (it rejects IsCA, so the peer would never be
// verifiable); a wrong CommonName is cosmetic but misleading; and a leaf that does not verify against
// the CA cannot be validated once chain checking replaces pinning.
func TestIssueRelayIdentity(t *testing.T) {
	caCertPEM, caKeyPEM, err := generateSelfSignedCA("central-bank-ca")
	if err != nil {
		t.Fatalf("ca: %v", err)
	}

	keyPEM, certPEM, err := issueRelayIdentity(caCertPEM, caKeyPEM, "central-bank-brazil")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("certificate is not PEM")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	// A CA certificate is refused as a peer identity by the pin loader, so a leaf that claims to be
	// one would never be verifiable.
	if leaf.IsCA {
		t.Fatal("the issued identity is flagged as a CA — the pin loader refuses those")
	}
	if leaf.Subject.CommonName != "central-bank-brazil" {
		t.Fatalf("CommonName = %q, want central-bank-brazil", leaf.Subject.CommonName)
	}
	if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Fatal("the identity cannot be used to sign")
	}

	// It must chain to the CA, or chain validation (which a real CA enables) could never replace
	// pinning without re-issuing everything.
	caBlock, _ := pem.Decode(caCertPEM)
	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}
	if err := leaf.CheckSignatureFrom(ca); err != nil {
		t.Fatalf("the identity does not chain to the CB's CA: %v", err)
	}

	if !strings.Contains(string(keyPEM), "PRIVATE KEY") {
		t.Fatalf("the private key is not PEM: %q", string(keyPEM)[:40])
	}
}

// Deterministic inputs must not produce a deterministic key: a service identity is a real secret,
// unlike the derived blockchain keys.
func TestIssueRelayIdentity_KeysAreRandom(t *testing.T) {
	caCertPEM, caKeyPEM, err := generateSelfSignedCA("central-bank-ca")
	if err != nil {
		t.Fatalf("ca: %v", err)
	}
	k1, _, err := issueRelayIdentity(caCertPEM, caKeyPEM, "central-bank-brazil")
	if err != nil {
		t.Fatal(err)
	}
	k2, _, err := issueRelayIdentity(caCertPEM, caKeyPEM, "central-bank-brazil")
	if err != nil {
		t.Fatal(err)
	}
	if string(k1) == string(k2) {
		t.Fatal("two issuances produced the same private key — this identity must be random, not derived")
	}
}
