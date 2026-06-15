// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

// ValidateSignature verifies that signatureHex is a valid ECDSA-P256 signature
// over messageHex (both hex-encoded), produced by the private key whose
// corresponding public key is embedded in certPEM.
//
// The signature must be in DER-encoded ASN.1 format (standard Go/OpenSSL output).
// The message is hashed with SHA-256 before verification.
func ValidateSignature(certPEM, messageHex, signatureHex string) error {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return fmt.Errorf("pki: parse cert: %w", err)
	}

	ecPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("pki: certificate does not contain an ECDSA public key")
	}

	msgBytes, err := hex.DecodeString(messageHex)
	if err != nil {
		return fmt.Errorf("pki: decode message hex: %w", err)
	}

	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("pki: decode signature hex: %w", err)
	}

	digest := sha256.Sum256(msgBytes)

	var sig struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(sigBytes, &sig); err != nil {
		return fmt.Errorf("pki: parse DER signature: %w", err)
	}

	if !ecdsa.Verify(ecPub, digest[:], sig.R, sig.S) {
		return errors.New("pki: signature verification failed")
	}
	return nil
}

// SignMessage signs messageHex with the EC private key from privKeyPEM and
// returns a hex-encoded DER signature. Intended for testing and key bootstrap.
func SignMessage(privKeyPEM, messageHex string) (string, error) {
	privKey, err := parseECPrivKeyPEM(privKeyPEM)
	if err != nil {
		return "", fmt.Errorf("pki: parse private key: %w", err)
	}

	msgBytes, err := hex.DecodeString(messageHex)
	if err != nil {
		return "", fmt.Errorf("pki: decode message hex: %w", err)
	}

	digest := sha256.Sum256(msgBytes)

	r, s, err := ecdsa.Sign(randReader(), privKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("pki: sign: %w", err)
	}

	sigDER, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	if err != nil {
		return "", fmt.Errorf("pki: marshal DER signature: %w", err)
	}

	return hex.EncodeToString(sigDER), nil
}
