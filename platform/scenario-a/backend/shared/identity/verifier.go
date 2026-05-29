package pki

import (
	"crypto/x509"
	"errors"
	"fmt"
	"time"
)

// CertMetadata holds the key identity fields extracted from an X.509 certificate.
type CertMetadata struct {
	CommonName   string    // Subject.CN — typically the userID
	Organization string    // Subject.O  — institution name
	Role         string    // Subject.OU — participant role
	NotAfter     time.Time // certificate expiry
}

// VerifyChain verifies that certPEM was signed by (or chains up to) the CA
// represented by caCertPEM.
func VerifyChain(certPEM, caCertPEM string) error {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return fmt.Errorf("pki: parse cert: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caCertPEM)) {
		return errors.New("pki: failed to parse CA cert into pool")
	}

	opts := x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: time.Now().UTC(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("pki: chain verification failed: %w", err)
	}
	return nil
}

// ExtractMetadata parses a PEM certificate and returns its key subject fields.
func ExtractMetadata(certPEM string) (CertMetadata, error) {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return CertMetadata{}, err
	}

	org := ""
	if len(cert.Subject.Organization) > 0 {
		org = cert.Subject.Organization[0]
	}

	role := ""
	if len(cert.Subject.OrganizationalUnit) > 0 {
		role = cert.Subject.OrganizationalUnit[0]
	}

	return CertMetadata{
		CommonName:   cert.Subject.CommonName,
		Organization: org,
		Role:         role,
		NotAfter:     cert.NotAfter,
	}, nil
}
