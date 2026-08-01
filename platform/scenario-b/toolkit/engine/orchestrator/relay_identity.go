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
	"log"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// A central bank's SERVICE identity: the key it signs with and the certificate its peers pin.
//
// Why it cannot come from onboarding. A commercial bank gets its identity from its CB — it generates
// a CSR over PKI_DIR/<bankCode>.key and the CB signs it, which is why the participants table is a
// valid pin source. A CB has nobody to onboard it, so provisioning issues its identity.
//
// Two consumers, one identity:
//
//	relay auth                        signs internal requests when the CB is the SENDER
//	circuit-breaker attestation       signs the institutional attestation server-side, so operators
//	                                  never paste a signature by hand
//
// Both read PKI_DIR/<relay-key-id>.key, and before this the CB had neither: its PKI_DIR was unset,
// so the gateway skipped the attestation signer entirely.
//
// Issued by the CB's OWN CA rather than self-signed. For pinning the two are equivalent — the pin is
// the trust — but a CA-issued leaf is the shape a real CA takes over later: same file, same place,
// different issuer. Self-signing now would have to be undone then.

// generateSelfSignedCA builds a CA certificate and key. Used by tests and by the CA generation path.
func generateSelfSignedCA(commonName string) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, OrganizationalUnit: []string{"CENTRAL_BANK_CA"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create CA certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal CA key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

// issueRelayIdentity mints a fresh P-256 key and a leaf certificate for keyID, signed by the given
// CA. The key is RANDOM, not derived: unlike the blockchain keys (keccak256 of a public salt and a
// public id, reproducible by anyone holding the repository), a service identity is a real secret.
func issueRelayIdentity(caCertPEM, caKeyPEM []byte, keyID string) (keyPEM, certPEM []byte, err error) {
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		return nil, nil, fmt.Errorf("relay identity: CA certificate is not PEM")
	}
	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: parse CA certificate: %w", err)
	}
	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("relay identity: CA key is not PEM")
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: parse CA key: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: keyID, OrganizationalUnit: []string{"SERVICE_IDENTITY"}},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		// NOT a CA: the pin loader refuses CA certificates as peer identities, so a leaf claiming to
		// be one would never be verifiable by anybody.
		IsCA:                  false,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: create certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: marshal key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// ensureRelayIdentity issues the CB's service identity into its PKI volume, once.
//
// Idempotent on the certificate's presence: re-issuing would mint a new key, and any peer that had
// pinned the old one by file would stop verifying this CB.
//
// The key is written 0600. The gateway that reads it runs as root in this image, so a root-owned
// 0600 file is readable — unlike the CA material next to it, which is 0644 because the compliance
// service reads that as a different uid.
func ensureRelayIdentity(ctx context.Context, r exec.CommandRunner, volume, keyID string) error {
	if keyID == "" {
		return fmt.Errorf("relay identity: key id is required")
	}
	if volumeHasFile(ctx, r, volume, keyID+".crt") {
		return nil
	}
	// Prefer a CA-issued leaf, because that is the shape a real CA takes over later: same file, same
	// place, different issuer. But do not DEPEND on it — the CB's CA material was found inconsistent
	// on a live stack: central-bank.crt, central-bank.key and central-bank-ca.* are three different
	// keys, and the certificate that actually signed the banks' credentials has no matching private
	// key in the volume. Coupling this identity to that material would make it fail for a reason that
	// has nothing to do with it.
	//
	// Self-signing is not a downgrade for the current trust model: peers PIN the certificate, so the
	// issuer is never consulted. It becomes CA-issued the moment a coherent CA exists, with no change
	// to filenames or consumers.
	keyPEM, certPEM, err := issueFromVolumeCA(ctx, r, volume, keyID)
	if err != nil {
		log.Printf("[gen-relay-identity] the CB's CA could not issue the service identity for %s (%v) — self-signing instead; peers pin the certificate, so the issuer is not consulted", keyID, err)
		keyPEM, certPEM, err = selfSignRelayIdentity(keyID)
		if err != nil {
			return err
		}
	}
	if err := writeVolumeFile(ctx, r, volume, keyID+".crt", certPEM, "0644"); err != nil {
		return err
	}
	return writeVolumeFile(ctx, r, volume, keyID+".key", keyPEM, "0600")
}

// issueFromVolumeCA reads the CB's CA material and issues the leaf from it. Tries the matched-pair
// naming first (central-bank-ca.*) then the legacy naming (central-bank.*); a pair whose key does not
// belong to its certificate surfaces as an error from x509.CreateCertificate, which is what the caller
// falls back on.
func issueFromVolumeCA(ctx context.Context, r exec.CommandRunner, volume, keyID string) (keyPEM, certPEM []byte, err error) {
	var lastErr error
	for _, base := range []string{"central-bank-ca", "central-bank"} {
		caCertPEM, cErr := readVolumeFile(ctx, r, volume, base+".crt")
		if cErr != nil {
			lastErr = cErr
			continue
		}
		caKeyPEM, kErr := readVolumeFile(ctx, r, volume, base+".key")
		if kErr != nil {
			lastErr = kErr
			continue
		}
		keyPEM, certPEM, err = issueRelayIdentity(caCertPEM, caKeyPEM, keyID)
		if err == nil {
			return keyPEM, certPEM, nil
		}
		lastErr = fmt.Errorf("%s: %w", base, err)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no CA material found in the volume")
	}
	return nil, nil, lastErr
}

// selfSignRelayIdentity mints a standalone service identity. Used when no usable CA is available;
// equivalent for pinning, since the pin is the trust anchor and the issuer is never consulted.
func selfSignRelayIdentity(keyID string) (keyPEM, certPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: keyID, OrganizationalUnit: []string{"SERVICE_IDENTITY"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		IsCA:                  false,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: create certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("relay identity: marshal key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}
