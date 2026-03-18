package kmsproviders

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/kms"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// KMSLocal implements kms.Provider using in-memory secp256k1 keys.
// Intended for development and testing only.
type KMSLocal struct {
	mu   sync.RWMutex
	keys map[string]localEntry // userID → key data
}

type localEntry struct {
	privKeyHex string
	address    string
}

// NewKMSLocal creates a new in-memory KMS local provider.
func NewKMSLocal() *KMSLocal {
	return &KMSLocal{
		keys: make(map[string]localEntry),
	}
}

func (k *KMSLocal) Name() string { return ProviderLocal }

// CreateKey generates a secp256k1 key pair for userID. Idempotent: if a key
// already exists for userID, the existing KeyInfo is returned unchanged.
func (k *KMSLocal) CreateKey(_ context.Context, userID string) (kms.KeyInfo, error) {
	if userID == "" {
		return kms.KeyInfo{}, errors.New("kms: userID is required")
	}

	k.mu.RLock()
	entry, exists := k.keys[userID]
	k.mu.RUnlock()

	if exists {
		address := entry.address
		return kms.KeyInfo{
			UserID:  userID,
			Address: address,
			DID:     "did:lac:openprotest:" + strings.ToLower(address),
		}, nil
	}

	privKey, err := gethcrypto.GenerateKey()
	if err != nil {
		return kms.KeyInfo{}, fmt.Errorf("kms: generating key: %w", err)
	}

	privHex := hex.EncodeToString(gethcrypto.FromECDSA(privKey))
	address := gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
	did := "did:lac:openprotest:" + strings.ToLower(address)

	k.mu.Lock()
	// Double-check under write lock to avoid TOCTOU race.
	if existing, ok := k.keys[userID]; ok {
		k.mu.Unlock()
		return kms.KeyInfo{
			UserID:  userID,
			Address: existing.address,
			DID:     "did:lac:openprotest:" + strings.ToLower(existing.address),
		}, nil
	}
	k.keys[userID] = localEntry{privKeyHex: privHex, address: address}
	k.mu.Unlock()

	return kms.KeyInfo{
		UserID:  userID,
		Address: address,
		DID:     did,
	}, nil
}

// Sign signs the given digestHex (exactly 32 bytes after hex decode) with the
// private key stored for userID. Returns ErrKeyNotFound if no key exists.
func (k *KMSLocal) Sign(_ context.Context, userID, digestHex string) (kms.SignResult, error) {
	if userID == "" {
		return kms.SignResult{}, errors.New("kms: userID is required")
	}

	k.mu.RLock()
	entry, ok := k.keys[userID]
	k.mu.RUnlock()
	if !ok {
		return kms.SignResult{}, kms.ErrKeyNotFound
	}

	digestBytes, err := hex.DecodeString(strings.TrimPrefix(digestHex, "0x"))
	if err != nil {
		return kms.SignResult{}, fmt.Errorf("kms: invalid digest hex: %w", err)
	}
	if len(digestBytes) != 32 {
		return kms.SignResult{}, errors.New("kms: digest must be exactly 32 bytes")
	}

	privKeyBytes, err := hex.DecodeString(entry.privKeyHex)
	if err != nil {
		return kms.SignResult{}, fmt.Errorf("kms: corrupt private key: %w", err)
	}
	privKey, err := gethcrypto.ToECDSA(privKeyBytes)
	if err != nil {
		return kms.SignResult{}, fmt.Errorf("kms: parsing private key: %w", err)
	}

	sig, err := gethcrypto.Sign(digestBytes, privKey)
	if err != nil {
		return kms.SignResult{}, fmt.Errorf("kms: signing: %w", err)
	}

	return kms.SignResult{
		Signature: "0x" + hex.EncodeToString(sig),
		Address:   entry.address,
	}, nil
}

// GetAddress returns the EVM address for userID, or ErrKeyNotFound.
func (k *KMSLocal) GetAddress(_ context.Context, userID string) (string, error) {
	k.mu.RLock()
	entry, ok := k.keys[userID]
	k.mu.RUnlock()
	if !ok {
		return "", kms.ErrKeyNotFound
	}
	return entry.address, nil
}

// DeleteKey removes the key for userID from the in-memory store.
func (k *KMSLocal) DeleteKey(_ context.Context, userID string) error {
	k.mu.Lock()
	delete(k.keys, userID)
	k.mu.Unlock()
	return nil
}
