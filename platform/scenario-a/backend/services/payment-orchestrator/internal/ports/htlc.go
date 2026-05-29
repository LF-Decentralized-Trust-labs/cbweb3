package ports

import "context"

// HTLCContractPort abstracts on-chain HTLC coordination operations against
// HashTimeLockedContract.sol deployed on the spoke's Besu chain.
// The on-chain contract does NOT hold tokens — it records {hashLock, timeLock}
// and emits events (LogHTLCLocked, LogHTLCClaimed, LogHTLCRefunded) that the
// cross-spoke relay watches to bridge the secret between isolated spokes.
type HTLCContractPort interface {
	// Lock records a new HTLC coordination entry on-chain.
	// The receiver must be an Ethereum address (resolve Paladin identity first).
	Lock(ctx context.Context, params HTLCLockParams) (txHash string, err error)

	// Settle reveals the secret on-chain and transitions state to SETTLED.
	// Emits LogHTLCClaimed(contractId, secret) — the event the relay watches.
	Settle(ctx context.Context, contractID [32]byte, secret [32]byte) (txHash string, err error)

	// Refund marks the on-chain HTLC as refunded after the timeLock expires.
	Refund(ctx context.Context, contractID [32]byte) (txHash string, err error)

	// RegisterAgreementCommitment records an accepted agreement commitment hash
	// for fallback on-chain gating when FX_AGREEMENT integration is unavailable.
	RegisterAgreementCommitment(ctx context.Context, commitment [32]byte) (txHash string, err error)
}

// HTLCLockParams contains the parameters for recording an HTLC lock on-chain.
type HTLCLockParams struct {
	ContractID  [32]byte
	Receiver    string // Ethereum address (0x...)
	HashLock    [32]byte
	TimeLock    uint64
	ZetoLockRef [32]byte
	AgreementID [32]byte // FX trade ID (when FX_AGREEMENT active) or commitment hash fallback
}
