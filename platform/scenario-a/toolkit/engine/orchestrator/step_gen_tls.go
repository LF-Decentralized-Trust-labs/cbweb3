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
	// found generates ONLY the central bank's Paladin cert. Each commercial bank
	// generates its own cert dynamically at join time (mode:join), per the
	// dynamic-registration architecture (concat.md / spk-02 join-paladin.sh) —
	// the found step must not bake in a fixed set of bank nodes.
	// The Paladin gRPC transport authenticates a peer by matching the cert's TLS
	// identity against the EXPECTED NODE NAME from the registry (PD030011), not the
	// dial hostname. The node is registered as cbNodeName (e.g. "spoke-brl-cb"), so
	// the cert CN must be that node name — using the container hostname
	// ("paladin-spoke-brl-cb") makes the mutual-TLS handshake fail. The hostname is
	// kept in the SAN so the dns:/// dial still validates.
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         cbNodeName(s.spokeID),
			OrganizationalUnit: []string{"ROLE_CENTRAL_BANK"},
			Organization:       []string{s.spokeID},
		},
		DNSNames: []string{
			cbNodeName(s.spokeID),
			cbGrpcHostname(s.spokeID),
			"localhost",
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
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshal TLS key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Write the cert/key where the engine and the CB Paladin compose expect them.
	// Legacy tls/central-bank.{crt,key} is kept; the CB Paladin node dir gets
	// tls.{crt,key} (mounted into /etc/paladin; the config references
	// /etc/paladin/tls.{crt,key}).
	targets := []struct{ dir, certName, keyName string }{
		{filepath.Join(s.dataDir, "tls"), "central-bank.crt", "central-bank.key"},
		{filepath.Join(s.dataDir, "paladin", "central-bank"), "tls.crt", "tls.key"},
	}
	for _, t := range targets {
		if err := os.MkdirAll(t.dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", t.dir, err)
		}
		if err := os.WriteFile(filepath.Join(t.dir, t.certName), certPEM, 0o644); err != nil {
			return fmt.Errorf("write cert %s: %w", t.certName, err)
		}
		if err := os.WriteFile(filepath.Join(t.dir, t.keyName), keyPEM, 0o600); err != nil {
			return fmt.Errorf("write key %s: %w", t.keyName, err)
		}
	}

	_ = ctx
	return nil
}
