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
