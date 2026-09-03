// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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
