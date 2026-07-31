// Package keyprovider is the custody boundary for the toolkit's blockchain
// (secp256k1) keys. Each entity holds its own key; the boundary exposes only
// public keys / EVM addresses and signatures — private key material never
// crosses into a manifest, state file, or bundle (no-secrets invariant).
//
// Two implementations sit behind the URI factory (New): a local, in-memory
// provider (kms://local-emulator) and a production stub (any other kms://...).
package keyprovider

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

// KeyProvider generates and guards a secp256k1 key per entity id, signs
// 32-byte digests, and exposes public keys. It never returns a private key.
type KeyProvider interface {
	// GenerateKey creates (or returns, idempotently) the key for id and
	// returns its uncompressed public key (65 bytes, 0x04 prefix).
	GenerateKey(ctx context.Context, id string) ([]byte, error)
	// Sign signs a 32-byte digest with id's key, returning a 65-byte [R||S||V].
	Sign(ctx context.Context, id string, digest []byte) ([]byte, error)
	// GetPublicKey returns id's public key (65 bytes) without generating it.
	GetPublicKey(ctx context.Context, id string) ([]byte, error)
}

// LocalKeyExporter is implemented ONLY by the local provider. It exposes the
// private key hex for the backend's local Besu-layer signing path. The
// production provider deliberately does NOT implement this interface.
type LocalKeyExporter interface {
	ExportPrivateKeyHex(id string) (string, error)
}

// Typed, distinguishable errors (no silent failures — Constitution VI).
var (
	ErrNotImplemented = errors.New("keyprovider: not implemented in production stub")
	ErrKeyNotFound    = errors.New("keyprovider: key not found for id")
	ErrInvalidDigest  = errors.New("keyprovider: digest must be 32 bytes")
	ErrUnsupportedURI = errors.New("keyprovider: unsupported factory URI")
)

// EVMAddress derives the checksummed EVM address (0x-hex) from a 65-byte
// uncompressed secp256k1 public key.
func EVMAddress(pubkey []byte) (string, error) {
	pub, err := crypto.UnmarshalPubkey(pubkey)
	if err != nil {
		return "", err
	}
	return crypto.PubkeyToAddress(*pub).Hex(), nil
}
