// Package domain defines the BridgedAssetPosition and RelayerQueueItem models for the
// gateway-local bridge service (FR-029 / FR-031 / FR-032 / data-model §6).
//
// These mirror the canonical models in payment-orchestrator and share the same Postgres
// tables so both services can read/write bridge state.
package domain

import "time"

// BridgeState is the lifecycle state of a bridged asset position.
type BridgeState string

const (
	BridgeStateLocking                BridgeState = "LOCKING"
	BridgeStateActive                 BridgeState = "ACTIVE"
	BridgeStateBurning                BridgeState = "BURNING"
	BridgeStateReleased               BridgeState = "RELEASED"
	BridgeStateReconciliationRequired BridgeState = "RECONCILIATION_REQUIRED"
)

// BridgedAssetPosition tracks a cross-spoke bridging lifecycle (Lock→Active→Burn→Released).
type BridgedAssetPosition struct {
	PositionID      string      `gorm:"primaryKey;column:position_id;type:varchar(64)"`
	OwnerBankID     string      `gorm:"column:owner_bank_id;not null"`
	SpokeNetwork    string      `gorm:"column:spoke_network;not null"`
	NativeAsset     string      `gorm:"column:native_asset;not null"`
	MirroredAsset   string      `gorm:"column:mirrored_asset;not null"`
	MirroredAmount  string      `gorm:"column:mirrored_amount;not null"`
	BridgeState     BridgeState `gorm:"column:bridge_state;not null;default:'LOCKING'"`
	RelayerRetries  int         `gorm:"column:relayer_retries;not null;default:0"`
	LastAttemptAt   *time.Time  `gorm:"column:last_attempt_at"`
	FirstAttemptAt  *time.Time  `gorm:"column:first_attempt_at"`
	RelayerErrorLog *string     `gorm:"column:relayer_error_log;type:jsonb"`
	// BurnFromHubAddress overrides the default burnFrom in the Relayer executor for
	// cross-currency bridge-out (009). When empty, the executor falls back to
	// HUB_MINT_RECIPIENT env var. Populated by CrossCurrencyBridgeOutHandler with
	// the Hub address that actually holds the wrapped tokens after the AMM swap.
	BurnFromHubAddress string `gorm:"column:burn_from_hub_address;default:''"`
	// MintToHubAddress overrides the default mint recipient in the Relayer executor for
	// cross-currency bridge-in (009). When set, the issuing CB mints W-<source> to this Hub
	// address — the initiating gateway's swap signer, which executes the AMM swap (Step 2)
	// and from which CB-B later burns (bridge-out SwapSenderAddress). When empty, the
	// executor falls back to HUB_MINT_RECIPIENT. Populated by CrossCurrencyBridgeInHandler.
	MintToHubAddress string `gorm:"column:mint_to_hub_address;default:''"`
	// BeneficiarySpokeAddress is the Spoke-B on-chain address (EVM) of the beneficiary bank.
	// For cross-currency bridge-out, CB-B mints tCeBM directly to this address
	// (CENTRAL_BANK_ROLE) instead of calling SpokeBridge.release() which requires a prior lock.
	BeneficiarySpokeAddress string `gorm:"column:beneficiary_spoke_address;default:''"`
	// BurnFromSpokeAddress is the Spoke-A on-chain address of the payer bank.
	// When set on a LOCK_MINT position, the executor burns tCeBM from this address instead of
	// auto-minting (ensureSpokeFunds). Enforces that bank-a must hold tokenized reserves
	// obtained via Reserve Tokenisation before a cross-currency bridge-in can proceed.
	BurnFromSpokeAddress string `gorm:"column:burn_from_spoke_address;default:''"`
	// SwapTxHash is the Hub AMM swap transaction this bridge-out settles (R2-CR-6).
	// Unique (when set): each on-chain swap can be consumed by exactly one burn/mint,
	// so replayed relay notifications cannot mint twice. Partial index because legacy
	// flows (plain lock-mint, dev paths) have no associated swap.
	SwapTxHash string `gorm:"column:swap_tx_hash;default:'';index:idx_bridge_swap_tx_hash,unique,where:swap_tx_hash <> ''"`
	// CorrelationID links the position to the cross-currency swap operation (009) for
	// tracing. Not unique: a rollback position legitimately shares the correlation of
	// the bridge-in it reverses — replay protection is keyed on SwapTxHash.
	CorrelationID string    `gorm:"column:correlation_id;default:'';index:idx_bridge_correlation_id"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// RelayerItemState is the state of a Relayer queue item.
type RelayerItemState string

const (
	RelayerStatePending   RelayerItemState = "PENDING"
	RelayerStateInFlight  RelayerItemState = "IN_FLIGHT"
	RelayerStateCompleted RelayerItemState = "COMPLETED"
	RelayerStateFailed    RelayerItemState = "FAILED"
	RelayerStateEscalated RelayerItemState = "ESCALATED"
)

// RelayerQueueItem is a durable queue entry for the Cacti Relayer (FR-031 / FR-039).
type RelayerQueueItem struct {
	ItemID         string           `gorm:"primaryKey;column:item_id;type:varchar(64)"`
	IdempotencyKey string           `gorm:"column:idempotency_key;not null;uniqueIndex"`
	EventType      string           `gorm:"column:event_type;not null"`
	PositionID     string           `gorm:"column:position_id;not null"`
	State          RelayerItemState `gorm:"column:state;not null;default:'PENDING'"`
	AttemptCount   int              `gorm:"column:attempt_count;not null;default:0"`
	NextAttemptAt  time.Time        `gorm:"column:next_attempt_at"`
	LastError      string           `gorm:"column:last_error"`
	CreatedAt      time.Time        `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time        `gorm:"column:updated_at;autoUpdateTime"`
}
