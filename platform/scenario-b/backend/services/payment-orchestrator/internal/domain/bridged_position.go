// SPDX-License-Identifier: Apache-2.0

// Package domain defines the BridgedAssetPosition model for Scenario B bridging.
package domain

import "time"

// BridgeState enumerates the lifecycle states of a bridged asset position (FR-029 / FR-031 / FR-032).
type BridgeState string

const (
	BridgeStateLocking                BridgeState = "LOCKING"
	BridgeStateActive                 BridgeState = "ACTIVE"
	BridgeStateBurning                BridgeState = "BURNING"
	BridgeStateBurned                 BridgeState = "BURNED"
	BridgeStateReleased               BridgeState = "RELEASED"
	BridgeStateReconciliationRequired BridgeState = "RECONCILIATION_REQUIRED"
)

// BridgedAssetPosition models a cross-chain asset position created by the Lock&Mint / Burn&Unlock cycle.
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
	RelayerErrorLog string      `gorm:"column:relayer_error_log;type:jsonb"`
	// BurnFromHubAddress overrides the default burnFrom in the Relayer executor for
	// cross-currency bridge-out (009). When empty, the executor uses HUB_MINT_RECIPIENT.
	// Populated with the Hub address that actually holds the wrapped tokens after the AMM swap.
	BurnFromHubAddress string `gorm:"column:burn_from_hub_address;default:''"`
	// MintToHubAddress overrides the default mint recipient for cross-currency bridge-in (009).
	// When set, the issuing CB mints W-<source> to this Hub address — the initiating gateway's
	// swap signer, which executes the AMM swap (Step 2). When empty, the executor falls back to
	// HUB_MINT_RECIPIENT. Populated by the gateway's CrossCurrencyBridgeInHandler.
	MintToHubAddress string `gorm:"column:mint_to_hub_address;default:''"`
	// BeneficiarySpokeAddress is the Spoke-B on-chain address (EVM) of the beneficiary bank.
	// For cross-currency bridge-out, CB-B mints tCeBM directly to this address
	// (CENTRAL_BANK_ROLE) instead of calling SpokeBridge.release() which requires a prior lock.
	BeneficiarySpokeAddress string `gorm:"column:beneficiary_spoke_address;default:''"`
	// BurnFromSpokeAddress is the Spoke-A on-chain address of the payer bank (e.g. bank-a).
	// When set on a LOCK_MINT position, the executor burns tCeBM from this address instead of
	// auto-minting new tCeBM for CB-A to lock. Enforces that the bank must hold tokenized
	// reserves (obtained via Reserve Tokenisation / ApproveEscrow) before a bridge-in can proceed.
	// Empty string = sovereign CB self-service path (CB mints its own liquidity).
	BurnFromSpokeAddress string `gorm:"column:burn_from_spoke_address;default:''"`
	// SwapTxHash is the Hub AMM swap transaction this bridge-out settles (R2-CR-6).
	// Unique when set (partial index, owned by the api-gateway migration): each on-chain
	// swap can be consumed by exactly one burn/mint — replay protection.
	SwapTxHash string `gorm:"column:swap_tx_hash;default:''"`
	// HubBurnTxHash is the confirmed Hub burn transaction for this bridge-out position (R2-H-12).
	// It is persisted only after the burn transaction is mined with a successful receipt, and it
	// is the idempotency key for burn retries: a non-empty value means the Hub burn is confirmed
	// on-chain, so a retry skips the burn and resumes at the spoke release/mint step. Native value
	// is never released on the spoke while this is empty. Mirrors the swap_tx_hash replay guard
	// (R2-CR-6) — completion is tracked by persisted receipt, never inferred from token balance.
	HubBurnTxHash string `gorm:"column:hub_burn_tx_hash;default:''"`
	// CorrelationID links the position to the cross-currency swap operation (009) for tracing.
	CorrelationID string    `gorm:"column:correlation_id;default:''"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
