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
	CreatedAt               time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt               time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
