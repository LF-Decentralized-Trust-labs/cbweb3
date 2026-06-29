// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// genTLSJoinStep generates the commercial bank's Paladin TLS transport cert
// (mode:join). Distinct from the bank's blockchain identity (gen-csr/request-cert):
// this is a self-signed cert for the bank's Paladin gRPC transport, CN/SAN derived
// from the bank id. Each joining bank generates its own cert dynamically — there is
// no shared cert and no fixed bank list (concat.md / spk-02 generate-paladin-certs).
type genTLSJoinStep struct {
	spokeID string
	bankID  string
	dataDir string
}

func newGenTLSJoinStep(spokeID, bankID, dataDir string) Step {
	return &genTLSJoinStep{spokeID: spokeID, bankID: bankID, dataDir: dataDir}
}

func (s *genTLSJoinStep) Name() string { return StepGenTLSJoin }

func (s *genTLSJoinStep) Check(_ context.Context) (bool, error) {
	_, err := os.Stat(filepath.Join(s.dataDir, "paladin", s.bankID, "tls.crt"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *genTLSJoinStep) Run(_ context.Context) error {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate TLS key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}
	host := bankGrpcHostname(s.spokeID, s.bankID)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         host,
			OrganizationalUnit: []string{"ROLE_COMMERCIAL_BANK"},
			Organization:       []string{s.spokeID},
		},
		DNSNames:              []string{host, "localhost"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshal TLS key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	dir := filepath.Join(s.dataDir, "paladin", s.bankID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tls.crt"), certPEM, 0o644); err != nil {
		return fmt.Errorf("write tls.crt: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tls.key"), keyPEM, 0o600); err != nil {
		return fmt.Errorf("write tls.key: %w", err)
	}
	return nil
}
