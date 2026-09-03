// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/pki"
)

func p256Key(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// makeCSR builds a PEM CSR with the given signer key and OUs.
func makeCSR(t *testing.T, key any, ou []string) []byte {
	t.Helper()
	tmpl := x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "bank-a", OrganizationalUnit: ou},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &tmpl, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}

func parseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("bad cert PEM")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// SC-004: leaf issued from a valid CSR verifies against the spoke anchor.
func TestIssueLeafVerifiesAgainstAnchor(t *testing.T) {
	cs := newLocal()
	ctx := context.Background()
	csr := makeCSR(t, p256Key(t), []string{roleCommercialBank})

	leafPEM, err := cs.IssueLeafCert(ctx, csr, "spoke-a")
	if err != nil {
		t.Fatalf("IssueLeafCert: %v", err)
	}
	caPEM, err := cs.GetTrustAnchor(ctx, "spoke-a")
	if err != nil {
		t.Fatalf("GetTrustAnchor: %v", err)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("could not load anchor")
	}
	leaf := parseCert(t, leafPEM)
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("leaf does not verify against anchor: %v", err)
	}
}

// Integration: the real pki.GenerateBankCSR output is accepted.
func TestIssueFromPKIGeneratedCSR(t *testing.T) {
	dir := t.TempDir()
	_, csrPath, err := pki.GenerateBankCSR("bank-a", "Bank A S.A.", dir)
	if err != nil {
		t.Fatal(err)
	}
	csrPEM, err := os.ReadFile(csrPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newLocal().IssueLeafCert(context.Background(), csrPEM, "spoke-a"); err != nil {
		t.Fatalf("IssueLeafCert on pki CSR: %v", err)
	}
}

// SC-005: reject non-P-256, wrong-curve, missing-OU, malformed CSRs.
func TestRejectRSAKey(t *testing.T) {
	cs := newLocal()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	csr := makeCSR(t, rsaKey, []string{roleCommercialBank})
	if _, err := cs.IssueLeafCert(context.Background(), csr, "spoke-a"); err != ErrUnsupportedKeyAlgorithm {
		t.Fatalf("got %v, want ErrUnsupportedKeyAlgorithm", err)
	}
}

func TestRejectWrongCurve(t *testing.T) {
	cs := newLocal()
	k, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr := makeCSR(t, k, []string{roleCommercialBank})
	if _, err := cs.IssueLeafCert(context.Background(), csr, "spoke-a"); err != ErrUnsupportedKeyAlgorithm {
		t.Fatalf("got %v, want ErrUnsupportedKeyAlgorithm", err)
	}
}

func TestRejectMissingRole(t *testing.T) {
	cs := newLocal()
	csr := makeCSR(t, p256Key(t), []string{"SOMETHING_ELSE"})
	if _, err := cs.IssueLeafCert(context.Background(), csr, "spoke-a"); err != ErrForbiddenRole {
		t.Fatalf("got %v, want ErrForbiddenRole", err)
	}
}

func TestRejectMalformedCSR(t *testing.T) {
	cs := newLocal()
	if _, err := cs.IssueLeafCert(context.Background(), []byte("not a pem"), "spoke-a"); err != ErrInvalidCSR {
		t.Fatalf("got %v, want ErrInvalidCSR", err)
	}
}

// Edge case: GetTrustAnchor never creates a CA; a rejected issue leaves none.
func TestGetTrustAnchorDoesNotCreateCA(t *testing.T) {
	cs := newLocal()
	ctx := context.Background()
	if _, err := cs.GetTrustAnchor(ctx, "spoke-x"); err != ErrTrustAnchorNotFound {
		t.Fatalf("got %v, want ErrTrustAnchorNotFound", err)
	}
	// A rejected issuance must not lazily create a CA.
	csr := makeCSR(t, p256Key(t), []string{"NOPE"})
	_, _ = cs.IssueLeafCert(ctx, csr, "spoke-x")
	if _, err := cs.GetTrustAnchor(ctx, "spoke-x"); err != ErrTrustAnchorNotFound {
		t.Fatalf("rejected issue created a CA: %v", err)
	}
}
