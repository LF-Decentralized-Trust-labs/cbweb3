// SPDX-License-Identifier: Apache-2.0

// Package keyprovider defines the key-management abstraction for the Scenario A
// provisioning toolkit. All secp256k1 key material is generated and held exclusively
// by the provider; callers receive only public keys and signatures.
// Private keys never appear in files, environment variables, or manifests.
package keyprovider

import (
	"context"
	"errors"
	"fmt"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// KeyProvider is the key-management boundary for participant blockchain keys.
type KeyProvider interface {
	// GenerateKey creates and stores a secp256k1 key pair for id.
	// Returns the uncompressed 65-byte public key.
	// Idempotent: if a key for id already exists, returns the existing pubkey.
	GenerateKey(ctx context.Context, id string) (pubkey []byte, err error)

	// Sign signs the 32-byte digest using the key identified by id.
	// Returns a 65-byte Ethereum-compatible signature (r || s || v).
	Sign(ctx context.Context, id string, digest []byte) (sig []byte, err error)

	// GetPublicKey returns the uncompressed 65-byte public key for id.
	// Returns ErrKeyNotFound if no key exists for id.
	GetPublicKey(ctx context.Context, id string) (pubkey []byte, err error)
}

var (
	// ErrKeyNotFound is returned when no key exists for the requested id.
	ErrKeyNotFound = errors.New("keyprovider: key not found")
	// ErrNotImplemented is returned by the production stub until PR-1 is complete.
	ErrNotImplemented = errors.New("keyprovider: not implemented")
	// ErrInvalidDigest is returned when Sign receives a payload that is not exactly 32 bytes.
	ErrInvalidDigest = errors.New("keyprovider: digest must be exactly 32 bytes")
)

// EVMAddress derives the EVM address from an uncompressed 65-byte secp256k1 public key.
// Returns the EIP-55 checksummed hex address (e.g. "0xAbCd...").
func EVMAddress(pubkey []byte) (string, error) {
	if len(pubkey) != 65 {
		return "", fmt.Errorf("keyprovider: EVMAddress requires 65-byte uncompressed public key, got %d bytes", len(pubkey))
	}
	ecKey, err := gethcrypto.UnmarshalPubkey(pubkey)
	if err != nil {
		return "", fmt.Errorf("keyprovider: EVMAddress: %w", err)
	}
	return gethcrypto.PubkeyToAddress(*ecKey).Hex(), nil
}
