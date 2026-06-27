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

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

type genTLSStep struct {
	spokeID     string
	dataDir     string
	keyProvider keyprovider.KeyProvider
}

func newGenTLSStep(spokeID, dataDir string, _ interface{}, kp keyprovider.KeyProvider) Step {
	return &genTLSStep{spokeID: spokeID, dataDir: dataDir, keyProvider: kp}
}

func (s *genTLSStep) Name() string { return StepGenTLS }

func (s *genTLSStep) Check(_ context.Context) (bool, error) {
	_, err := os.Stat(filepath.Join(s.dataDir, "tls", "central-bank.crt"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *genTLSStep) Run(ctx context.Context) error {
	tlsDir := filepath.Join(s.dataDir, "tls")
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir tls: %w", err)
	}

	// Generate an ephemeral ECDSA P-256 key for the TLS certificate.
	// The CB blockchain key (for signing transactions) is held by KeyProvider.
	// The TLS key is a transport-layer key generated fresh here.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate TLS key: %w", err)
	}

	// Build a self-signed X.509 certificate for the central-bank node.
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         "paladin-" + s.spokeID + "-cb",
			OrganizationalUnit: []string{"ROLE_CENTRAL_BANK"},
			Organization:       []string{s.spokeID},
		},
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

	// Write certificate.
	certPath := filepath.Join(tlsDir, "central-bank.crt")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("encode cert: %w", err)
	}

	// Write private key.
	keyPath := filepath.Join(tlsDir, "central-bank.key")
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshal TLS key: %w", err)
	}
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("encode key: %w", err)
	}

	_ = ctx
	return nil
}
