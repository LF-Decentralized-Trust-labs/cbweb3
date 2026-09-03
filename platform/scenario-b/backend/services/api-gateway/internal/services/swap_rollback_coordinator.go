// SPDX-License-Identifier: Apache-2.0

// Package services provides SwapRollbackCoordinator for handling automatic rollback
// when swap fails after successful bridge-in (FR-010).
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/research.md (Q2: Automatic Rollback)
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// BridgeReversoServiceIface handles bridge reverso (Hub→Spoke-A) to return funds to payer.
type BridgeReversoServiceIface interface {
	BurnAndEnqueue(ctx context.Context, positionID, correlationID string) (*BridgePositionResult, error)
}

// SwapRollbackRepository persists rollback audit logs.
type SwapRollbackRepository interface {
	Create(ctx context.Context, log *domain.SwapRollbackLog) error
	UpdateStatus(ctx context.Context, rollbackID string, status domain.RollbackStatus) error
	IncrementRetryCount(ctx context.Context, rollbackID string) error
	UpdateTxHashes(ctx context.Context, rollbackID, burnTxHash, unlockTxHash string) error
}

// SwapRollbackCoordinator executes automatic rollback with up to 3 retries (5s, 15s, 45s backoff).
type SwapRollbackCoordinator struct {
	bridgeService BridgeReversoServiceIface
	rollbackRepo  SwapRollbackRepository
}

// NewSwapRollbackCoordinator creates a SwapRollbackCoordinator.
func NewSwapRollbackCoordinator(bridgeService BridgeReversoServiceIface, rollbackRepo SwapRollbackRepository) *SwapRollbackCoordinator {
	return &SwapRollbackCoordinator{
		bridgeService: bridgeService,
		rollbackRepo:  rollbackRepo,
	}
}

// ReverseBridge attempts to reverse bridge (Hub→Spoke-A) with exponential backoff retries.
// Returns error if all 3 retries fail (requires manual intervention).
func (c *SwapRollbackCoordinator) ReverseBridge(ctx context.Context, swapOperationID, bridgeInPositionID string) error {
	// Create rollback log
	rollbackLog := &domain.SwapRollbackLog{
		RollbackID:         generateRollbackID(), // TODO: Use proper UUID generator
		SwapOperationID:    swapOperationID,
		BridgeInPositionID: bridgeInPositionID,
		RollbackStatus:     domain.RollbackStatusPending,
		RetryCount:         0,
	}

	if err := c.rollbackRepo.Create(ctx, rollbackLog); err != nil {
		return fmt.Errorf("failed to create rollback log: %w", err)
	}

	// Retry logic: 3 attempts with backoff 5s, 15s, 45s
	backoffs := []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		// Update status to IN_PROGRESS before retry
		if err := c.rollbackRepo.UpdateStatus(ctx, rollbackLog.RollbackID, domain.RollbackStatusInProgress); err != nil {
			log.Printf("warning: failed to update rollback status to IN_PROGRESS: %v", err)
		}

		// Attempt bridge reverso
		_, err := c.bridgeService.BurnAndEnqueue(ctx, bridgeInPositionID, "")
		if err == nil {
			// Success — update rollback log (tx_hashes tracked by relayer)
			if err := c.rollbackRepo.UpdateStatus(ctx, rollbackLog.RollbackID, domain.RollbackStatusCompleted); err != nil {
				log.Printf("warning: failed to update rollback status to COMPLETED: %v", err)
			}
			log.Printf("rollback successful for swap_operation_id=%s (bridge_position=%s, attempt=%d)",
				swapOperationID, bridgeInPositionID, attempt+1)
			return nil
		}

		lastErr = err
		log.Printf("rollback attempt %d/%d failed for swap_operation_id=%s: %v",
			attempt+1, 3, swapOperationID, err)

		// Increment retry count
		if err := c.rollbackRepo.IncrementRetryCount(ctx, rollbackLog.RollbackID); err != nil {
			log.Printf("warning: failed to increment retry count: %v", err)
		}

		// Wait before next retry (except after last attempt)
		if attempt < 2 {
			log.Printf("waiting %v before retry %d...", backoffs[attempt], attempt+2)
			time.Sleep(backoffs[attempt])
		}
	}

	// All retries exhausted — mark as FAILED
	if err := c.rollbackRepo.UpdateStatus(ctx, rollbackLog.RollbackID, domain.RollbackStatusFailed); err != nil {
		log.Printf("warning: failed to update rollback status to FAILED: %v", err)
	}

	return fmt.Errorf("rollback failed after 3 attempts for swap_operation_id=%s: %w", swapOperationID, lastErr)
}

// generateRollbackID is a placeholder for UUID generation (should use shared utility).
func generateRollbackID() string {
	// TODO: Replace with proper UUID generator from shared package
	return "rollback-" + time.Now().Format("20060102150405.000000000")
}
