// SPDX-License-Identifier: Apache-2.0

// Package pki — PKI contract tests for the Scenario A provisioning toolkit.
//
// These tests define the REQUIRED behavior of the TK-9 commercial-bank join
// flow. They are in the RED phase (failing) until TK-9 implements the functions
// in csr.go. Do NOT remove or weaken these tests — they enforce Constitution
// Principle V (test-first) and the PKI trust model documented in spec 016.
//
// FIX-1 spec: specs/016-fix-pki-local-ca/spec.md
// Plan:       specs/016-fix-pki-local-ca/plan.md
// Data model: specs/016-fix-pki-local-ca/data-model.md
package pki

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGenerateCSR_NoCAKeyCreated verifies CONTRACT: GenerateBankCSR must never
// create a CA keypair (*-ca.key, *-ca.crt) in the output directory.
// Commercial banks MUST NOT hold CA credentials (spec SC-003, FR-004).
func TestGenerateCSR_NoCAKeyCreated(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()

	if err := GenerateBankCSR("bank-x", "Test Bank X", outDir); err != nil {
		t.Fatalf("GenerateBankCSR returned error: %v", err)
	}

	// Enumerate all CA-like files — must be empty.
	caFiles, err := filepath.Glob(filepath.Join(outDir, "*-ca.*"))
	if err != nil {
		t.Fatalf("glob error: %v", err)
	}
	if len(caFiles) > 0 {
		t.Errorf("found prohibited CA artifact(s): %v — commercial banks must not hold CA credentials", caFiles)
	}
}

// TestGenerateCSR_ProducesKeyAndCSR verifies that GenerateBankCSR creates exactly:
//   - {bankCode}.key  (PEM EC PRIVATE KEY)
//   - {bankCode}.csr  (PEM CERTIFICATE REQUEST, OU=ROLE_COMMERCIAL_BANK)
func TestGenerateCSR_ProducesKeyAndCSR(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	bankCode := "bank-test"
	institution := "Test Institution"

	if err := GenerateBankCSR(bankCode, institution, outDir); err != nil {
		t.Fatalf("GenerateBankCSR returned error: %v", err)
	}

	// Verify key file.
	keyPath := filepath.Join(outDir, bankCode+".key")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", keyPath, err)
	}
	if !strings.Contains(string(keyData), "PRIVATE KEY") {
		t.Errorf("key file does not contain PRIVATE KEY PEM block")
	}

	// Verify CSR file and its subject.
	csrPath := filepath.Join(outDir, bankCode+".csr")
	csrData, err := os.ReadFile(csrPath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", csrPath, err)
	}
	block, _ := pem.Decode(csrData)
	if block == nil {
		t.Fatal("CSR file does not contain a valid PEM block")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CSR: %v", err)
	}
	if csr.Subject.CommonName != bankCode {
		t.Errorf("CSR CN = %q, want %q", csr.Subject.CommonName, bankCode)
	}
	foundRole := false
	for _, ou := range csr.Subject.OrganizationalUnit {
		if ou == "ROLE_COMMERCIAL_BANK" {
			foundRole = true
			break
		}
	}
	if !foundRole {
		t.Errorf("CSR OU does not contain ROLE_COMMERCIAL_BANK; got %v", csr.Subject.OrganizationalUnit)
	}
}

// TestSubmitCSR_FailsFastWhenCBUnreachable verifies that SubmitCSRToCB returns a
// non-nil error when the CB endpoint is unreachable, and does NOT create any CA
// key as a fallback (spec SC-005, FR-006).
func TestSubmitCSR_FailsFastWhenCBUnreachable(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	csrPath := filepath.Join(outDir, "bank-x.csr")
	// Write a placeholder CSR file so the function can attempt to read it.
	if err := os.WriteFile(csrPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("setup: write placeholder CSR: %v", err)
	}

	_, err := SubmitCSRToCB(csrPath, "http://127.0.0.1:1", 2*time.Second)
	if err == nil {
		t.Error("expected error when CB is unreachable, got nil")
	}

	// No CA key must have been created as fallback.
	caFiles, _ := filepath.Glob(filepath.Join(outDir, "*-ca.*"))
	if len(caFiles) > 0 {
		t.Errorf("found prohibited CA fallback artifact(s): %v", caFiles)
	}
}

// TestSubmitCSR_FailsFastWhenCBReturnsError verifies that SubmitCSRToCB returns a
// non-nil error when the CB returns a non-2xx status (spec SC-005, FR-006).
func TestSubmitCSR_FailsFastWhenCBReturnsError(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	csrPath := filepath.Join(outDir, "bank-x.csr")
	if err := os.WriteFile(csrPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("setup: write placeholder CSR: %v", err)
	}

	// Simulate a CB that returns 500 Internal Server Error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := SubmitCSRToCB(csrPath, srv.URL, 5*time.Second)
	if err == nil {
		t.Error("expected error when CB returns 500, got nil")
	}

	// No CA key fallback.
	caFiles, _ := filepath.Glob(filepath.Join(outDir, "*-ca.*"))
	if len(caFiles) > 0 {
		t.Errorf("found prohibited CA fallback artifact(s): %v", caFiles)
	}
}

// TestIssuedCert_IssuerIsCBCA verifies that a certificate returned by SubmitCSRToCB
// has its Issuer set to the CB CA (not the bank itself), confirming the single-tier
// CB-issued trust model (spec SC-002, FR-004, data-model.md).
func TestIssuedCert_IssuerIsCBCA(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()

	// Step 1: generate a real CSR so we have a valid csrPath.
	if err := GenerateBankCSR("bank-x", "Test Bank X", outDir); err != nil {
		t.Fatalf("GenerateBankCSR: %v", err)
	}
	csrPath := filepath.Join(outDir, "bank-x.csr")

	// Step 2: generate a test CB CA (self-signed, simulates central-bank-x-ca).
	caCertPEM, caKeyPEM := testCBCA(t)

	// Step 3: start a test HTTP server that signs the CSR with the test CB CA.
	srv := httptest.NewServer(newTestCBSigningServer(t, caCertPEM, caKeyPEM))
	defer srv.Close()

	// Step 4: submit CSR and get the cert.
	certPEM, err := SubmitCSRToCB(csrPath, srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("SubmitCSRToCB: %v", err)
	}

	// Step 5: parse the cert and verify the issuer.
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatal("returned certPEM does not contain a valid PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse issued cert: %v", err)
	}

	// Parse the CB CA to get its subject (which must equal the cert's issuer).
	caBlock, _ := pem.Decode([]byte(caCertPEM))
	if caBlock == nil {
		t.Fatal("testCBCA returned invalid PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("parse test CA cert: %v", err)
	}

	if cert.Issuer.CommonName != caCert.Subject.CommonName {
		t.Errorf("cert issuer CN = %q, want %q (CB CA CN) — bank must not self-sign",
			cert.Issuer.CommonName, caCert.Subject.CommonName)
	}
	if cert.IsCA {
		t.Error("issued cert has isCA=true — leaf certs for banks must have isCA=false")
	}
}
