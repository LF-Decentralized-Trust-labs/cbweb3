// Package domain defines the core business types for the payment-orchestrator.
package domain

import "time"

// HTLCState mirrors the on-chain HTLCState enum.
type HTLCState string

const (
	HTLCStateInvalid   HTLCState = "INVALID"
	HTLCStatePending   HTLCState = "PENDING"
	HTLCStateLocked    HTLCState = "LOCKED"
	HTLCStateSettled   HTLCState = "SETTLED"
	HTLCStateRefunded  HTLCState = "REFUNDED"
	HTLCStateSettling  HTLCState = "SETTLING"  // transient: on-chain settle in progress
	HTLCStateRefunding HTLCState = "REFUNDING" // transient: on-chain refund in progress
)

// HTLCRecord is the off-chain representation of an HTLC lock,
// coordinated via the on-chain HashTimeLockedContract on Besu.
type HTLCRecord struct {
	ContractID  string    `json:"contract_id"`
	AgreementID string    `json:"agreement_id"`
	Sender      string    `json:"sender"`
	Receiver    string    `json:"receiver"`
	Amount      string    `json:"amount"`
	HashLock    string    `json:"hash_lock"`
	TimeLock    uint64    `json:"time_lock"`
	Secret      string    `json:"secret,omitempty"`
	State       HTLCState `json:"state"`
	HTLCTxHash  string    `json:"htlc_tx_hash"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
