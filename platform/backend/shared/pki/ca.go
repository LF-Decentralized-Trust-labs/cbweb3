// Package pki provides X.509 PKI utilities for the CBWeb3 platform.
// It covers certificate issuance (CA), chain verification and ECDSA signature
// validation. All cryptographic operations use P-256 (ECDSA) for X.509
// compatibility; secp256k1 EVM wallet operations remain in the identity/kms
// package.
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// CertRequest holds the parameters for issuing a participant certificate.
type CertRequest struct {
	// Subject is stored as CN (typically the userID or institution identifier).
	Subject string
	// Org is the institution name, stored as O.
	Org string
	// Role is the participant role, stored as OU.
	Role string
	// ValidYears is the certificate validity period in years (default: 1).
	ValidYears int
}

// IssuedCert contains the issued certificate and the newly generated key pair.
// The caller is responsible for persisting PrivKeyPEM securely (e.g. KMS).
type IssuedCert struct {
	CertPEM    string // PEM-encoded X.509 certificate
	PrivKeyPEM string // PEM-encoded EC PRIVATE KEY (PKCS8)
}

// IssueCertificate signs a new X.509 ECDSA-P256 end-entity certificate using
// the provided CA certificate and key (both PEM-encoded).
func IssueCertificate(caCertPEM, caKeyPEM string, req CertRequest) (IssuedCert, error) {
	caCert, err := parseCertPEM(caCertPEM)
	if err != nil {
		return IssuedCert{}, fmt.Errorf("pki: parse CA cert: %w", err)
	}

	caKey, err := parseECPrivKeyPEM(caKeyPEM)
	if err != nil {
		return IssuedCert{}, fmt.Errorf("pki: parse CA key: %w", err)
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return IssuedCert{}, fmt.Errorf("pki: generate key: %w", err)
	}

	validYears := req.ValidYears
	if validYears <= 0 {
		validYears = 1
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return IssuedCert{}, fmt.Errorf("pki: generate serial: %w", err)
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         req.Subject,
			Organization:       []string{req.Org},
			OrganizationalUnit: []string{req.Role},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(validYears, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &privKey.PublicKey, caKey)
	if err != nil {
		return IssuedCert{}, fmt.Errorf("pki: create certificate: %w", err)
	}

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	privKeyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return IssuedCert{}, fmt.Errorf("pki: marshal private key: %w", err)
	}
	privKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privKeyDER}))

	return IssuedCert{CertPEM: certPEM, PrivKeyPEM: privKeyPEM}, nil
}

// GenerateSelfSignedCA creates a new self-signed CA certificate and key pair.
// Intended for the Central Bank's root CA bootstrap and for testing.
func GenerateSelfSignedCA(commonName, org string, validYears int) (certPEM, keyPEM string, err error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("pki: generate CA key: %w", err)
	}

	if validYears <= 0 {
		validYears = 10
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("pki: generate serial: %w", err)
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{org},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(validYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return "", "", fmt.Errorf("pki: create CA certificate: %w", err)
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return "", "", fmt.Errorf("pki: marshal CA key: %w", err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))

	return certPEM, keyPEM, nil
}
