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

// EscrowStatus represents the lifecycle state of an escrow (tokenization) request.
type EscrowStatus string

const (
	EscrowStatusPending  EscrowStatus = "PENDING"
	EscrowStatusApproved EscrowStatus = "APPROVED"
	EscrowStatusRejected EscrowStatus = "REJECTED"
)

// RedeemStatus represents the lifecycle state of a redeem (de-tokenization) request.
type RedeemStatus string

const (
	RedeemStatusPending  RedeemStatus = "PENDING"
	RedeemStatusApproved RedeemStatus = "APPROVED"
	RedeemStatusRejected RedeemStatus = "REJECTED"
)

// DepositRecord is the off-chain representation of a fiat deposit request
// initiated by a commercial bank and processed by the central bank.
type DepositRecord struct {
	ID                       string        `json:"id"`
	RequesterID              string        `json:"requester_id"`
	RequesterBesuAddress     string        `json:"requester_besu_address"`
	RequesterPaladinIdentity string        `json:"requester_paladin_identity"`
	Amount                   string        `json:"amount"`
	Status                   DepositStatus `json:"status"`
	MintTxHash               string        `json:"mint_tx_hash,omitempty"`
	RejectionReason          string        `json:"rejection_reason,omitempty"`
	CreatedAt                time.Time     `json:"created_at"`
}

// EscrowRecord is the off-chain representation of an escrow (tokenization) request
// where fCeBM on Besu is burned and tCeBM on Paladin/Zeto is minted.
type EscrowRecord struct {
	ID                       string       `json:"id"`
	RequesterID              string       `json:"requester_id"`
	RequesterBesuAddress     string       `json:"requester_besu_address"`
	RequesterPaladinIdentity string       `json:"requester_paladin_identity"`
	Amount                   string       `json:"amount"`
	Status                   EscrowStatus `json:"status"`
	BurnTxHash               string       `json:"burn_tx_hash,omitempty"`
	MintTxHash               string       `json:"mint_tx_hash,omitempty"`
	RejectionReason          string       `json:"rejection_reason,omitempty"`
	CreatedAt                time.Time    `json:"created_at"`
}

// RedeemRecord is the off-chain representation of a redeem (de-tokenization) request
// where tCeBM on Paladin/Zeto is burned and fCeBM on Besu is minted.
type RedeemRecord struct {
	ID                       string       `json:"id"`
	RequesterID              string       `json:"requester_id"`
	RequesterBesuAddress     string       `json:"requester_besu_address"`
	RequesterPaladinIdentity string       `json:"requester_paladin_identity"`
	Amount                   string       `json:"amount"`
	Status                   RedeemStatus `json:"status"`
	ZetoTransferTxHash       string       `json:"zeto_transfer_tx_hash,omitempty"`
	FiatMintTxHash           string       `json:"fiat_mint_tx_hash,omitempty"`
	RejectionReason          string       `json:"rejection_reason,omitempty"`
	CreatedAt                time.Time    `json:"created_at"`
}
