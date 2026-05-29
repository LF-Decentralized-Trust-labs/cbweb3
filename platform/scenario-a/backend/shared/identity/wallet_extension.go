package pki

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// OIDBlockchainWallet is the custom X.509 extension OID for the blockchain
// wallet address. Uses a placeholder PEN (99999); replace with the actual
// IANA-registered PEN for production.
var OIDBlockchainWallet = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 99999, 1, 1}

// WalletExtension builds a non-critical X.509 extension that encodes the
// 20-byte Ethereum-style wallet address as an ASN.1 OCTET STRING.
// The CA signature covers this extension, making the binding tamper-proof.
func WalletExtension(walletAddr string) (pkix.Extension, error) {
	addrBytes, err := decodeAddress(walletAddr)
	if err != nil {
		return pkix.Extension{}, fmt.Errorf("pki: wallet extension: %w", err)
	}

	raw, err := asn1.Marshal(addrBytes)
	if err != nil {
		return pkix.Extension{}, fmt.Errorf("pki: marshal wallet extension: %w", err)
	}

	return pkix.Extension{
		Id:       OIDBlockchainWallet,
		Critical: false,
		Value:    raw,
	}, nil
}

// ExtractWalletFromCert parses a PEM-encoded certificate and returns the
// blockchain wallet address from the custom OID extension, if present.
// Returns an empty string (no error) when the extension is absent.
func ExtractWalletFromCert(certPEM string) (string, error) {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return "", fmt.Errorf("pki: extract wallet: %w", err)
	}

	for _, ext := range cert.Extensions {
		if ext.Id.Equal(OIDBlockchainWallet) {
			var addrBytes []byte
			if _, err := asn1.Unmarshal(ext.Value, &addrBytes); err != nil {
				return "", fmt.Errorf("pki: unmarshal wallet extension: %w", err)
			}
			if len(addrBytes) != 20 {
				return "", fmt.Errorf("pki: wallet extension has %d bytes, expected 20", len(addrBytes))
			}
			return "0x" + hex.EncodeToString(addrBytes), nil
		}
	}
	return "", nil
}

// CertFingerprint returns the SHA-256 fingerprint of a PEM-encoded certificate
// as a 32-byte array, suitable for on-chain storage.
func CertFingerprint(certPEM string) ([32]byte, error) {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return [32]byte{}, fmt.Errorf("pki: cert fingerprint: %w", err)
	}
	return sha256Raw(cert.Raw), nil
}

func decodeAddress(addr string) ([]byte, error) {
	addr = strings.TrimPrefix(addr, "0x")
	if len(addr) != 40 {
		return nil, errors.New("address must be 40 hex characters (20 bytes)")
	}
	b, err := hex.DecodeString(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex address: %w", err)
	}
	return b, nil
}
