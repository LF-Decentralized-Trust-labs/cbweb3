// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// FXState mirrors the on-chain FXAgreementState enum.
type FXState string

const (
	FXStateInvalid   FXState = "INVALID"
	FXStateProposed  FXState = "PROPOSED"
	FXStateAccepted  FXState = "ACCEPTED"
	FXStateRejected  FXState = "REJECTED"
	FXStateCancelled FXState = "CANCELLED"
	FXStateSettled   FXState = "SETTLED"
)

// IsTerminal returns true if the state cannot transition further.
func (s FXState) IsTerminal() bool {
	return s == FXStateRejected || s == FXStateCancelled || s == FXStateSettled
}

// FXAgreementRecord is the off-chain representation of an FX agreement.
type FXAgreementRecord struct {
	TradeID         string
	Originator      string
	CounterpartyB   string
	SettlementAgent string
	Custodian       string
	Beneficiary     string
	OriginAmount    string
	CounterAmount   string
	OriginCurrency  string
	CounterCurrency string
	Rate            string
	SpokeAReceiver  string // Paladin identity that must receive the HTLC lock on Spoke-A
	SpokeBReceiver  string // Paladin identity that must receive the HTLC lock on Spoke-B
	ExpiryDate      uint64
	State           FXState
	OnChainTxHash   string
	// Pente bilateral context — optional, populated when Pente integration is active.
	GroupID         string // Pente privacy group identifier
	ContractAddress string // deployed FXAgreement Pente contract address
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// EventSource indicates the origin of a lifecycle event.
type EventSource string

const (
	EventSourceLocalAPI  EventSource = "LOCAL_API"
	EventSourceRelay     EventSource = "RELAY"
	EventSourceSystemJob EventSource = "SYSTEM_JOB"
	EventSourceOnBehalf  EventSource = "ON_BEHALF"
)

// FXAgreementEvent is an immutable record of a single state transition.
type FXAgreementEvent struct {
	ID         int64
	TradeID    string
	FromState  FXState
	ToState    FXState
	Actor      string // user/service identity that triggered the transition
	OccurredAt time.Time
	Notes      string      // optional human-readable detail
	TxHash     string      // optional on-chain tx reference
	Source     EventSource // LOCAL_API | RELAY | SYSTEM_JOB | ON_BEHALF
}

// RelayDeliveryRecord tracks cross-spoke event forwarding with idempotency and retry semantics.
type RelayDeliveryRecord struct {
	ID             int64      // unique database identifier
	IdempotencyKey string     // unique key: "${action}:${tradeId}" for dedup
	TradeID        string     // referenced agreement
	EventType      string     // PROPOSED | ACCEPTED | REJECTED | CANCELLED | SETTLED
	SourceSpoke    string     // originating spoke name
	TargetSpoke    string     // destination spoke name
	Status         string     // PENDING | RETRYING | DELIVERED | FAILED
	AttemptCount   int        // number of attempts so far
	NextRetryAt    *time.Time // when to attempt next delivery (exponential backoff)
	LastError      string     // error message from last attempt
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
