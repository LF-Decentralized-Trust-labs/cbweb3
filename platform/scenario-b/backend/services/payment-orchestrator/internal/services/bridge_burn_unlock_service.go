// SPDX-License-Identifier: Apache-2.0

// Package services provides the bridge burn-unlock service for Scenario B (FR-029 / FR-032 / SC-015).
package services

import (
	"context"
	"fmt"
	"time"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SpokeBridgeUnlockCaller executes unlock-asset calls on a Spoke chain.
type SpokeBridgeUnlockCaller interface {
	UnlockAsset(ctx context.Context, positionID, spokeNetwork, mirroredAsset, amount string) (*podmain.UnlockResult, error)
}

// BurnRelayerSubmitter enqueues burn events to the Relayer.
type BurnRelayerSubmitter interface {
	SubmitBurnEvent(ctx context.Context, idempotencyKey, positionID string) error
}

// BridgeBurnUnlockService handles the Burn on Hub → Unlock on Spoke lifecycle (FR-029 / FR-032).
type BridgeBurnUnlockService struct {
	db      *gorm.DB
	bridge  SpokeBridgeUnlockCaller
	relayer BurnRelayerSubmitter
}

// NewBridgeBurnUnlockService creates a BridgeBurnUnlockService.
func NewBridgeBurnUnlockService(db *gorm.DB, bridge SpokeBridgeUnlockCaller, relayer BurnRelayerSubmitter) *BridgeBurnUnlockService {
	return &BridgeBurnUnlockService{db: db, bridge: bridge, relayer: relayer}
}

// BurnAndEnqueue transitions an ACTIVE position to BURNING and enqueues the unlock event (FR-032).
func (s *BridgeBurnUnlockService) BurnAndEnqueue(ctx context.Context, positionID string) (*podmain.BridgedAssetPosition, error) {
	if positionID == "" {
		return nil, fmt.Errorf("position_id is required")
	}

	var pos podmain.BridgedAssetPosition
	if err := s.db.WithContext(ctx).Where("position_id = ? AND bridge_state = ?", positionID, podmain.BridgeStateActive).First(&pos).Error; err != nil {
		return nil, fmt.Errorf("active bridged position not found: %w", err)
	}

	unlockResult, err := s.bridge.UnlockAsset(ctx, positionID, pos.SpokeNetwork, pos.MirroredAsset, pos.MirroredAmount)
	if err != nil {
		return nil, fmt.Errorf("spoke bridge unlock failed: %w", err)
	}

	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&pos).Updates(map[string]interface{}{
		"bridge_state":    podmain.BridgeStateBurning,
		"last_attempt_at": now,
	}).Error; err != nil {
		return nil, fmt.Errorf("update bridge state failed: %w", err)
	}
	pos.BridgeState = podmain.BridgeStateBurning

	idempotencyKey := fmt.Sprintf("burn:%s:%s", positionID, unlockResult.TxHash)
	item := &podmain.RelayerQueueItem{
		ItemID:         uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		EventType:      podmain.RelayerEventTypeBurnUnlock,
		PositionID:     positionID,
		State:          podmain.RelayerItemStatePending,
		NextAttemptAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, fmt.Errorf("persist relayer queue item failed: %w", err)
	}

	if submitErr := s.relayer.SubmitBurnEvent(ctx, idempotencyKey, positionID); submitErr != nil {
		// Non-fatal: relayer worker will retry from DB
		_ = submitErr
	}

	return &pos, nil
}
