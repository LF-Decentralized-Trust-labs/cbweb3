// SPDX-License-Identifier: Apache-2.0

// Package services provides the bridge lock-mint service for Scenario B (FR-029 / SC-015).
package services

import (
	"context"
	"fmt"
	"time"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SpokeBridgeLockCaller executes lock-asset calls on a Spoke chain.
type SpokeBridgeLockCaller interface {
	LockAsset(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, amount string) (*podmain.LockResult, error)
}
// RelayerSubmitter enqueues events to the Hyperledger Cacti Relayer.
type RelayerSubmitter interface {
	SubmitLockEvent(ctx context.Context, idempotencyKey, positionID string) error
}

// BridgeLockMintService handles the Lock on Spoke → Mint on Hub lifecycle (FR-029).
type BridgeLockMintService struct {
	db      *gorm.DB
	bridge  SpokeBridgeLockCaller
	relayer RelayerSubmitter
}

// NewBridgeLockMintService creates a BridgeLockMintService.
func NewBridgeLockMintService(db *gorm.DB, bridge SpokeBridgeLockCaller, relayer RelayerSubmitter) *BridgeLockMintService {
	return &BridgeLockMintService{db: db, bridge: bridge, relayer: relayer}
}

// LockAndEnqueue locks native assets on the Spoke and enqueues a RelayerQueueItem (FR-029 / FR-031).
func (s *BridgeLockMintService) LockAndEnqueue(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount string) (*podmain.BridgedAssetPosition, error) {
	if ownerBankID == "" || spokeNetwork == "" || nativeAsset == "" || amount == "" {
		return nil, fmt.Errorf("owner_bank_id, spoke_network, native_asset, and amount are required")
	}

	lockResult, err := s.bridge.LockAsset(ctx, ownerBankID, spokeNetwork, nativeAsset, amount)
	if err != nil {
		return nil, fmt.Errorf("spoke bridge lock failed: %w", err)
	}

	positionID := uuid.NewString()
	now := time.Now()
	// NOTE: this writes bridged_asset_positions, which the api-gateway's Hub reconciliation reads
	// and which now carries a `direction` column (IN = mints on the Hub, OUT = burns). This
	// service is currently wired nowhere — the gateway's own BridgeLockMintService is the live
	// producer — and the orchestrator's model does not declare the column, so a row created here
	// would be invisible to the reconciliation until the gateway's startup backfill defaults it to
	// IN. If this service is ever wired up, add the column to podmain.BridgedAssetPosition and set
	// it here rather than relying on that backfill.
	pos := &podmain.BridgedAssetPosition{
		PositionID:     positionID,
		OwnerBankID:    ownerBankID,
		SpokeNetwork:   spokeNetwork,
		NativeAsset:    nativeAsset,
		MirroredAsset:  mirroredAsset,
		MirroredAmount: amount,
		BridgeState:    podmain.BridgeStateLocking,
		FirstAttemptAt: &now,
		LastAttemptAt:  &now,
	}
	if err := s.db.WithContext(ctx).Create(pos).Error; err != nil {
		return nil, fmt.Errorf("persist bridged position failed: %w", err)
	}

	idempotencyKey := fmt.Sprintf("lock:%s:%s", positionID, lockResult.TxHash)
	item := &podmain.RelayerQueueItem{
		ItemID:          uuid.NewString(),
		IdempotencyKey:  idempotencyKey,
		EventType:       podmain.RelayerEventTypeLockMint,
		PositionID:      positionID,
		State:           podmain.RelayerItemStatePending,
		NextAttemptAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, fmt.Errorf("persist relayer queue item failed: %w", err)
	}

	if submitErr := s.relayer.SubmitLockEvent(ctx, idempotencyKey, positionID); submitErr != nil {
		// Non-fatal: relayer worker will retry from DB
		_ = submitErr
	}

	return pos, nil
}
