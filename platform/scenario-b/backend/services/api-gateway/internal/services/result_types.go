// SPDX-License-Identifier: Apache-2.0

// Package services defines result types shared between services and handlers.
package services

import "time"

// BridgePositionResult is the service-level result for bridge position operations.
type BridgePositionResult struct {
	PositionID      string `json:"position_id"`
	OwnerBankID     string `json:"owner_bank_id"`
	SpokeNetwork    string `json:"spoke_network"`
	NativeAsset     string `json:"native_asset"`
	MirroredAsset   string `json:"mirrored_asset"`
	MirroredAmount  string `json:"mirrored_amount"`
	BridgeState     string `json:"bridge_state"`
	RelayerRetries  int    `json:"relayer_retries"`
	RelayerErrorLog string `json:"relayer_error_log,omitempty"`
}

// BridgePositionDetail carries the internal fields of a bridge position, including the
// Hub/spoke addresses that BridgePositionResult deliberately omits. Used to authorize a
// residue return against the bridge-in position the CB created, so the amount and the
// burn-from address come from the CB's own records rather than from a request body.
//
// Not serialized to any public endpoint.
type BridgePositionDetail struct {
	PositionID           string
	OwnerBankID          string
	SpokeNetwork         string
	NativeAsset          string
	MirroredAsset        string
	MirroredAmount       string
	BridgeState          string
	MintToHubAddress     string
	BurnFromSpokeAddress string
	Leg                  string
}

// HubSwapRecord is the CB's record of a Hub AMM swap it executed on behalf of a bank
// (sovereign delegation of Step 2). Keyed on the funding bridge-in position, which backs
// exactly one swap — the replay anchor for a retried delegation.
type HubSwapRecord struct {
	BridgeInPositionID string
	CorrelationID      string
	PayerBankID        string
	PoolPair           string
	AmountOut          string
	AmountIn           string
	SwapTxHash         string
	// Status says whether the trade this record anchors has completed. It exists because the
	// record has to be written BEFORE the trade, not after: the AMM swap is not idempotent
	// on-chain, so two concurrent deliveries of one delegation must not both get past the guard
	// and trade. See HubSwapStatus*.
	Status string
	// FailureReason is why an abandoned claim was abandoned, kept so an operator reconciling the
	// position does not have to correlate logs to find out.
	FailureReason string
}

// The lifecycle of a delegated swap record. The row is inserted PENDING before the trade and moves
// exactly once: to EXECUTED when the realized cost is known, or to FAILED when the trade did not
// complete. A FAILED claim is NOT a free retry — a trade can fail after broadcast, so whether the
// tokens moved is unknown from here and only reconciliation can say.
const (
	HubSwapStatusPending  = "PENDING"
	HubSwapStatusExecuted = "EXECUTED"
	HubSwapStatusFailed   = "FAILED"
)

// DisclosureResult is the service-level result for oversight disclosure operations.
type DisclosureResult struct {
	RequestID            string     `json:"request_id"`
	RequestedByBankID    string     `json:"requested_by_bank_id"`
	TargetTransactionRef string     `json:"target_transaction_ref"`
	ReasonCode           string     `json:"reason_code"`
	State                string     `json:"state"`
	QuorumRequired       int        `json:"quorum_required"`
	QuorumReached        int        `json:"quorum_reached"`
	OpenedAt             time.Time  `json:"opened_at"`
	ExpiresAt            time.Time  `json:"expires_at"`
	ClosedAt             *time.Time `json:"closed_at,omitempty"`
}
