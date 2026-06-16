// SPDX-License-Identifier: Apache-2.0

// Package domain defines Circuit Breaker signature models for Scenario B governance (FR-030).
package domain

import "time"

// CBSignatureEventKind enumerates the circuit breaker event types.
type CBSignatureEventKind string

const (
	CBEventKindPause  CBSignatureEventKind = "PAUSE"
	CBEventKindResume CBSignatureEventKind = "RESUME"
)

// CircuitBreakerSignature records an institutional signature for a circuit breaker event.
// Append-only (FR-048 / data-model.md §5a).
type CircuitBreakerSignature struct {
	SignatureID     string               `gorm:"primaryKey;column:signature_id;type:varchar(64)"`
	ControlID       string               `gorm:"column:control_id;not null;index"`
	EventKind       CBSignatureEventKind `gorm:"column:event_kind;not null"`
	RequestID       string               `gorm:"column:request_id;index"`
	SignerBankID    string               `gorm:"column:signer_bank_id;not null"`
	SignerWallet    string               `gorm:"column:signer_wallet;not null"`
	SignaturePayload []byte              `gorm:"column:signature_payload;type:bytea"`
	OnChainTxRef    string               `gorm:"column:on_chain_tx_ref"`
	SignedAt        time.Time            `gorm:"column:signed_at;not null;autoCreateTime"`
}
