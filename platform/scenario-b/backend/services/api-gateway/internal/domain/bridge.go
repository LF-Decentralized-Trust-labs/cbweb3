// SPDX-License-Identifier: Apache-2.0

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
	// BridgeStateBurned is written by the relayer once a Hub burn is CONFIRMED on-chain, before
	// the spoke-side delivery. The gateway needs it to tell a confirmed burn from a merely
	// broadcast one: the tx hash alone is recorded pre-confirmation as an intent, so reading the
	// hash would count value as gone while it is still sitting on the Hub address.
	BridgeStateBurned BridgeState = "BURNED"
)

// BridgeDirection says which way a position moves value, which no other field on the row does.
//
// Without it a bridge-out is indistinguishable from a bridge-in: both are created with
// leg=SETTLEMENT, both reach ACTIVE, and both carry the local CB's own W-token as
// mirrored_asset. Anything reasoning about what a CB HOLDS therefore has to know the direction,
// or it counts inbound payments — whose tokens sit at the SOURCE CB's address — as its own.
type BridgeDirection string

const (
	// BridgeDirectionIn mints the mirrored asset on the Hub (lock/burn on the spoke first).
	BridgeDirectionIn BridgeDirection = "IN"
	// BridgeDirectionOut burns the mirrored asset on the Hub and delivers on a spoke.
	BridgeDirectionOut BridgeDirection = "OUT"
)

// BridgeLeg distinguishes what a position settles for a given Hub swap. A single swap
// produces up to two legs: the payment itself and the return of the unspent slippage
// buffer. Both carry the same swap_tx_hash, so replay protection is keyed on the pair.
type BridgeLeg string

const (
	// BridgeLegSettlement is the payment leg: the swap output delivered to the beneficiary.
	// Default for every legacy position (plain lock-mint, dev paths, bridge-out).
	BridgeLegSettlement BridgeLeg = "SETTLEMENT"
	// BridgeLegResidue is the return of MaxAmountIn − realized amount_in to the payer.
	// Bridge-in must move the worst-case input before the swap runs, so the unspent
	// remainder is burned on the Hub and given back on the source spoke.
	BridgeLegResidue BridgeLeg = "RESIDUE"
)

// BridgedAssetPosition tracks a cross-spoke bridging lifecycle (Lock→Active→Burn→Released).
type BridgedAssetPosition struct {
	PositionID   string `gorm:"primaryKey;column:position_id;type:varchar(64)"`
	OwnerBankID  string `gorm:"column:owner_bank_id;not null"`
	SpokeNetwork string `gorm:"column:spoke_network;not null"`
	NativeAsset  string `gorm:"column:native_asset;not null"`
	// MirroredAsset leads the reconciliation index. That query filters on
	// (mirrored_asset, leg, bridge_state) to find what a CB still holds on the Hub for its banks;
	// without it, a gateway with a long payment history would scan the whole table every few
	// minutes. owner_bank_id is deliberately NOT the indexed column — the query groups by it,
	// it does not filter on it.
	MirroredAsset  string `gorm:"column:mirrored_asset;not null;index:idx_bridge_recon,priority:1"`
	MirroredAmount string `gorm:"column:mirrored_amount;not null"`
	// Direction distinguishes a mint-on-Hub from a burn-on-Hub. Set at creation; legacy rows are
	// backfilled from the relayer queue's event type, which is the only durable record of which
	// way a pre-existing position went.
	Direction       BridgeDirection `gorm:"column:direction;default:''"`
	BridgeState     BridgeState     `gorm:"column:bridge_state;not null;default:'LOCKING';index:idx_bridge_recon,priority:3"`
	RelayerRetries  int             `gorm:"column:relayer_retries;not null;default:0"`
	LastAttemptAt   *time.Time      `gorm:"column:last_attempt_at"`
	FirstAttemptAt  *time.Time      `gorm:"column:first_attempt_at"`
	RelayerErrorLog *string         `gorm:"column:relayer_error_log;type:jsonb"`
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
	// SwapTxHash is the Hub AMM swap transaction this position settles (R2-CR-6).
	// Unique per (swap_tx_hash, leg) when set: each on-chain swap can be consumed by
	// exactly one burn/mint *per leg*, so replayed notifications cannot mint twice.
	// Partial index because legacy flows (plain lock-mint, dev paths) have no swap.
	//
	// The index is composite because a swap legitimately produces two positions — the
	// settlement and the residue return — and in a single-CB deployment both land in the
	// same table. Keying uniqueness on swap_tx_hash alone would make the residue leg look
	// like a replay of the settlement and silently drop it.
	SwapTxHash string `gorm:"column:swap_tx_hash;default:'';index:idx_bridge_swap_tx_leg,unique,priority:1,where:swap_tx_hash <> ''"`
	// Leg is SETTLEMENT (the payment) or RESIDUE (return of the unspent slippage buffer).
	// Legacy rows default to SETTLEMENT, which is what they are.
	Leg BridgeLeg `gorm:"column:leg;not null;default:'SETTLEMENT';index:idx_bridge_swap_tx_leg,unique,priority:2;index:idx_bridge_recon,priority:2"`
	// ParentPositionID links a RESIDUE leg to the bridge-in position it corrects. The net
	// amount actually consumed by the swap is parent.mirrored_amount − residue.mirrored_amount;
	// the parent's mirrored_amount is never rewritten, since it records what the chain did.
	ParentPositionID string `gorm:"column:parent_position_id;default:'';index:idx_bridge_parent_position_id"`
	// HubBurnTxHash is the confirmed Hub burn transaction for a bridge-out position (R2-H-12).
	// Written by the payment-orchestrator relayer executor only after the burn is mined with a
	// successful receipt; it is the idempotency key for burn retries so native value is never
	// released on the spoke without a confirmed Hub burn. Completion is tracked by this persisted
	// receipt hash, never inferred from token balance. This column is created here because the
	// gateway owns the AutoMigrate for the shared bridged_asset_positions table.
	HubBurnTxHash string `gorm:"column:hub_burn_tx_hash;default:''"`
	// SpokeMintTxHash is the confirmed Spoke-side mint transaction for a cross-currency
	// bridge-out (R2-H-12 follow-up, mirror of HubBurnTxHash on the spoke leg). Written by the
	// payment-orchestrator relayer executor: a broadcast intent is persisted before the receipt
	// wait, and the value is authoritative once the mint is mined successfully. It is the
	// idempotency key for the spoke mint so a WaitMined timeout on a landed mint cannot issue
	// unbacked tCeBM to the beneficiary twice. Column created here because the gateway owns the
	// AutoMigrate for the shared bridged_asset_positions table.
	SpokeMintTxHash string `gorm:"column:spoke_mint_tx_hash;default:''"`
	// SpokeFundTxHash is the confirmed Spoke-side mint that funded THIS position's sovereign
	// lock. Before it existed, the relayer decided whether to mint by reading the signer's
	// balance: a position whose predecessor had left tokens on that address locked those
	// instead of minting its own, so the CB's tCeBM supply stopped corresponding to positions.
	// Recorded on broadcast, like the burn/mint hashes above, so a retry reconciles by hash
	// instead of funding twice.
	SpokeFundTxHash string `gorm:"column:spoke_fund_tx_hash;default:''"`
	// CorrelationID links the position to the cross-currency swap operation (009), and on a
	// bridge-in it is also the replay key.
	//
	// Bridge-out keys replay on (swap_tx_hash, leg), which bridge-in cannot use: it runs
	// BEFORE the swap, so no swap hash exists yet. The correlation id is what the relay
	// notification carries, so a retried notification carries the same one — which is exactly
	// what makes it usable as the idempotency key, and exactly what made its absence a
	// double-charge (R2-CR-6 follow-up).
	//
	// Uniqueness is scoped to inbound positions, not global, because two other kinds of row
	// legitimately share one correlation id:
	//   - the RESIDUE leg, which returns the unspent slippage buffer (direction OUT);
	//   - the settlement bridge-out of the same swap (direction OUT).
	// A rollback does not add a row at all — it transitions this position to BURNING.
	// Scoping on direction is therefore what lets replay protection coexist with the legs a
	// swap is supposed to produce. Rows with an empty correlation id (plain lock-mint, dev
	// paths) stay outside the index: they have no notification to replay.
	CorrelationID string    `gorm:"column:correlation_id;default:'';index:idx_bridge_correlation_id;index:idx_bridge_in_correlation,unique,where:direction = 'IN' AND correlation_id <> ''"`
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
