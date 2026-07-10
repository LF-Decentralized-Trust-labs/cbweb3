package keyprovider

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
)

// SeededDevID is the well-known entity id pre-seeded in the local emulator so
// that kms://local-emulator always has a stable default identity.
const SeededDevID = "local-emulator"

// seededDevKeyHex is a well-known, NON-PRODUCTION development private key. It
// is a fixed dev constant (never used in production paths), present only so the
// seeded emulator id resolves to a stable, reproducible address.
const seededDevKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// defaultSeed is the base seed for deterministic per-id key derivation. A
// distinct seed (factory ?seed=...) yields a distinct, still-deterministic set.
const defaultSeed = "cbweb3b/scenario-b/keyprovider/v1"

// localKeyProvider keeps keys in memory only, derived deterministically per id
// so addresses are stable across runs (useful for genesis alloc / grants).
type localKeyProvider struct {
	mu   sync.Mutex
	seed []byte
	keys map[string]*ecdsa.PrivateKey
}

func newLocal(seed string) *localKeyProvider {
	if seed == "" {
		seed = defaultSeed
	}
	l := &localKeyProvider{
		seed: []byte(seed),
		keys: make(map[string]*ecdsa.PrivateKey),
	}
	if k, err := crypto.HexToECDSA(seededDevKeyHex); err == nil {
		l.keys[SeededDevID] = k
	}
	return l
}

// deriveKey deterministically derives a valid secp256k1 key from seed+id,
// re-hashing on the (astronomically unlikely) chance the scalar is invalid.
func (l *localKeyProvider) deriveKey(id string) *ecdsa.PrivateKey {
	material := crypto.Keccak256(l.seed, []byte(id))
	for {
		k, err := crypto.ToECDSA(material)
		if err == nil {
			return k
		}
		material = crypto.Keccak256(material)
	}
}

// getOrCreate must be called with l.mu held.
func (l *localKeyProvider) getOrCreate(id string) *ecdsa.PrivateKey {
	if k, ok := l.keys[id]; ok {
		return k
	}
	k := l.deriveKey(id)
	l.keys[id] = k
	return k
}

func (l *localKeyProvider) GenerateKey(_ context.Context, id string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.getOrCreate(id)
	return crypto.FromECDSAPub(&k.PublicKey), nil
}

func (l *localKeyProvider) GetPublicKey(_ context.Context, id string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k, ok := l.keys[id]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return crypto.FromECDSAPub(&k.PublicKey), nil
}

func (l *localKeyProvider) Sign(_ context.Context, id string, digest []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, ErrInvalidDigest
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	k, ok := l.keys[id]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return crypto.Sign(digest, k)
}

// ExportPrivateKeyHex is LOCAL-ONLY (satisfies LocalKeyExporter). The private
// key is returned only for the local backend signing path — never serialized
// into any manifest/state/bundle by callers.
func (l *localKeyProvider) ExportPrivateKeyHex(id string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k, ok := l.keys[id]
	if !ok {
		return "", ErrKeyNotFound
	}
	return hex.EncodeToString(crypto.FromECDSA(k)), nil
}
