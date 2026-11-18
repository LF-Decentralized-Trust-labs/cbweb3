// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

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
	SourceSpokeId  string // Spoke ID of the initiating leg (e.g. "spoke-brl")
	DestSpokeId    string // Spoke ID of the receiving leg (e.g. "spoke-usd")
	SourceReceiver string // Paladin identity on source spoke
	DestReceiver   string // Paladin identity on dest spoke
	ExpiryDate      uint64
	State           FXState
	OnChainTxHash   string
	// Pente bilateral context — optional, populated when Pente integration is active.
	GroupID         string // Pente privacy group identifier
	ContractAddress string // deployed FXAgreement Pente contract address
	// Pending records an on-chain operation that was, or may have been, accepted by Paladin but
	// whose outcome is not known yet (see FXPending). Empty when nothing is pending.
	Pending   FXPending
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FXPendingOp names an on-chain FX operation whose outcome is not known yet.
type FXPendingOp string

const (
	FXPendingPropose FXPendingOp = "propose"
	FXPendingAccept  FXPendingOp = "accept"
	FXPendingReject  FXPendingOp = "reject"
	FXPendingCancel  FXPendingOp = "cancel"
	FXPendingSettle  FXPendingOp = "settle"
)

// TargetState is the on-chain state the agreement reaches when the operation completes.
func (op FXPendingOp) TargetState() FXState {
	switch op {
	case FXPendingPropose:
		return FXStateProposed
	case FXPendingAccept:
		return FXStateAccepted
	case FXPendingReject:
		return FXStateRejected
	case FXPendingCancel:
		return FXStateCancelled
	case FXPendingSettle:
		return FXStateSettled
	default:
		return FXStateInvalid
	}
}

// FXPending is an operation submitted to Paladin whose receipt has not been seen.
//
// Paladin v1.0.0 can leave a private transaction pending indefinitely and cannot cancel it
// (upstream LFDT-Paladin/paladin#1356). Such a transaction may still commit, days later. The
// record keeps the operation so that the database can be reconciled with the chain when it
// does (FXPendingReconciler), and so that no second operation is submitted for the same
// agreement meanwhile.
//
// A pending propose is stored with State INVALID: the agreement is not on chain yet, and the
// relay forwards only PROPOSED agreements, so nothing is propagated until it is confirmed.
type FXPending struct {
	Op    FXPendingOp
	TxID  string    // Paladin transaction id; empty when the submission got no answer at all
	Since time.Time // when the operation was submitted
}

// IsSet reports whether an operation is pending.
func (p FXPending) IsSet() bool { return p.Op != "" }

// OnChainTradeID is the bytes32 trade id an off-chain trade id maps to on chain: the id itself
// when it is 32 bytes of hex, otherwise its SHA-256.
func OnChainTradeID(tradeID string) [32]byte {
	var out [32]byte
	if b, err := hex.DecodeString(tradeID); err == nil && len(b) == 32 {
		copy(out[:], b)
		return out
	}
	return sha256.Sum256([]byte(tradeID))
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
