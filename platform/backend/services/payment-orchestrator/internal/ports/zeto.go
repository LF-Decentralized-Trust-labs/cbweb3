package ports

import "context"

// ZetoLockResult holds the outputs from a Zeto lock operation.
type ZetoLockResult struct {
	TxHash         string   `json:"tx_hash"`
	ZetoLockRef    string   `json:"zeto_lock_ref"`
	LockedStateIDs []string `json:"locked_state_ids"`
}

// ZetoOperator abstracts Zeto token operations via the Paladin sidecar HTTP API.
// The payment-orchestrator calls this port for private token movements.
type ZetoOperator interface {
	// Mint issues new Zeto tokens to an identity (Central Bank only).
	// The underlying flow: ERC-20 mint -> approve -> Zeto.deposit().
	Mint(ctx context.Context, to string, amount string) (txHash string, err error)

	// Transfer moves Zeto tokens between identities using ZK proofs.
	Transfer(ctx context.Context, to string, amount string) (txHash string, err error)

	// Lock locks Zeto tokens with a delegated lock proof for HTLC coordination.
	// Returns the Zeto lock reference that links to the public HTLC contract.
	Lock(ctx context.Context, amount string, delegate string) (*ZetoLockResult, error)

	// Unlock releases previously locked Zeto tokens back to the owner (refund path).
	Unlock(ctx context.Context, zetoLockRef string) (txHash string, err error)

	// TransferLocked transfers locked tokens to the receiver (settle path).
	// This is called when the HTLC secret is revealed on-chain.
	TransferLocked(ctx context.Context, zetoLockRef string, to string, amount string) (txHash string, err error)

	// Balance returns the Zeto token balance for an identity.
	Balance(ctx context.Context, identity string) (string, error)
}
