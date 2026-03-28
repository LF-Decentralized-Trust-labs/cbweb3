package domain

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
