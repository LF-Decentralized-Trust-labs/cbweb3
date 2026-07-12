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
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// genCBCA generates the central bank's self-signed CA (ECDSA P-256) and seeds
// central-bank.crt + central-bank.key into the given named volume, from memory
// (writeVolumeFile — no host file). The compliance service mounts this volume
// and signs participant CSRs with it (CA_CERT_FILE/CA_KEY_FILE); auth may read
// the cert for PKI validation. Mirrors scenario-a's gen-tls standard.
//
// The CA key IS persisted (unlike the no-secrets certsource) because the
// compliance signer needs it at runtime; this is local-dev CA material only.
// Non-destructive: an existing central-bank.crt in the volume is preserved.
func genCBCA(ctx context.Context, r exec.CommandRunner, volume string) error {
	if volumeHasFile(ctx, r, volume, "central-bank.crt") {
		return nil // preserve an existing CA
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("gen-tls: generate CA key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("gen-tls: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         "central-bank-ca",
			OrganizationalUnit: []string{"CENTRAL_BANK_CA"},
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("gen-tls: create certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("gen-tls: marshal key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := writeVolumeFile(ctx, r, volume, "central-bank.crt", certPEM, "0644"); err != nil {
		return err
	}
	// 0644 (not 0600): writeVolumeFile writes as root, but compliance reads the key
	// as a different uid — a root-owned 0600 file would be unreadable (scenario-a
	// hit exactly this). Local-dev CA material only.
	return writeVolumeFile(ctx, r, volume, "central-bank.key", keyPEM, "0644")
}
