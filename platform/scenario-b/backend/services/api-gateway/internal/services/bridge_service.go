// Package services provides gateway-local bridge services that interact with the shared
// Postgres tables. In production, the payment-orchestrator drives the Relayer lifecycle;
// the gateway provides the REST surface for creating positions and listing state.
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BridgeLockMintService handles Lock on Spoke → Mint on Hub (FR-029 / SC-015).
type BridgeLockMintService struct {
	db *gorm.DB
}

// NewBridgeLockMintService creates a gateway-local BridgeLockMintService.
func NewBridgeLockMintService(db *gorm.DB) *BridgeLockMintService {
	return &BridgeLockMintService{db: db}
}

// LockAndEnqueue creates a BridgedAssetPosition in LOCKING state and enqueues a Relayer item.
// correlationID is optional and used for tracing cross-currency swap flows (009-commercial-cross-currency-swap).
//
// extras is an optional variadic list:
//   - extras[0] = mintToHubAddress: Hub address that receives the minted W-<source> tokens.
//     Set for cross-currency bridge-in so the issuing CB mints to the initiating gateway's
//     swap signer (which executes the AMM swap Step 2), not to the CB itself.
//   - extras[1] = burnFromSpokeAddress: Spoke address of the payer bank. When set, the relayer
//     executor burns tCeBM from this address instead of auto-minting new ones (ensureSpokeFunds).
//     Enforces that the bank must hold tokenized reserves (from Reserve Tokenisation) before the
//     bridge-in can proceed. The CB MUST NOT create new tCeBM in this path.
func (s *BridgeLockMintService) LockAndEnqueue(ctx context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string, extras ...string) (*BridgePositionResult, error) {
	if ownerBankID == "" || spokeNetwork == "" || nativeAsset == "" || amount == "" {
		return nil, fmt.Errorf("owner_bank_id, spoke_network, native_asset, and amount are required")
	}

	positionID := uuid.NewString()
	now := time.Now()

	mintTo := ""
	if len(extras) > 0 {
		mintTo = extras[0]
	}
	burnFromSpoke := ""
	if len(extras) > 1 {
		burnFromSpoke = extras[1]
	}

	logPrefix := ""
	if correlationID != "" {
		logPrefix = fmt.Sprintf("[correlation_id=%s] ", correlationID)
	}
	fmt.Printf("%sbridge lock-mint initiated: position_id=%s owner=%s spoke=%s asset=%s amount=%s mint_to=%s burn_from_spoke=%s\n",
		logPrefix, positionID, ownerBankID, spokeNetwork, nativeAsset, amount, mintTo, burnFromSpoke)

	pos := &domain.BridgedAssetPosition{
		PositionID:           positionID,
		OwnerBankID:          ownerBankID,
		SpokeNetwork:         spokeNetwork,
		NativeAsset:          nativeAsset,
		MirroredAsset:        mirroredAsset,
		MirroredAmount:       amount,
		BridgeState:          domain.BridgeStateLocking,
		MintToHubAddress:     mintTo,
		BurnFromSpokeAddress: burnFromSpoke,
		FirstAttemptAt:       &now,
		LastAttemptAt:        &now,
	}
	if err := s.db.WithContext(ctx).Create(pos).Error; err != nil {
		return nil, fmt.Errorf("persist bridged position failed: %w", err)
	}

	idempotencyKey := fmt.Sprintf("lock:%s", positionID)
	item := &domain.RelayerQueueItem{
		ItemID:         uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		EventType:      "LOCK_MINT",
		PositionID:     positionID,
		State:          domain.RelayerStatePending,
		NextAttemptAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, fmt.Errorf("persist relayer queue item failed: %w", err)
	}

	fmt.Printf("%sbridge lock-mint enqueued: position_id=%s\n", logPrefix, positionID)
	return toPositionResult(pos), nil
}

// BridgeBurnUnlockService handles Burn on Hub → Unlock on Spoke (FR-029 / FR-032 / SC-015).
type BridgeBurnUnlockService struct {
	db *gorm.DB
}

// NewBridgeBurnUnlockService creates a gateway-local BridgeBurnUnlockService.
func NewBridgeBurnUnlockService(db *gorm.DB) *BridgeBurnUnlockService {
	return &BridgeBurnUnlockService{db: db}
}

// BurnAndEnqueue transitions an ACTIVE position to BURNING and enqueues the unlock event.
// correlationID is optional and used for tracing cross-currency swap flows (009-commercial-cross-currency-swap).
func (s *BridgeBurnUnlockService) BurnAndEnqueue(ctx context.Context, positionID, correlationID string) (*BridgePositionResult, error) {
	if positionID == "" {
		return nil, fmt.Errorf("position_id is required")
	}

	logPrefix := ""
	if correlationID != "" {
		logPrefix = fmt.Sprintf("[correlation_id=%s] ", correlationID)
	}
	fmt.Printf("%sbridge burn-unlock initiated: position_id=%s\n", logPrefix, positionID)

	var pos domain.BridgedAssetPosition
	if err := s.db.WithContext(ctx).Where("position_id = ? AND bridge_state = ?", positionID, domain.BridgeStateActive).First(&pos).Error; err != nil {
		return nil, fmt.Errorf("active bridged position not found: %w", err)
	}

	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&pos).Updates(map[string]interface{}{
		"bridge_state":    domain.BridgeStateBurning,
		"last_attempt_at": now,
	}).Error; err != nil {
		return nil, fmt.Errorf("update bridge state failed: %w", err)
	}
	pos.BridgeState = domain.BridgeStateBurning

	idempotencyKey := fmt.Sprintf("burn:%s", positionID)
	item := &domain.RelayerQueueItem{
		ItemID:         uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		EventType:      "BURN_UNLOCK",
		PositionID:     positionID,
		State:          domain.RelayerStatePending,
		NextAttemptAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, fmt.Errorf("persist relayer queue item failed: %w", err)
	}

	fmt.Printf("%sbridge burn-unlock enqueued: position_id=%s\n", logPrefix, positionID)
	return toPositionResult(&pos), nil
}

// EnqueueBurnAfterSwap records an ACTIVE bridge-out position (W-target already on the hub wallet
// after AMM swap) and enqueues BURN_UNLOCK (009 cross-currency step 3).
//
// burnFromHubAddress optionally overrides the default burnFrom address used by the
// BesuRelayerExecutor. Pass the Hub address that actually holds the wrapped tokens
// (i.e. the address that received W-ARS from the AMM swap on CB-A's side). When empty,
// the executor falls back to HUB_MINT_RECIPIENT.
//
// beneficiarySpokeAddress optionally specifies the Spoke-B on-chain address where tCeBM
// should be minted. When provided, the executor calls tCeBM.mint() directly (CENTRAL_BANK_ROLE)
// instead of SpokeBridge.release() (which requires a prior lock on Spoke-B).
func (s *BridgeBurnUnlockService) EnqueueBurnAfterSwap(
	ctx context.Context,
	ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string,
	burnFromHubAddress ...string,
) (*BridgePositionResult, error) {
	if ownerBankID == "" || spokeNetwork == "" || mirroredAsset == "" || amount == "" {
		return nil, fmt.Errorf("owner_bank_id, spoke_network, mirrored_asset, and amount are required")
	}

	burnFrom := ""
	beneficiarySpoke := ""
	if len(burnFromHubAddress) > 0 {
		burnFrom = burnFromHubAddress[0]
	}
	if len(burnFromHubAddress) > 1 {
		beneficiarySpoke = burnFromHubAddress[1]
	}

	logPrefix := ""
	if correlationID != "" {
		logPrefix = fmt.Sprintf("[correlation_id=%s] ", correlationID)
	}

	positionID := uuid.NewString()
	now := time.Now()
	fmt.Printf("%sbridge-out enqueue: position_id=%s owner=%s wrapped=%s amount=%s burn_from=%s beneficiary=%s\n",
		logPrefix, positionID, ownerBankID, mirroredAsset, amount, burnFrom, beneficiarySpoke)

	pos := &domain.BridgedAssetPosition{
		PositionID:              positionID,
		OwnerBankID:             ownerBankID,
		SpokeNetwork:            spokeNetwork,
		NativeAsset:             nativeAsset,
		MirroredAsset:           mirroredAsset,
		MirroredAmount:          amount,
		BridgeState:             domain.BridgeStateActive,
		BurnFromHubAddress:      burnFrom,
		BeneficiarySpokeAddress: beneficiarySpoke,
		FirstAttemptAt:          &now,
		LastAttemptAt:           &now,
	}
	if err := s.db.WithContext(ctx).Create(pos).Error; err != nil {
		return nil, fmt.Errorf("persist bridge-out position failed: %w", err)
	}

	idempotencyKey := fmt.Sprintf("burn:%s", positionID)
	item := &domain.RelayerQueueItem{
		ItemID:         uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		EventType:      "BURN_UNLOCK",
		PositionID:     positionID,
		State:          domain.RelayerStatePending,
		NextAttemptAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, fmt.Errorf("persist relayer queue item failed: %w", err)
	}

	fmt.Printf("%sbridge-out burn-unlock enqueued: position_id=%s\n", logPrefix, positionID)
	return toPositionResult(pos), nil
}

// BridgePositionReader reads bridge positions from the DB (FR-033).
type BridgePositionReader struct {
	db *gorm.DB
}

// NewBridgePositionReader creates a BridgePositionReader.
func NewBridgePositionReader(db *gorm.DB) *BridgePositionReader {
	return &BridgePositionReader{db: db}
}

// ListPositions returns bridge positions, optionally filtered by state.
func (s *BridgePositionReader) ListPositions(ctx context.Context, stateFilter string) ([]BridgePositionResult, error) {
	var positions []domain.BridgedAssetPosition
	q := s.db.WithContext(ctx)
	if stateFilter != "" {
		q = q.Where("bridge_state = ?", stateFilter)
	}
	if err := q.Order("created_at DESC").Find(&positions).Error; err != nil {
		return nil, err
	}
	dtos := make([]BridgePositionResult, len(positions))
	for i, p := range positions {
		dtos[i] = *toPositionResult(&p)
	}
	return dtos, nil
}

// HasActiveBridgePosition returns true if the given ownerBankID has at least one
// BridgedAssetPosition with bridge_state = ACTIVE. Used by the sovereign commit gate
// (T008 / 007-bridge-based-cb-liquidity) to enforce that the CB has completed the
// lock-mint bridge step before registering a liquidity commit on-chain.
// GetBridgeState returns the current bridge_state for a position (FR-033 / 009 polling).
func (s *BridgePositionReader) GetBridgeState(ctx context.Context, positionID string) (domain.BridgeState, error) {
	var pos domain.BridgedAssetPosition
	if err := s.db.WithContext(ctx).Where("position_id = ?", positionID).First(&pos).Error; err != nil {
		return "", fmt.Errorf("position %s: %w", positionID, err)
	}
	return pos.BridgeState, nil
}

func (s *BridgePositionReader) HasActiveBridgePosition(ctx context.Context, ownerBankID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.BridgedAssetPosition{}).
		Where("owner_bank_id = ? AND bridge_state = ?", ownerBankID, domain.BridgeStateActive).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func toPositionResult(p *domain.BridgedAssetPosition) *BridgePositionResult {
	errLog := ""
	if p.RelayerErrorLog != nil {
		errLog = *p.RelayerErrorLog
	}
	return &BridgePositionResult{
		PositionID:      p.PositionID,
		OwnerBankID:     p.OwnerBankID,
		SpokeNetwork:    p.SpokeNetwork,
		NativeAsset:     p.NativeAsset,
		MirroredAsset:   p.MirroredAsset,
		MirroredAmount:  p.MirroredAmount,
		BridgeState:     string(p.BridgeState),
		RelayerRetries:  p.RelayerRetries,
		RelayerErrorLog: errLog,
	}
}
