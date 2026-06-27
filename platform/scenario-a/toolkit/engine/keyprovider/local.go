// SPDX-License-Identifier: Apache-2.0

package keyprovider

import (
	"context"
	"crypto/ecdsa"
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
