// SPDX-License-Identifier: Apache-2.0

// Package pki is the compliance-orchestrator wrapper around the shared pkg/pki
// library. It loads the Central Bank CA credentials from the environment and
// exposes higher-level operations for issuing participant certificates.
package pki

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
)

// CA holds the loaded Central Bank CA credentials.
type CA struct {
	certPEM string
	keyPEM  string
}

// NewCAFromEnv reads CA_CERT_FILE and CA_KEY_FILE from the environment and
// loads the PEM content. Returns an error if either file is missing.
func NewCAFromEnv() (*CA, error) {
	certFile := os.Getenv("CA_CERT_FILE")
	keyFile := os.Getenv("CA_KEY_FILE")
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("compliance/pki: CA_CERT_FILE and CA_KEY_FILE must be set")
	}

	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("compliance/pki: read CA cert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("compliance/pki: read CA key: %w", err)
	}

	// Refuse a cert and key that are not each other's. Two producers have owned these
	// filenames — the toolkit's gen-tls step and this service's own PKI bootstrap — and when
	// they disagreed the process still started, reported "CA loaded from disk", and failed
	// only at the first bank's onboarding, weeks later, with an x509 error naming neither
	// file. A CA that cannot sign is not a CA; say so here, where the operator is looking.
	if err := assertKeyMatchesCert(string(certPEM), string(keyPEM)); err != nil {
		return nil, fmt.Errorf("compliance/pki: CA_CERT_FILE %s and CA_KEY_FILE %s are not a pair: %w", certFile, keyFile, err)
	}

	return &CA{certPEM: string(certPEM), keyPEM: string(keyPEM)}, nil
}

// assertKeyMatchesCert reports whether keyPEM is the private key of certPEM.
//
// Compared by public key rather than by issuing anything: the check must hold for every
// certificate this CA will ever sign, not for one sample.
func assertKeyMatchesCert(certPEM, keyPEM string) error {
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return fmt.Errorf("no PEM block in the certificate")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse certificate: %w", err)
	}
	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return fmt.Errorf("no PEM block in the key")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse EC private key: %w", err)
	}
	pub, ok := cert.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	if !ok {
		return fmt.Errorf("certificate public key of type %T cannot be compared", cert.PublicKey)
	}
	if !pub.Equal(key.Public()) {
		return fmt.Errorf("the certificate's public key is not the one this private key derives")
	}
	return nil
}

// NewCAFromPEM creates a CA from PEM strings directly. Useful for testing.
func NewCAFromPEM(certPEM, keyPEM string) *CA {
	return &CA{certPEM: certPEM, keyPEM: keyPEM}
}

// CertPEM returns the CA certificate in PEM format (for chain verification).
func (ca *CA) CertPEM() string {
	return ca.certPEM
}

// SignCSR signs a PKCS#10 Certificate Signing Request submitted by a
// participant. The participant's public key and subject from the CSR are
// preserved; the CA enforces the validity period. The caller is responsible
// for persisting the returned CertPEM; no private key is returned.
func (ca *CA) SignCSR(csrPEM string) (pki.IssuedCert, error) {
	issued, err := pki.SignCSR(ca.certPEM, ca.keyPEM, csrPEM, 1)
	if err != nil {
		return pki.IssuedCert{}, fmt.Errorf("compliance/pki: sign CSR: %w", err)
	}
	return issued, nil
}

// IssueParticipantCert issues a signed X.509 certificate for a participant.
func (ca *CA) IssueParticipantCert(userID, institutionName, role string) (pki.IssuedCert, error) {
	issued, err := pki.IssueCertificate(ca.certPEM, ca.keyPEM, pki.CertRequest{
		Subject:    userID,
		Org:        institutionName,
		Role:       role,
		ValidYears: 1,
	})
	if err != nil {
		return pki.IssuedCert{}, fmt.Errorf("compliance/pki: issue cert for %s: %w", userID, err)
	}
	return issued, nil
}
