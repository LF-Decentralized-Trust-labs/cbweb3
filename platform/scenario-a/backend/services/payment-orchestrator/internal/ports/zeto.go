// SPDX-License-Identifier: Apache-2.0

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

	// Burn destroys Zeto tokens previously held by the caller (Central Bank only).
	// Underlying flow: Zeto.withdraw(amount) -> ERC-20 returned -> ERC-20.burn().
	Burn(ctx context.Context, from string, amount string) (txHash string, err error)

	// Transfer moves Zeto tokens between identities using ZK proofs.
	Transfer(ctx context.Context, to string, amount string) (txHash string, err error)

	// Lock locks Zeto tokens with a delegated lock proof for HTLC coordination.
	// Returns the Zeto lock reference that links to the public HTLC contract.
	Lock(ctx context.Context, amount string, delegate string) (*ZetoLockResult, error)

	// TransferLocked transfers locked tokens to a recipient (settle path to receiver,
	// or refund path back to the original sender).
	// This is called when the HTLC secret is revealed on-chain.
	TransferLocked(ctx context.Context, zetoLockRef string, to string, amount string) (txHash string, err error)

	// Balance returns the Zeto token balance for the identity configured in the service.
	Balance(ctx context.Context) (string, error)

	// ResolveIdentity resolves a Paladin identity string (e.g. "funded_operator@spoke-a-bank-c")
	// to its EVM address (e.g. "0xc110...") via ptx_resolveVerifier.
	ResolveIdentity(ctx context.Context, identity string) (string, error)
}
