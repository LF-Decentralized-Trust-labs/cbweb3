// SPDX-License-Identifier: Apache-2.0

// Package pki provides certificate-request helpers for the toolkit. It
// generates the bank-side keypair + CSR consumed by the CertSource (the bank
// holds its own key; the central bank signs the leaf). It never generates CA
// material.
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"path/filepath"
)

// RoleCommercialBank is the OU the CertSource requires on a leaf CSR.
const RoleCommercialBank = "ROLE_COMMERCIAL_BANK"

// GenerateBankCSR generates an ECDSA P-256 keypair and a PKCS#10 CSR for a
// commercial bank, writing <outDir>/<bankCode>.key (0600) and
// <outDir>/<bankCode>.csr. Subject: CN=<bankCode>, O=<institution>,
// OU=ROLE_COMMERCIAL_BANK, C=BR. It never generates CA material.
func GenerateBankCSR(bankCode, institution, outDir string) (keyPath, csrPath string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	tmpl := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         bankCode,
			Organization:       []string{institution},
			OrganizationalUnit: []string{RoleCommercialBank},
			Country:            []string{"BR"},
		},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &tmpl, key)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}

	keyPath = filepath.Join(outDir, bankCode+".key")
	csrPath = filepath.Join(outDir, bankCode+".csr")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(csrPath, csrPEM, 0o644); err != nil {
		return "", "", err
	}
	return keyPath, csrPath, nil
}
