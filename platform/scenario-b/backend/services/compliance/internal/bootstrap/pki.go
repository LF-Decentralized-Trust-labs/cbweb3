// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"

	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
)

// EnsurePKIFiles generates PKI files (CA, CSR, certificate) for the given
// commercial bank if they do not already exist on disk. Idempotent: existing
// files are never overwritten.
//
// Generated files:
//
//	{bankCode}-ca.crt, {bankCode}-ca.key  — self-signed CA
//	{bankCode}.key, {bankCode}.csr        — participant P-256 key + CSR
//	{bankCode}.crt                        — certificate signed by the CA
//
// The pkiDir must be writable. Key files are created with mode 0600.
func EnsurePKIFiles(pkiDir, bankCode, institutionName, country, role string) error {
	if pkiDir == "" || bankCode == "" {
		return nil
	}

	caCertPath := filepath.Join(pkiDir, bankCode+"-ca.crt")
	caKeyPath := filepath.Join(pkiDir, bankCode+"-ca.key")
	partKeyPath := filepath.Join(pkiDir, bankCode+".key")
	csrPath := filepath.Join(pkiDir, bankCode+".csr")
	certPath := filepath.Join(pkiDir, bankCode+".crt")

	// 1. Ensure CA exists.
	caCertPEM, caKeyPEM, err := ensureCA(caCertPath, caKeyPath, bankCode, institutionName)
	if err != nil {
		return fmt.Errorf("bootstrap/pki: CA: %w", err)
	}

	// 2. Ensure participant CSR + key exist.
	csrPEM, err := ensureCSR(csrPath, partKeyPath, bankCode, institutionName, country, role)
	if err != nil {
		return fmt.Errorf("bootstrap/pki: CSR: %w", err)
	}

	// 3. Ensure signed certificate exists.
	if err := ensureCert(certPath, caCertPEM, caKeyPEM, csrPEM); err != nil {
		return fmt.Errorf("bootstrap/pki: cert: %w", err)
	}

	return nil
}

func ensureCA(certPath, keyPath, bankCode, institutionName string) (certPEM, keyPEM string, err error) {
	if fileExists(certPath) && fileExists(keyPath) {
		log.Printf("bootstrap/pki: CA already exists at %s — skipping", certPath)
		certBytes, err := os.ReadFile(certPath)
		if err != nil {
			return "", "", err
		}
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return "", "", err
		}
		return string(certBytes), string(keyBytes), nil
	}

	cn := bankCode + "-CA"
	org := institutionName
	if org == "" {
		org = bankCode
	}

	certPEM, keyPEM, err = pki.GenerateSelfSignedCA(cn, org, 10)
	if err != nil {
		return "", "", err
	}

	if err := writeFile(certPath, certPEM, 0644); err != nil {
		return "", "", err
	}
	if err := writeFile(keyPath, keyPEM, 0600); err != nil {
		return "", "", err
	}

	log.Printf("bootstrap/pki: generated CA at %s", certPath)
	return certPEM, keyPEM, nil
}

func ensureCSR(csrPath, keyPath, bankCode, institutionName, country, role string) (csrPEM string, err error) {
	if fileExists(csrPath) && fileExists(keyPath) {
		log.Printf("bootstrap/pki: CSR already exists at %s — skipping", csrPath)
		csrBytes, err := os.ReadFile(csrPath)
		if err != nil {
			return "", err
		}
		return string(csrBytes), nil
	}

	// A key WITHOUT a CSR is not "nothing here": it is the state the toolkit's gen-tls step
	// leaves behind, and generating a fresh pair over it silently replaced a private key this
	// service did not create. Its certificate stayed — ensureCert skips an existing file — so
	// {bankCode}.crt and {bankCode}.key ended up holding two different keys, and every
	// issuance from that pair failed with "x509: provided PrivateKey doesn't match parent's
	// PublicKey" weeks later, at the first bank's onboarding.
	//
	// Derive the CSR from the key that is already there instead. That honours what this
	// function's own doc comment has always claimed — existing files are never overwritten.
	if fileExists(keyPath) {
		csrPEM, err := csrFromExistingKey(keyPath, bankCode, institutionName, country, role)
		if err != nil {
			return "", fmt.Errorf("bootstrap/pki: derive CSR from the existing key at %s: %w", keyPath, err)
		}
		if err := writeFile(csrPath, csrPEM, 0644); err != nil {
			return "", err
		}
		log.Printf("bootstrap/pki: reused the existing key at %s and wrote its CSR to %s", keyPath, csrPath)
		return csrPEM, nil
	}

	org := institutionName
	if org == "" {
		org = bankCode
	}
	if country == "" {
		country = "BR"
	}
	if role == "" {
		role = "ROLE_COMMERCIAL_BANK"
	}

	csrPEM, keyPEM, err := pki.GenerateCSR(bankCode, org, role, country)
	if err != nil {
		return "", err
	}

	if err := writeFile(keyPath, keyPEM, 0600); err != nil {
		return "", err
	}
	if err := writeFile(csrPath, csrPEM, 0644); err != nil {
		return "", err
	}

	log.Printf("bootstrap/pki: generated CSR at %s", csrPath)
	return csrPEM, nil
}

func ensureCert(certPath, caCertPEM, caKeyPEM, csrPEM string) error {
	if fileExists(certPath) {
		log.Printf("bootstrap/pki: certificate already exists at %s — skipping", certPath)
		return nil
	}

	if csrPEM == "" || caCertPEM == "" || caKeyPEM == "" {
		return nil // nothing to sign
	}

	issued, err := pki.SignCSR(caCertPEM, caKeyPEM, csrPEM, 5)
	if err != nil {
		return err
	}

	if err := writeFile(certPath, issued.CertPEM, 0644); err != nil {
		return err
	}

	log.Printf("bootstrap/pki: generated certificate at %s", certPath)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeFile(path, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	return os.WriteFile(path, []byte(content), mode)
}

// csrFromExistingKey builds a CSR for a key already on disk, with the same subject shape
// pki.GenerateCSR produces so the two paths are indistinguishable downstream.
//
// It exists so ensureCSR can adopt a key rather than replace it. Written here with the
// standard library rather than added to backend/shared/identity: that package is imported
// by auth and compliance, and a new exported helper there is a wider surface than one
// unexported function needs. (Each scenario carries its OWN copy of that package, so
// adding to it would not have crossed the scenario boundary — that is not the reason.)
//
// The key parse is NOT local, though: it comes from internal/pki, which is where the same
// decision is made at boot. A key this bootstrap adopts but NewCAFromEnv would refuse to
// read is a stack that provisions cleanly and then crash-loops, so the two must accept
// exactly the same set — which two copies of the logic do not guarantee.
func csrFromExistingKey(keyPath, bankCode, institutionName, country, role string) (string, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", fmt.Errorf("no PEM block in %s", keyPath)
	}
	key, err := compliancepki.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	org := institutionName
	if org == "" {
		org = bankCode
	}
	if country == "" {
		country = "BR"
	}
	if role == "" {
		role = "ROLE_COMMERCIAL_BANK"
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         bankCode,
			Organization:       []string{org},
			OrganizationalUnit: []string{role},
			Country:            []string{country},
		},
	}, key)
	if err != nil {
		return "", fmt.Errorf("create CSR: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})), nil
}
