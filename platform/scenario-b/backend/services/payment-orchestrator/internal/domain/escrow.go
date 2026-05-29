// Package domain defines the core business types for the payment-orchestrator.
package domain

import "time"

// DepositStatus represents the lifecycle state of a fiat deposit request.
type DepositStatus string

const (
	DepositStatusPending    DepositStatus = "PENDING"
	DepositStatusApproved   DepositStatus = "APPROVED"
	DepositStatusRejected   DepositStatus = "REJECTED"
	DepositStatusMintFailed DepositStatus = "MINT_FAILED"
)

// RedeemStatus represents the lifecycle state of a redeem (de-tokenization) request.
type RedeemStatus string

const (
	RedeemStatusPending  RedeemStatus = "PENDING"
	RedeemStatusApproved RedeemStatus = "APPROVED"
	RedeemStatusRejected RedeemStatus = "REJECTED"
)

// EscrowStatus represents the lifecycle state of an escrow (tokenization) request.
type EscrowStatus string

const (
	EscrowStatusPending  EscrowStatus = "PENDING"
	EscrowStatusApproved EscrowStatus = "APPROVED"
	EscrowStatusRejected EscrowStatus = "REJECTED"
)

// EscrowRecord is the off-chain representation of a tokenization request
// where a commercial bank converts fCeBM into tCeBM via burn+mint on-chain.
// On approval, the Central Bank burns fCeBM from the bank's Besu address and
// mints the equivalent tCeBM to the same address.
type EscrowRecord struct {
	ID              string       `json:"id"`
	RequesterID     string       `json:"requester_id"`
	BesuAddress     string       `json:"besu_address"`
	DepositID       string       `json:"deposit_id"`
	Amount          string       `json:"amount"`
	Status          EscrowStatus `json:"status"`
	BurnTxHash      string       `json:"burn_tx_hash,omitempty"`
	MintTxHash      string       `json:"mint_tx_hash,omitempty"`
	RejectionReason string       `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
}

// DepositRecord is the off-chain representation of a fiat deposit request
// initiated by a commercial bank and processed by the central bank.
// On approval, fCeBM is minted to the bank's Besu address.
type DepositRecord struct {
	ID                   string        `json:"id"`
	RequesterID          string        `json:"requester_id"`
	RequesterBesuAddress string        `json:"requester_besu_address"`
	Amount               string        `json:"amount"`
	Status               DepositStatus `json:"status"`
	FiatMintTxHash       string        `json:"fiat_mint_tx_hash,omitempty"`
	RejectionReason      string        `json:"rejection_reason,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
}

// RedeemRecord is the off-chain representation of a redeem (de-tokenization) request
// where the central bank mints tCeBM back to the commercial bank.
type RedeemRecord struct {
	ID                   string       `json:"id"`
	RequesterID          string       `json:"requester_id"`
	RequesterBesuAddress string       `json:"requester_besu_address"`
	Amount               string       `json:"amount"`
	Status               RedeemStatus `json:"status"`
	MintTxHash           string       `json:"mint_tx_hash,omitempty"`
	RejectionReason      string       `json:"rejection_reason,omitempty"`
	CreatedAt            time.Time    `json:"created_at"`
}
