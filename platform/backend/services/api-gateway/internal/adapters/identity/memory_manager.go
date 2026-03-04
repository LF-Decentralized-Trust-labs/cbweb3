// This file provides an in-memory wallet binding manager for identity associations.
package identity

import (
	"strings"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/ethereum/go-ethereum/common"
)

// MemoryIdentityManager stores user-wallet bindings in memory.
type MemoryIdentityManager struct {
	mu           sync.RWMutex
	byUser       map[string]string
	walletToUser map[string]string
}

// NewMemoryIdentityManager creates an empty in-memory identity store.
func NewMemoryIdentityManager() *MemoryIdentityManager {
	return &MemoryIdentityManager{
		byUser:       make(map[string]string),
		walletToUser: make(map[string]string),
	}
}

// BindWallet creates or validates a one-to-one binding between user and wallet.
func (m *MemoryIdentityManager) BindWallet(userID, walletAddress string) (domain.WalletBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wallet := common.HexToAddress(walletAddress).Hex()
	if existingUser, exists := m.walletToUser[wallet]; exists && existingUser != userID {
		return domain.WalletBinding{}, domain.ErrWalletAlreadyBound
	}
	if existingWallet, exists := m.byUser[userID]; exists && !strings.EqualFold(existingWallet, wallet) {
		return domain.WalletBinding{}, domain.ErrUserAlreadyBound
	}

	m.byUser[userID] = wallet
	m.walletToUser[wallet] = userID
	return domain.WalletBinding{
		UserID:        userID,
		WalletAddress: wallet,
	}, nil
}

// GetByUser returns the binding for a user when present.
func (m *MemoryIdentityManager) GetByUser(userID string) (domain.WalletBinding, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wallet, ok := m.byUser[userID]
	if !ok {
		return domain.WalletBinding{}, false
	}
	return domain.WalletBinding{
		UserID:        userID,
		WalletAddress: wallet,
	}, true
}

