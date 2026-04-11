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
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
