// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"sync"
	"time"
)

// spokeCA is a per-spoke self-signed CA held ONLY in memory. The key is never
// serialized.
type spokeCA struct {
	key  *ecdsa.PrivateKey
	cert *x509.Certificate
}

// localCertSource keeps one CA per spoke, created lazily on first issuance.
type localCertSource struct {
	mu  sync.Mutex
	cas map[string]*spokeCA
}

func newLocal() *localCertSource {
	return &localCertSource{cas: make(map[string]*spokeCA)}
}

// caFor lazily creates the per-spoke CA. Must be called with l.mu held.
func (l *localCertSource) caFor(spokeID string) (*spokeCA, error) {
	if ca, ok := l.cas[spokeID]; ok {
		return ca, nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := randSerial()
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         "spoke-" + spokeID + "-ca",
			OrganizationalUnit: []string{"CENTRAL_BANK_CA"},
			Country:            []string{"BR"},
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	ca := &spokeCA{key: key, cert: cert}
	l.cas[spokeID] = ca
	return ca, nil
}

// IssueLeafCert validates in the order mandated by FR-009 BEFORE the CA is
// touched, so a rejected CSR never lazily creates a spoke CA.
func (l *localCertSource) IssueLeafCert(_ context.Context, csrPEM []byte, spokeID string) ([]byte, error) {
	csr, err := parseAndVerifyCSR(csrPEM)
	if err != nil {
		return nil, err
	}
	pub, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return nil, ErrUnsupportedKeyAlgorithm
	}
	if !hasRole(csr.Subject.OrganizationalUnit, roleCommercialBank) {
		return nil, ErrForbiddenRole
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	ca, err := l.caFor(spokeID)
	if err != nil {
		return nil, err
	}
	serial, err := randSerial()
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IsCA:         false,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, csr.PublicKey, ca.key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

func (l *localCertSource) GetTrustAnchor(_ context.Context, spokeID string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ca, ok := l.cas[spokeID]
	if !ok {
		return nil, ErrTrustAnchorNotFound
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw}), nil
}
