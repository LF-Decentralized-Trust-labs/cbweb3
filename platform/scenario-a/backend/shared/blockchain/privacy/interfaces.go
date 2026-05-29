// Package privacy provides interfaces and implementations for interacting with
// Hyperledger Paladin — the privacy layer between the Hub & Spoke and Besu networks.
//
// Paladin handles confidential transactions (Noto HTLC, Zeto ZK proofs, Private AMM)
// via its SDK and Sidecar nodes, which in turn submit transactions to Besu.
//
// Governance operations (ParticipantRegistry) use direct Besu calls (see registry package).
// This package covers only privacy-domain operations under /api/v1/privacy/*.
//
// Current status: PaladinBypass is the only implementation — all operations are
// no-ops until the Paladin SDK integration is built (payment-orchestrator, liquidity-service).
package privacy

import "context"

// PrivacyOperator covers privacy-domain operations delegated to Paladin.
// The Paladin SDK translates these into private transactions submitted via
// Paladin Sidecar nodes to the Besu network.
type PrivacyOperator interface {
	// MintNoto mints Noto tokens (Central Bank digital currency issuance).
	MintNoto(ctx context.Context, to, amount string) (txHash string, err error)

	// TransferZeto transfers Zeto tokens with a ZK proof for privacy.
	TransferZeto(ctx context.Context, from, to, amount string) (txHash string, err error)

	// CreateNotoHTLC creates a hash-time-locked contract for cross-border settlement.
	CreateNotoHTLC(ctx context.Context, hash, timeout, amount string) (contractAddr string, err error)

	// ClaimNotoHTLC claims a Noto HTLC by revealing the pre-image.
	ClaimNotoHTLC(ctx context.Context, contractAddr, preimage string) (txHash string, err error)
}
