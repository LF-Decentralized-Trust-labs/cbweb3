// This file declares the wallet identity binding contract for persistence layers.
package interfaces

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// IIdentityManager defines wallet binding operations for user identities.
type IIdentityManager interface {
	BindWallet(ctx context.Context, userID, walletAddress string) (domain.WalletBinding, error)
	GetByUser(ctx context.Context, userID string) (domain.WalletBinding, bool)
}

