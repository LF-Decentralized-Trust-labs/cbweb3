// SPDX-License-Identifier: Apache-2.0

// Package services provides gateway-local bridge services that interact with the shared
// Postgres tables. In production, the payment-orchestrator drives the Relayer lifecycle;
// the gateway provides the REST surface for creating positions and listing state.
package services

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sanitizeLogField strips CR/LF so relay-influenced fields (correlation_id, swap_tx_hash)
// cannot inject forged log lines (R2-CR-6 review, Low).
func sanitizeLogField(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// pgUniqueViolation is the Postgres SQLSTATE for a unique-constraint violation.
const pgUniqueViolation = "23505"

// isUniqueViolation reports whether err is a unique-constraint violation. Only such an error
// should trigger the swap_tx_hash idempotency fallback; a transient connection error or
// deadlock must surface so the caller can retry rather than masquerade as a duplicate swap.
//
// Driver-agnostic on purpose. Recognising only Postgres SQLSTATE 23505 would make the
// concurrent-replay fallback silently inert on any other driver — a replay would surface as a
// hard error instead of returning the winning position idempotently.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	// SQLite (repository tests) reports "UNIQUE constraint failed: <table>.<column>" and has
	// no error code to match on.
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

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
		logPrefix = fmt.Sprintf("[correlation_id=%s] ", sanitizeLogField(correlationID))
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
		logPrefix = fmt.Sprintf("[correlation_id=%s] ", sanitizeLogField(correlationID))
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
//
// extras[2] (swapTxHash) binds the position to the verified Hub swap transaction; a partial
// unique index on (swap_tx_hash, leg) makes each swap consumable at most once per leg
// (R2-CR-6). On a duplicate, the existing position is returned without enqueuing a second burn.
func (s *BridgeBurnUnlockService) EnqueueBurnAfterSwap(
	ctx context.Context,
	ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string,
	burnFromHubAddress ...string,
) (*BridgePositionResult, error) {
	burnFrom := ""
	beneficiarySpoke := ""
	swapTxHash := ""
	if len(burnFromHubAddress) > 0 {
		burnFrom = burnFromHubAddress[0]
	}
	if len(burnFromHubAddress) > 1 {
		beneficiarySpoke = burnFromHubAddress[1]
	}
	if len(burnFromHubAddress) > 2 {
		swapTxHash = burnFromHubAddress[2]
	}
	return s.enqueueBurn(ctx, burnParams{
		ownerBankID:      ownerBankID,
		spokeNetwork:     spokeNetwork,
		nativeAsset:      nativeAsset,
		mirroredAsset:    mirroredAsset,
		amount:           amount,
		correlationID:    correlationID,
		burnFrom:         burnFrom,
		beneficiarySpoke: beneficiarySpoke,
		swapTxHash:       swapTxHash,
		leg:              domain.BridgeLegSettlement,
	})
}

// EnqueueResidueReturn records the RESIDUE leg of a Hub swap: the slippage buffer that
// bridge-in had to move before the real cost was known and that the swap did not consume.
//
// Mechanically it is the same operation as a bridge-out — burn the wrapped token on the Hub,
// deliver the native token on a spoke — pointed back at the *source* spoke with the payer as
// the beneficiary. It is a distinct leg so it neither collides with the settlement position
// on the (swap_tx_hash, leg) index nor is mistaken for a replay of it.
//
// parentPositionID is the bridge-in position this return corrects; the pair of records is
// what expresses the net amount consumed, since the parent's mirrored_amount is on-chain
// truth and must not be rewritten.
func (s *BridgeBurnUnlockService) EnqueueResidueReturn(
	ctx context.Context,
	ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string,
	burnFromHubAddress, beneficiarySpokeAddress, swapTxHash, parentPositionID string,
) (*BridgePositionResult, error) {
	return s.enqueueBurn(ctx, burnParams{
		ownerBankID:      ownerBankID,
		spokeNetwork:     spokeNetwork,
		nativeAsset:      nativeAsset,
		mirroredAsset:    mirroredAsset,
		amount:           amount,
		correlationID:    correlationID,
		burnFrom:         burnFromHubAddress,
		beneficiarySpoke: beneficiarySpokeAddress,
		swapTxHash:       swapTxHash,
		leg:              domain.BridgeLegResidue,
		parentPositionID: parentPositionID,
	})
}

// burnParams carries the fields of a burn-side position. Used internally so the settlement
// and residue legs share one persistence path instead of drifting apart.
type burnParams struct {
	ownerBankID      string
	spokeNetwork     string
	nativeAsset      string
	mirroredAsset    string
	amount           string
	correlationID    string
	burnFrom         string
	beneficiarySpoke string
	swapTxHash       string
	leg              domain.BridgeLeg
	parentPositionID string
}

// ensureBurnQueueItem makes sure a BURN_UNLOCK queue item exists for the position, so that
// something will actually drive it. Idempotent on the item's own unique idempotency_key: an
// item that is already there — pending, in flight or already completed — is left untouched.
func (s *BridgeBurnUnlockService) ensureBurnQueueItem(ctx context.Context, positionID string) error {
	item := &domain.RelayerQueueItem{
		ItemID:         uuid.NewString(),
		IdempotencyKey: fmt.Sprintf("burn:%s", positionID),
		EventType:      "BURN_UNLOCK",
		PositionID:     positionID,
		State:          domain.RelayerStatePending,
		NextAttemptAt:  time.Now(),
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error
}

func (s *BridgeBurnUnlockService) enqueueBurn(ctx context.Context, p burnParams) (*BridgePositionResult, error) {
	if p.ownerBankID == "" || p.spokeNetwork == "" || p.mirroredAsset == "" || p.amount == "" {
		return nil, fmt.Errorf("owner_bank_id, spoke_network, mirrored_asset, and amount are required")
	}
	if p.leg == "" {
		p.leg = domain.BridgeLegSettlement
	}

	logPrefix := ""
	if p.correlationID != "" {
		logPrefix = fmt.Sprintf("[correlation_id=%s] ", sanitizeLogField(p.correlationID))
	}

	positionID := uuid.NewString()
	now := time.Now()
	fmt.Printf("%sbridge-out enqueue: position_id=%s leg=%s owner=%s wrapped=%s amount=%s burn_from=%s beneficiary=%s parent=%s\n",
		logPrefix, positionID, p.leg, p.ownerBankID, p.mirroredAsset, p.amount, p.burnFrom, p.beneficiarySpoke, p.parentPositionID)

	pos := &domain.BridgedAssetPosition{
		PositionID:              positionID,
		OwnerBankID:             p.ownerBankID,
		SpokeNetwork:            p.spokeNetwork,
		NativeAsset:             p.nativeAsset,
		MirroredAsset:           p.mirroredAsset,
		MirroredAmount:          p.amount,
		BridgeState:             domain.BridgeStateActive,
		BurnFromHubAddress:      p.burnFrom,
		BeneficiarySpokeAddress: p.beneficiarySpoke,
		SwapTxHash:              p.swapTxHash,
		Leg:                     p.leg,
		ParentPositionID:        p.parentPositionID,
		CorrelationID:           p.correlationID,
		FirstAttemptAt:          &now,
		LastAttemptAt:           &now,
	}
	if err := s.db.WithContext(ctx).Create(pos).Error; err != nil {
		// Unique-index race: a concurrent replay consumed this swap leg first. Return the
		// winning position so the caller's response stays idempotent. Only a unique
		// violation (23505) means "duplicate swap"; any other error (connection drop,
		// deadlock, etc.) must propagate so the caller can retry safely.
		if p.swapTxHash != "" && isUniqueViolation(err) {
			if existing, findErr := s.findBySwapTxHashLeg(ctx, p.swapTxHash, p.leg); findErr == nil && existing != nil {
				// The position existing does not mean anything is driving it. This function
				// persists the position and its queue item in two statements, so a failure on
				// the second leaves a position with no queue item and returns an error — the
				// caller records the attempt as failed. A later retry lands HERE, on the unique
				// violation, and would report success for a position nothing will ever process.
				// Ensure the queue item first, idempotently on its own unique key: a genuine
				// replay is a no-op, the interrupted case is repaired. Re-driving an
				// already-settled burn is safe — the executor reconciles by persisted tx hash
				// rather than re-burning (R2-H-12).
				if qerr := s.ensureBurnQueueItem(ctx, existing.PositionID); qerr != nil {
					return nil, fmt.Errorf("duplicate position %s has no relayer queue item and it could not be created: %w",
						existing.PositionID, qerr)
				}
				fmt.Printf("%sbridge-out duplicate swap_tx_hash=%s leg=%s — returning existing position %s\n",
					logPrefix, sanitizeLogField(p.swapTxHash), p.leg, existing.PositionID)
				return existing, nil
			}
		}
		return nil, fmt.Errorf("persist bridge-out position failed: %w", err)
	}

	if err := s.ensureBurnQueueItem(ctx, positionID); err != nil {
		return nil, fmt.Errorf("persist relayer queue item failed: %w", err)
	}

	fmt.Printf("%sbridge-out burn-unlock enqueued: position_id=%s leg=%s\n", logPrefix, positionID, p.leg)
	return toPositionResult(pos), nil
}

// FindBySwapTxHash returns the SETTLEMENT position that already consumed the given Hub
// swap transaction, or (nil, nil) when the swap has not been processed (R2-CR-6 replay
// protection). Used by CrossCurrencyBridgeOutHandler before enqueuing a burn.
//
// Scoped to the settlement leg on purpose: a residue return carries the same swap_tx_hash,
// and treating it as a prior settlement would make a legitimate payment look like a replay.
func (s *BridgeBurnUnlockService) FindBySwapTxHash(ctx context.Context, swapTxHash string) (*BridgePositionResult, error) {
	return s.findBySwapTxHashLeg(ctx, swapTxHash, domain.BridgeLegSettlement)
}

// FindResidueBySwapTxHash is the residue-leg counterpart, used by the residue-return
// handler for its own replay protection.
func (s *BridgeBurnUnlockService) FindResidueBySwapTxHash(ctx context.Context, swapTxHash string) (*BridgePositionResult, error) {
	return s.findBySwapTxHashLeg(ctx, swapTxHash, domain.BridgeLegResidue)
}

func (s *BridgeBurnUnlockService) findBySwapTxHashLeg(ctx context.Context, swapTxHash string, leg domain.BridgeLeg) (*BridgePositionResult, error) {
	if swapTxHash == "" {
		return nil, nil
	}
	var pos domain.BridgedAssetPosition
	// Legacy rows predate the leg column; AutoMigrate backfills them to SETTLEMENT, but
	// tolerate an empty value so a partially migrated table cannot silently lose replay
	// protection on the settlement leg.
	q := s.db.WithContext(ctx).Where("swap_tx_hash = ?", swapTxHash)
	if leg == domain.BridgeLegSettlement {
		q = q.Where("leg = ? OR leg = ''", leg)
	} else {
		q = q.Where("leg = ?", leg)
	}
	if err := q.First(&pos).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup by swap_tx_hash failed: %w", err)
	}
	return toPositionResult(&pos), nil
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

// GetPosition returns the internal detail of a bridge position. Used by the residue-return
// handler to authorize a return against the bridge-in position the CB itself created: the
// amount bridged in is read from here, never from the request body.
//
// Distinct from BridgePositionResult (the public JSON DTO) so Hub/spoke addresses stay off
// the list endpoints.
func (s *BridgePositionReader) GetPosition(ctx context.Context, positionID string) (*BridgePositionDetail, error) {
	if positionID == "" {
		return nil, fmt.Errorf("position_id is required")
	}
	var pos domain.BridgedAssetPosition
	if err := s.db.WithContext(ctx).Where("position_id = ?", positionID).First(&pos).Error; err != nil {
		return nil, fmt.Errorf("position %s: %w", positionID, err)
	}
	return &BridgePositionDetail{
		PositionID:           pos.PositionID,
		OwnerBankID:          pos.OwnerBankID,
		SpokeNetwork:         pos.SpokeNetwork,
		NativeAsset:          pos.NativeAsset,
		MirroredAsset:        pos.MirroredAsset,
		MirroredAmount:       pos.MirroredAmount,
		BridgeState:          string(pos.BridgeState),
		MintToHubAddress:     pos.MintToHubAddress,
		BurnFromSpokeAddress: pos.BurnFromSpokeAddress,
		Leg:                  string(pos.Leg),
	}, nil
}

// NetConsumed reports how much of a bridge-in position the swap actually consumed:
// mirrored_amount minus whatever its residue leg gives back.
//
// The bridge-in position's mirrored_amount is deliberately never rewritten — it records what
// the chain did, and is audit evidence. The truth about consumption is the *pair* of records,
// which is what this derives.
//
// A residue leg that has not reached RELEASED is still counted: the return is in flight, and
// treating it as consumed would overstate the payer's debit.
func (s *BridgePositionReader) NetConsumed(ctx context.Context, bridgeInPositionID string) (string, error) {
	var parent domain.BridgedAssetPosition
	if err := s.db.WithContext(ctx).Where("position_id = ?", bridgeInPositionID).First(&parent).Error; err != nil {
		return "", fmt.Errorf("position %s: %w", bridgeInPositionID, err)
	}
	bridged, ok := new(big.Int).SetString(strings.TrimSpace(parent.MirroredAmount), 10)
	if !ok {
		return "", fmt.Errorf("position %s has an unparseable mirrored_amount %q", bridgeInPositionID, parent.MirroredAmount)
	}

	var legs []domain.BridgedAssetPosition
	if err := s.db.WithContext(ctx).
		Where("parent_position_id = ? AND leg = ?", bridgeInPositionID, domain.BridgeLegResidue).
		Find(&legs).Error; err != nil {
		return "", fmt.Errorf("list residue legs of %s: %w", bridgeInPositionID, err)
	}

	net := new(big.Int).Set(bridged)
	for i := range legs {
		returned, ok := new(big.Int).SetString(strings.TrimSpace(legs[i].MirroredAmount), 10)
		if !ok {
			return "", fmt.Errorf("residue leg %s has an unparseable mirrored_amount %q", legs[i].PositionID, legs[i].MirroredAmount)
		}
		net.Sub(net, returned)
	}
	if net.Sign() < 0 {
		return "", fmt.Errorf("position %s: residue legs exceed the bridged amount — reconciliation required", bridgeInPositionID)
	}
	return net.String(), nil
}

// ListStrandedResidueLegs returns residue positions that have not reached RELEASED: value
// burned or pending on the Hub that the payer has not received back yet. A growing result
// set means slippage buffers are accumulating instead of being returned.
func (s *BridgePositionReader) ListStrandedResidueLegs(ctx context.Context) ([]BridgePositionResult, error) {
	var positions []domain.BridgedAssetPosition
	if err := s.db.WithContext(ctx).
		Where("leg = ? AND bridge_state <> ?", domain.BridgeLegResidue, domain.BridgeStateReleased).
		Order("created_at ASC").
		Find(&positions).Error; err != nil {
		return nil, fmt.Errorf("list stranded residue legs: %w", err)
	}
	dtos := make([]BridgePositionResult, len(positions))
	for i := range positions {
		dtos[i] = *toPositionResult(&positions[i])
	}
	return dtos, nil
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
