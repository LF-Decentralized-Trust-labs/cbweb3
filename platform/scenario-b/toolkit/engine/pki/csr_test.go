// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateBankCSR(t *testing.T) {
	dir := t.TempDir()
	keyPath, csrPath, err := GenerateBankCSR("bank-a", "Bank A S.A.", dir)
	if err != nil {
		t.Fatalf("GenerateBankCSR: %v", err)
	}

	// Private key file must be 0600.
	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("key perms = %v, want 0600", fi.Mode().Perm())
	}

	// CSR parses, self-signature checks, subject is correct, key is ECDSA.
	csrPEM, err := os.ReadFile(csrPath)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		t.Fatal("bad CSR PEM block")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificateRequest: %v", err)
	}
	if err := csr.CheckSignature(); err != nil {
		t.Fatalf("CheckSignature: %v", err)
	}
	if csr.Subject.CommonName != "bank-a" {
		t.Fatalf("CN = %q", csr.Subject.CommonName)
	}
	foundOU := false
	for _, ou := range csr.Subject.OrganizationalUnit {
		if ou == RoleCommercialBank {
			foundOU = true
		}
	}
	if !foundOU {
		t.Fatalf("OU missing %s: %v", RoleCommercialBank, csr.Subject.OrganizationalUnit)
	}
	if _, ok := csr.PublicKey.(*ecdsa.PublicKey); !ok {
		t.Fatal("CSR public key is not ECDSA")
	}

	// No CA material is ever produced.
	if _, err := os.Stat(filepath.Join(dir, "bank-a-ca.key")); !os.IsNotExist(err) {
		t.Fatal("unexpected CA key material created")
	}
}
