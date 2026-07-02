// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ── US1: IssueLeafCert ────────────────────────────────────────────────────────

func TestIssueLeafCert_ValidCSR_IsVerifiableByCACert(t *testing.T) {
	cs := NewLocalCertSource()
	csrPEM := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")

	certPEM, err := cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl")
	if err != nil {
		t.Fatalf("IssueLeafCert: unexpected error: %v", err)
	}

	caCertPEM, err := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	if err != nil {
		t.Fatalf("GetTrustAnchor: unexpected error: %v", err)
	}

	verifyLeafCert(t, certPEM, caCertPEM, "bank-a")
}

func TestIssueLeafCert_MalformedPEM_ReturnsError(t *testing.T) {
	cs := NewLocalCertSource()
	_, err := cs.IssueLeafCert(context.Background(), []byte("not a valid PEM"), "spoke-brl")
	if !errors.Is(err, ErrInvalidCSR) {
		t.Errorf("expected ErrInvalidCSR, got %v", err)
	}
}

func TestIssueLeafCert_ForbiddenRole_ReturnsErrForbiddenRole(t *testing.T) {
	cs := NewLocalCertSource()
	csrPEM := generateTestCSR(t, "cb-brazil", "ROLE_CENTRAL_BANK")

	_, err := cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl")
	if !errors.Is(err, ErrForbiddenRole) {
		t.Errorf("expected ErrForbiddenRole, got %v", err)
	}
}

func TestIssueLeafCert_UnsupportedKeyAlgorithm_ReturnsError(t *testing.T) {
	cs := NewLocalCertSource()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         "bank-rsa",
			OrganizationalUnit: []string{"ROLE_COMMERCIAL_BANK"},
		},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, template, rsaKey)
	if err != nil {
		t.Fatalf("create RSA CSR: %v", err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})

	_, err = cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl")
	if !errors.Is(err, ErrUnsupportedKeyAlgorithm) {
		t.Errorf("expected ErrUnsupportedKeyAlgorithm, got %v", err)
	}
}

func TestIssueLeafCert_InvalidCSRSignature_ReturnsError(t *testing.T) {
	cs := NewLocalCertSource()
	csrPEM := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")

	block, _ := pem.Decode(csrPEM)
	// Corrupt the last 4 bytes — deep inside the ECDSA signature's s-value.
	// These bytes are within the BIT STRING content and will not affect DER framing.
	der := append([]byte(nil), block.Bytes...)
	for i := range 4 {
		der[len(der)-1-i] ^= 0xFF
	}
	corrupt := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})

	_, err := cs.IssueLeafCert(context.Background(), corrupt, "spoke-brl")
	if !errors.Is(err, ErrInvalidCSR) {
		t.Errorf("expected ErrInvalidCSR, got %v", err)
	}
}

func TestLocalCertSource_NoPrivateKeyFile(t *testing.T) {
	cs := NewLocalCertSource()
	csrPEM := generateTestCSR(t, "bank-file-test", "ROLE_COMMERCIAL_BANK")
	_, _ = cs.IssueLeafCert(context.Background(), csrPEM, "spoke-file-test")
	_, _ = cs.GetTrustAnchor(context.Background(), "spoke-file-test")

	for _, pattern := range []string{"*.key", "*-ca.*"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %q: %v", pattern, err)
		}
		if len(matches) > 0 {
			t.Errorf("unexpected files matching %q: %v", pattern, matches)
		}
	}
}

// ── US2: GetTrustAnchor ───────────────────────────────────────────────────────

func TestGetTrustAnchor_UnknownSpokeID_ReturnsErrNotFound(t *testing.T) {
	cs := NewLocalCertSource()
	_, err := cs.GetTrustAnchor(context.Background(), "spoke-never-seen")
	if !errors.Is(err, ErrTrustAnchorNotFound) {
		t.Errorf("expected ErrTrustAnchorNotFound, got %v", err)
	}
}

func TestGetTrustAnchor_AfterIssue_ReturnsSelfSignedCACert(t *testing.T) {
	cs := NewLocalCertSource()
	csrPEM := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")
	if _, err := cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl"); err != nil {
		t.Fatalf("IssueLeafCert: %v", err)
	}

	caCertPEM, err := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	if err != nil {
		t.Fatalf("GetTrustAnchor: %v", err)
	}

	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		t.Fatal("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	if !caCert.IsCA {
		t.Error("CA cert IsCA must be true")
	}
	if !strings.HasPrefix(caCert.Subject.CommonName, "cbweb3-ca-") {
		t.Errorf("CA cert CN should start with 'cbweb3-ca-', got %q", caCert.Subject.CommonName)
	}
}

func TestGetTrustAnchor_Idempotent(t *testing.T) {
	cs := NewLocalCertSource()
	csrPEM := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")
	if _, err := cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl"); err != nil {
		t.Fatalf("IssueLeafCert: %v", err)
	}

	ca1, err := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	if err != nil {
		t.Fatalf("first GetTrustAnchor: %v", err)
	}
	ca2, err := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	if err != nil {
		t.Fatalf("second GetTrustAnchor: %v", err)
	}
	if !bytes.Equal(ca1, ca2) {
		t.Error("GetTrustAnchor should be idempotent: got different PEM on consecutive calls")
	}
}

// ── US1+US2 cross-story ───────────────────────────────────────────────────────

func TestIssueLeafCert_MultipleCalls_SameCA(t *testing.T) {
	cs := NewLocalCertSource()
	csr1 := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")
	csr2 := generateTestCSR(t, "bank-b", "ROLE_COMMERCIAL_BANK")

	cert1, err := cs.IssueLeafCert(context.Background(), csr1, "spoke-brl")
	if err != nil {
		t.Fatalf("first IssueLeafCert: %v", err)
	}
	cert2, err := cs.IssueLeafCert(context.Background(), csr2, "spoke-brl")
	if err != nil {
		t.Fatalf("second IssueLeafCert: %v", err)
	}

	caCertPEM, _ := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	verifyLeafCert(t, cert1, caCertPEM, "bank-a")
	verifyLeafCert(t, cert2, caCertPEM, "bank-b")
}

func TestIssueLeafCert_DifferentSpokes_DifferentCAs(t *testing.T) {
	cs := NewLocalCertSource()

	csr1 := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")
	if _, err := cs.IssueLeafCert(context.Background(), csr1, "spoke-brl"); err != nil {
		t.Fatalf("spoke-brl IssueLeafCert: %v", err)
	}
	csr2 := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")
	if _, err := cs.IssueLeafCert(context.Background(), csr2, "spoke-eur"); err != nil {
		t.Fatalf("spoke-eur IssueLeafCert: %v", err)
	}

	ca1, _ := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	ca2, _ := cs.GetTrustAnchor(context.Background(), "spoke-eur")
	if bytes.Equal(ca1, ca2) {
		t.Error("expected different CAs for different spokes, got identical CAs")
	}
}

// ── US1+US2 concurrency ───────────────────────────────────────────────────────

func TestLocalCertSource_RaceCondition(t *testing.T) {
	cs := NewLocalCertSource()
	csrPEM := generateTestCSR(t, "bank-race", "ROLE_COMMERCIAL_BANK")

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl")
			_, _ = cs.GetTrustAnchor(context.Background(), "spoke-brl")
		}()
	}
	wg.Wait()
}

// ── US3: Factory ──────────────────────────────────────────────────────────────

func TestFactory_SelfSignedURI_ReturnsLocalCertSource(t *testing.T) {
	cs, err := New("self-signed://local")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	csrPEM := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")
	certPEM, err := cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl")
	if err != nil {
		t.Fatalf("IssueLeafCert via factory: %v", err)
	}
	caCertPEM, err := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	if err != nil {
		t.Fatalf("GetTrustAnchor via factory: %v", err)
	}
	verifyLeafCert(t, certPEM, caCertPEM, "bank-a")
}

func TestFactory_ProdURI_ReturnsStubWithErrNotImplemented(t *testing.T) {
	cs, err := New("ca://lnet-pki")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = cs.IssueLeafCert(context.Background(), []byte("irrelevant"), "spoke-brl")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("IssueLeafCert: expected ErrNotImplemented, got %v", err)
	}
	_, err = cs.GetTrustAnchor(context.Background(), "spoke-brl")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("GetTrustAnchor: expected ErrNotImplemented, got %v", err)
	}
}

func TestFactory_SelfSignedBare_ReturnsLocalCertSource(t *testing.T) {
	cs, err := New("self-signed")
	if err != nil {
		t.Fatalf("New(self-signed): %v", err)
	}
	csrPEM := generateTestCSR(t, "bank-a", "ROLE_COMMERCIAL_BANK")
	certPEM, err := cs.IssueLeafCert(context.Background(), csrPEM, "spoke-brl")
	if err != nil {
		t.Fatalf("IssueLeafCert via factory: %v", err)
	}
	caCertPEM, err := cs.GetTrustAnchor(context.Background(), "spoke-brl")
	if err != nil {
		t.Fatalf("GetTrustAnchor via factory: %v", err)
	}
	verifyLeafCert(t, certPEM, caCertPEM, "bank-a")
}

func TestFactory_InvalidURI_ReturnsError(t *testing.T) {
	for _, uri := range []string{"", "http://foo", "lnet-pki", "self-signed-x", "ca:"} {
		_, err := New(uri)
		if err == nil {
			t.Errorf("New(%q): expected error, got nil", uri)
		}
	}
}
