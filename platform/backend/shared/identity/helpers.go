package pki

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
)

func parseCertPEM(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, errors.New("pki: failed to decode PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseECPrivKeyPEM(keyPEM string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, errors.New("pki: failed to decode PEM key block")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// randReader returns the crypto/rand reader. Extracted to allow patching in tests.
func randReader() io.Reader {
	return rand.Reader
}
