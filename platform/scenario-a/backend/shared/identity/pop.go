package pki

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// VerifyPoP validates a secp256k1 Proof of Possession: the commercial bank
// signs a nonce with its blockchain private key and the CA verifies that the
// resulting address matches the one derived from the declared public key.
//
// nonceHex is the hex-encoded nonce issued by the CA.
// signatureHex is the hex-encoded 65-byte recoverable signature (R‖S‖V).
// expectedAddr is the "0x…" Ethereum address the bank claims to own.
func VerifyPoP(nonceHex, signatureHex, expectedAddr string) error {
	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil {
		return fmt.Errorf("pop: decode nonce: %w", err)
	}

	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signatureHex, "0x"))
	if err != nil {
		return fmt.Errorf("pop: decode signature: %w", err)
	}
	if len(sigBytes) != 65 {
		return fmt.Errorf("pop: signature must be 65 bytes (got %d)", len(sigBytes))
	}

	digest := sha256.Sum256(nonceBytes)

	pubKey, err := crypto.SigToPub(digest[:], sigBytes)
	if err != nil {
		return fmt.Errorf("pop: recover public key: %w", err)
	}

	recovered := crypto.PubkeyToAddress(*pubKey).Hex()

	if !strings.EqualFold(recovered, expectedAddr) {
		return fmt.Errorf("pop: address mismatch: recovered %s, expected %s", recovered, expectedAddr)
	}
	return nil
}

// DeriveAddress returns the Ethereum address derived from a hex-encoded
// uncompressed secp256k1 public key (65 bytes, 04‖X‖Y) or compressed (33 bytes).
func DeriveAddress(pubKeyHex string) (string, error) {
	pubKeyBytes, err := hex.DecodeString(strings.TrimPrefix(pubKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("pop: decode public key: %w", err)
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		if len(pubKeyBytes) == 33 {
			pubKey, err = crypto.DecompressPubkey(pubKeyBytes)
			if err != nil {
				return "", fmt.Errorf("pop: decompress public key: %w", err)
			}
		} else {
			return "", fmt.Errorf("pop: unmarshal public key: %w", err)
		}
	}
	return crypto.PubkeyToAddress(*pubKey).Hex(), nil
}

// ValidatePubKeyMatchesAddress checks that the given hex public key derives to
// the expected Ethereum address.
func ValidatePubKeyMatchesAddress(pubKeyHex, expectedAddr string) error {
	derived, err := DeriveAddress(pubKeyHex)
	if err != nil {
		return err
	}
	if !strings.EqualFold(derived, expectedAddr) {
		return errors.New("pop: public key does not match the expected wallet address")
	}
	return nil
}
