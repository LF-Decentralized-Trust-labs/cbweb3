// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
)

// generateTestCSR creates an ECDSA P-256 key pair and a PKCS#10 CSR PEM for testing.
// cn is the CommonName; ou is the OrganizationalUnit (e.g. "ROLE_COMMERCIAL_BANK").
func generateTestCSR(t *testing.T, cn, ou string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generateTestCSR: generate key: %v", err)
	}
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         cn,
			OrganizationalUnit: []string{ou},
		},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		t.Fatalf("generateTestCSR: create CSR: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}

// verifyLeafCert asserts that certPEM is a valid certificate signed by the CA in
// caCertPEM and that the leaf Subject.CommonName equals cn.
func verifyLeafCert(t *testing.T, certPEM, caCertPEM []byte, cn string) {
	t.Helper()
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		t.Fatal("verifyLeafCert: failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("verifyLeafCert: parse CA cert: %v", err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		t.Fatal("verifyLeafCert: failed to decode leaf cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("verifyLeafCert: parse leaf cert: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	opts := x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	if _, err := cert.Verify(opts); err != nil {
		t.Errorf("verifyLeafCert: cert.Verify failed: %v", err)
	}
	if cert.Subject.CommonName != cn {
		t.Errorf("verifyLeafCert: CN=%q, want %q", cert.Subject.CommonName, cn)
	}
}
