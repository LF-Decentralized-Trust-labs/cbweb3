// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"strings"
	"testing"
)

// testCA generates a fresh self-signed CA for use across these tests.
func testCA(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	c, k, err := GenerateSelfSignedCA("Test Root CA", "TestOrg", 5)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCA: %v", err)
	}
	return c, k
}

func TestGenerateSelfSignedCA(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := testCA(t)
	if !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		t.Error("cert PEM malformed")
	}
	if !strings.Contains(keyPEM, "PRIVATE KEY") {
		t.Error("key PEM malformed")
	}
}

func TestIssueCertificate_AndMetadata(t *testing.T) {
	t.Parallel()
	caCert, caKey := testCA(t)

	issued, err := IssueCertificate(caCert, caKey, CertRequest{
		Subject: "bank-user", Org: "Bank A", Role: "ROLE_COMMERCIAL_BANK", ValidYears: 2,
	})
	if err != nil {
		t.Fatalf("IssueCertificate: %v", err)
	}
	if issued.CertPEM == "" || issued.PrivKeyPEM == "" {
		t.Fatal("issued cert missing PEM material")
	}

	// The issued cert must chain to the CA.
	if err := VerifyChain(issued.CertPEM, caCert); err != nil {
		t.Errorf("issued cert should verify against CA: %v", err)
	}

	// Metadata extraction.
	meta, err := ExtractMetadata(issued.CertPEM)
	if err != nil {
		t.Fatalf("ExtractMetadata: %v", err)
	}
	if meta.CommonName != "bank-user" || meta.Organization != "Bank A" || meta.Role != "ROLE_COMMERCIAL_BANK" {
		t.Errorf("metadata mismatch: %+v", meta)
	}
	if meta.NotAfter.IsZero() {
		t.Error("expected non-zero NotAfter")
	}
}

func TestIssueCertificate_BadCA(t *testing.T) {
	t.Parallel()
	if _, err := IssueCertificate("not-a-cert", "not-a-key", CertRequest{Subject: "x", ValidYears: 1}); err == nil {
		t.Error("expected error for malformed CA inputs")
	}
}

func TestSignCSR_RoundTrip(t *testing.T) {
	t.Parallel()
	caCert, caKey := testCA(t)

	csrPEM, _, err := GenerateCSR("bank-user", "Bank A", "ROLE_COMMERCIAL_BANK", "BR")
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}
	if !strings.Contains(csrPEM, "CERTIFICATE REQUEST") {
		t.Error("CSR PEM malformed")
	}

	issued, err := SignCSR(caCert, caKey, csrPEM, 1)
	if err != nil {
		t.Fatalf("SignCSR: %v", err)
	}
	if issued.CertPEM == "" {
		t.Fatal("signed cert missing")
	}
	if err := VerifyChain(issued.CertPEM, caCert); err != nil {
		t.Errorf("signed cert should chain to CA: %v", err)
	}
	meta, _ := ExtractMetadata(issued.CertPEM)
	if meta.CommonName != "bank-user" {
		t.Errorf("CN = %q, want bank-user", meta.CommonName)
	}
}

func TestSignCSR_InvalidCSR(t *testing.T) {
	t.Parallel()
	caCert, caKey := testCA(t)
	if _, err := SignCSR(caCert, caKey, "garbage", 1); err == nil {
		t.Error("expected error for malformed CSR")
	}
}

func TestSignMessageAndValidateSignature(t *testing.T) {
	t.Parallel()
	caCert, caKey := testCA(t)
	issued, err := IssueCertificate(caCert, caKey, CertRequest{Subject: "signer", Org: "Org", Role: "ROLE", ValidYears: 1})
	if err != nil {
		t.Fatalf("IssueCertificate: %v", err)
	}

	msgHex := "00112233aabbccdd"
	sigHex, err := SignMessage(issued.PrivKeyPEM, msgHex)
	if err != nil {
		t.Fatalf("SignMessage: %v", err)
	}

	// Valid signature verifies against the cert public key.
	if err := ValidateSignature(issued.CertPEM, msgHex, sigHex); err != nil {
		t.Errorf("ValidateSignature should succeed: %v", err)
	}

	// Tampered message fails.
	if err := ValidateSignature(issued.CertPEM, "deadbeef", sigHex); err == nil {
		t.Error("expected verification failure for different message")
	}
}

func TestSignMessage_Errors(t *testing.T) {
	t.Parallel()
	if _, err := SignMessage("not-a-key", "00"); err == nil {
		t.Error("expected error for bad private key")
	}
	_, key := testCA(t)
	if _, err := SignMessage(key, "zz"); err == nil {
		t.Error("expected error for bad message hex")
	}
}

func TestValidateSignature_Errors(t *testing.T) {
	t.Parallel()
	caCert, caKey := testCA(t)
	issued, _ := IssueCertificate(caCert, caKey, CertRequest{Subject: "s", ValidYears: 1})

	// Bad cert.
	if err := ValidateSignature("bad-cert", "00", "00"); err == nil {
		t.Error("expected error for bad cert")
	}
	// Bad message hex.
	if err := ValidateSignature(issued.CertPEM, "zz", "00"); err == nil {
		t.Error("expected error for bad message hex")
	}
	// Bad signature hex.
	if err := ValidateSignature(issued.CertPEM, "00", "zz"); err == nil {
		t.Error("expected error for bad signature hex")
	}
	// Non-DER signature bytes.
	if err := ValidateSignature(issued.CertPEM, "00", "0011"); err == nil {
		t.Error("expected error for non-DER signature")
	}
}

func TestVerifyChain_Failures(t *testing.T) {
	t.Parallel()
	caCert, caKey := testCA(t)
	issued, _ := IssueCertificate(caCert, caKey, CertRequest{Subject: "s", ValidYears: 1})

	// Bad leaf cert.
	if err := VerifyChain("bad", caCert); err == nil {
		t.Error("expected error for bad leaf cert")
	}
	// Bad CA pool input.
	if err := VerifyChain(issued.CertPEM, "not-a-ca"); err == nil {
		t.Error("expected error for bad CA pool")
	}
	// Valid cert against an unrelated CA must fail to chain.
	otherCA, _ := testCA(t)
	if err := VerifyChain(issued.CertPEM, otherCA); err == nil {
		t.Error("expected chain failure against unrelated CA")
	}
}

func TestExtractMetadata_BadCert(t *testing.T) {
	t.Parallel()
	if _, err := ExtractMetadata("not-a-cert"); err == nil {
		t.Error("expected error for bad cert")
	}
}
