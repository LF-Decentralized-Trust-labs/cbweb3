// This file declares the wallet identity binding contract for persistence layers.
package interfaces

import "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"

// IIdentityManager defines wallet binding operations for user identities.
type IIdentityManager interface {
	BindWallet(userID, walletAddress string) (domain.WalletBinding, error)
	GetByUser(userID string) (domain.WalletBinding, bool)
}

