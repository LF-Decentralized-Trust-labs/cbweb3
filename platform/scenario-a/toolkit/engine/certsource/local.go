// SPDX-License-Identifier: Apache-2.0

package certsource

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// spokeCert holds the in-memory CA key pair and certificate for a single spoke.
// The private key never leaves this struct; it is used only to sign leaf certificates.
type spokeCert struct {
	key  *ecdsa.PrivateKey
	cert *x509.Certificate
}

// LocalCertSource is an in-memory PKI provider for local and CI use.
// It generates a self-signed ECDSA P-256 CA per spoke on first use (via IssueLeafCert)
// and holds all key material exclusively in process memory.
// Each instance starts with an empty spoke store; keys are never written to disk.
type LocalCertSource struct {
	mu           sync.RWMutex
	spokes       map[string]*spokeCert
	leafValidity time.Duration
}

// NewLocalCertSource returns a LocalCertSource with an empty in-memory spoke store
// and a default leaf certificate validity of one year.
func NewLocalCertSource() *LocalCertSource {
	return &LocalCertSource{
		spokes:       make(map[string]*spokeCert),
		leafValidity: 365 * 24 * time.Hour,
	}
}

var _ CertSource = (*LocalCertSource)(nil)

// IssueLeafCert signs csrPEM with the CA of spokeID and returns the signed
// certificate PEM. The spoke CA is generated on first use (double-check locking).
func (c *LocalCertSource) IssueLeafCert(_ context.Context, csrPEM []byte, spokeID string) ([]byte, error) {
	// 1. Decode PEM block.
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, ErrInvalidCSR
	}

	// 2. Parse the CSR DER.
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, ErrInvalidCSR
	}

	// 3. Validate CSR self-signature.
	if err := csr.CheckSignature(); err != nil {
		return nil, ErrInvalidCSR
	}

	// 4. Require ECDSA P-256.
	ecKey, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok || ecKey.Curve != elliptic.P256() {
		return nil, ErrUnsupportedKeyAlgorithm
	}

	// 5. Require ROLE_COMMERCIAL_BANK in subject OU.
	if !hasRole(csr.Subject.OrganizationalUnit, "ROLE_COMMERCIAL_BANK") {
		return nil, ErrForbiddenRole
	}

	// 6. Get or create the spoke CA (double-check locking — same pattern as LocalKeyProvider).
	sc, err := c.getOrCreateSpoke(spokeID)
	if err != nil {
		return nil, fmt.Errorf("certsource: init spoke %q: %w", spokeID, err)
	}

	// 7. Build the leaf certificate template.
	serial, err := randomSerial()
	if err != nil {
		return nil, fmt.Errorf("certsource: generate serial: %w", err)
	}
	now := time.Now()
	leafTemplate := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               csr.Subject,
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(c.leafValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// 8. Sign with the spoke CA.
	der, err := x509.CreateCertificate(cryptorand.Reader, leafTemplate, sc.cert, csr.PublicKey, sc.key)
	if err != nil {
		return nil, fmt.Errorf("certsource: create certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// GetTrustAnchor returns the PEM-encoded CA certificate for spokeID.
// It never initialises a spoke CA — only IssueLeafCert does that.
func (c *LocalCertSource) GetTrustAnchor(_ context.Context, spokeID string) ([]byte, error) {
	c.mu.RLock()
	sc, ok := c.spokes[spokeID]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTrustAnchorNotFound, spokeID)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: sc.cert.Raw}), nil
}

// ── private helpers ───────────────────────────────────────────────────────────

// getOrCreateSpoke returns the existing spoke CA or creates a new one.
// Uses double-check locking to prevent concurrent duplicate CA generation.
func (c *LocalCertSource) getOrCreateSpoke(spokeID string) (*spokeCert, error) {
	c.mu.RLock()
	sc, ok := c.spokes[spokeID]
	c.mu.RUnlock()
	if ok {
		return sc, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if sc, ok := c.spokes[spokeID]; ok {
		return sc, nil
	}
	sc, err := initSpoke(spokeID)
	if err != nil {
		return nil, err
	}
	c.spokes[spokeID] = sc
	return sc, nil
}

// initSpoke generates a new self-signed ECDSA P-256 CA for spokeID.
// Called under write lock; never writes any file.
func initSpoke(spokeID string) (*spokeCert, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, fmt.Errorf("generate CA serial: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "cbweb3-ca-" + spokeID},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(cryptorand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}
	return &spokeCert{key: key, cert: cert}, nil
}

// randomSerial returns a random 128-bit serial number suitable for X.509 certs.
func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return cryptorand.Int(cryptorand.Reader, limit)
}

// hasRole reports whether ous contains role.
func hasRole(ous []string, role string) bool {
	for _, ou := range ous {
		if ou == role {
			return true
		}
	}
	return false
}
