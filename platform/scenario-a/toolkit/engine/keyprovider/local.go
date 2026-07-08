// SPDX-License-Identifier: Apache-2.0

package keyprovider

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"sync"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// LocalKeyProvider is an in-memory secp256k1 key provider for local and CI use.
// Key material is held exclusively in process memory and is never written to disk,
// environment variables, or logs. Each instance starts with an empty key store.
type LocalKeyProvider struct {
	mu   sync.RWMutex
	keys map[string]*ecdsa.PrivateKey
}

// NewLocalKeyProvider returns a LocalKeyProvider with an empty in-memory key store.
func NewLocalKeyProvider() *LocalKeyProvider {
	return &LocalKeyProvider{keys: make(map[string]*ecdsa.PrivateKey)}
}

// LocalOperatorKeyID is the reserved id of the local-profile funded operator key.
// The provisioning engine signs all bootstrap on-chain transactions (deploy,
// registerIdentity/setIdentityProperty, registerParticipant, faucet) under this
// id — never holding raw key material itself. In prod this same id resolves to a
// real KMS-managed key, funded out-of-band; no engine code changes.
const LocalOperatorKeyID = "local-operator"

// localFundedOperatorKeyHex is the well-known Hyperledger Besu dev account
// (0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73), pre-funded in the spoke genesis
// alloc and the deployer/owner of the spoke contracts. It is a PUBLIC test vector
// (published in Besu docs and the reference network), not a secret, and is used
// ONLY by the local emulator. It lives here — inside the key-custodian boundary —
// so the orchestrator never embeds key material.
const localFundedOperatorKeyHex = "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63"

// NewLocalKeyProviderSeeded returns a LocalKeyProvider seeded with the local
// funded operator key under LocalOperatorKeyID. Used for the local profile.
func NewLocalKeyProviderSeeded() *LocalKeyProvider {
	p := NewLocalKeyProvider()
	if k, err := gethcrypto.HexToECDSA(localFundedOperatorKeyHex); err == nil {
		p.keys[LocalOperatorKeyID] = k
	}
	return p
}

var _ KeyProvider = (*LocalKeyProvider)(nil)

// GenerateKey creates a secp256k1 key pair for id, or returns the existing pubkey
// if one was already generated for this id. Uses double-check under write-lock to
// prevent TOCTOU races under concurrent calls with the same id.
func (p *LocalKeyProvider) GenerateKey(_ context.Context, id string) ([]byte, error) {
	p.mu.RLock()
	if k, ok := p.keys[id]; ok {
		pub := gethcrypto.FromECDSAPub(&k.PublicKey)
		p.mu.RUnlock()
		return pub, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	if k, ok := p.keys[id]; ok {
		return gethcrypto.FromECDSAPub(&k.PublicKey), nil
	}
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	p.keys[id] = key
	return gethcrypto.FromECDSAPub(&key.PublicKey), nil
}

// Sign signs the 32-byte digest with the key for id.
// Returns ErrInvalidDigest if digest is not exactly 32 bytes.
// Returns ErrKeyNotFound if no key exists for id.
func (p *LocalKeyProvider) Sign(_ context.Context, id string, digest []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, ErrInvalidDigest
	}
	p.mu.RLock()
	key, ok := p.keys[id]
	p.mu.RUnlock()
	if !ok {
		return nil, ErrKeyNotFound
	}
	return gethcrypto.Sign(digest, key)
}

// ExportPrivateKeyHex returns the hex-encoded (no 0x prefix) private key for id.
// Returns ErrKeyNotFound if no key exists for id.
//
// This exists ONLY on the local emulator: the local funded operator key is a
// public Besu test vector (see localFundedOperatorKeyHex), and the local backend
// stack — like the legacy local compose (CB_PRIVATE_KEY in .env.infra) — needs a
// raw operator key to sign Besu-layer transactions (HTLC/fCeBM). The prod provider
// does NOT implement this: prod key material never leaves the KMS boundary, and the
// engine gates the Besu-signing env wiring on this method being available.
func (p *LocalKeyProvider) ExportPrivateKeyHex(id string) (string, error) {
	p.mu.RLock()
	key, ok := p.keys[id]
	p.mu.RUnlock()
	if !ok {
		return "", ErrKeyNotFound
	}
	return hex.EncodeToString(gethcrypto.FromECDSA(key)), nil
}

// GetPublicKey returns the uncompressed 65-byte public key for id.
// Returns ErrKeyNotFound if no key exists for id.
func (p *LocalKeyProvider) GetPublicKey(_ context.Context, id string) ([]byte, error) {
	p.mu.RLock()
	key, ok := p.keys[id]
	p.mu.RUnlock()
	if !ok {
		return nil, ErrKeyNotFound
	}
	return gethcrypto.FromECDSAPub(&key.PublicKey), nil
}
