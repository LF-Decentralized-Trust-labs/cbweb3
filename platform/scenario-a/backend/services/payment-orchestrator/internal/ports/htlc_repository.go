package ports

import "context"

// HTLCRepository provides off-chain persistence for HTLC coordination records.
// The default (nil) configuration uses in-memory state only; production deployments
// should supply a database-backed implementation so that CounterpartyLocked and
// other flags survive service restarts.
type HTLCRepository interface {
	// SetCounterpartyLocked marks the local HTLC identified by contractID as
	// having its counterparty leg confirmed on-chain. Called by the relay worker
	// when it observes the mirror HTLC lock event from the remote spoke.
	SetCounterpartyLocked(ctx context.Context, contractID string) error
}
