package providers

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/kmsprovider"
	"github.com/ethereum/go-ethereum/crypto"
)

type localKMS struct {
	mu         sync.RWMutex
	keysByID   map[string]*ecdsa.PrivateKey
	keyInfoByU map[string]kmsprovider.KeyInfo
}

func NewLocalKMS() kmsprovider.Provider {
	return &localKMS{
		keysByID:   map[string]*ecdsa.PrivateKey{},
		keyInfoByU: map[string]kmsprovider.KeyInfo{},
	}
}

func (k *localKMS) Name() string { return ProviderLocalKMS }

func (k *localKMS) CreateKey(_ context.Context, userID string) (kmsprovider.KeyInfo, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return kmsprovider.KeyInfo{}, errors.New("userID is required")
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	if existing, ok := k.keyInfoByU[userID]; ok {
		return existing, nil
	}

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return kmsprovider.KeyInfo{}, err
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	keyID := "localkms:" + userID
	k.keysByID[keyID] = privateKey
	info := kmsprovider.KeyInfo{
		KeyID:   keyID,
		Address: address,
	}
	k.keyInfoByU[userID] = info
	return info, nil
}

func (k *localKMS) GetByUser(_ context.Context, userID string) (kmsprovider.KeyInfo, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	info, ok := k.keyInfoByU[userID]
	return info, ok
}

func (k *localKMS) SignDigest(_ context.Context, keyID, digestHex string) (string, error) {
	keyID = strings.TrimSpace(keyID)
	digestHex = strings.TrimSpace(strings.TrimPrefix(digestHex, "0x"))
	if keyID == "" || digestHex == "" {
		return "", errors.New("keyID and digest are required")
	}

	digest, err := hex.DecodeString(digestHex)
	if err != nil {
		return "", err
	}
	if len(digest) != 32 {
		return "", errors.New("digest must be 32 bytes")
	}

	k.mu.RLock()
	privateKey, ok := k.keysByID[keyID]
	k.mu.RUnlock()
	if !ok {
		return "", errors.New("key not found")
	}

	signature, err := crypto.Sign(digest, privateKey)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(signature), nil
}
