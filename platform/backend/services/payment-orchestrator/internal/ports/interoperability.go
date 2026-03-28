// Package ports defines the hexagonal architecture port interfaces for the
// payment-orchestrator service.
package ports

import "context"

// InteroperabilityProof carries the cryptographic proof and metadata that a
// Cacti Relayer (or equivalent interoperability layer) captures when a
// cross-chain event is detected.
type InteroperabilityProof struct {
	SourceChain   string `json:"source_chain"`
	ContractID    string `json:"contract_id"`
	EventName     string `json:"event_name"`
	HashLock      string `json:"hash_lock"`
	TimeLock      uint64 `json:"time_lock"`
	ZetoLockRef   string `json:"zeto_lock_ref"`
	ProofPayload  []byte `json:"proof_payload"`
	CorrelationID string `json:"correlation_id"`
}

// InteroperabilityPort abstracts the cross-chain relay layer (Cacti / custom).
// The payment-orchestrator calls this port to:
//   - Subscribe to lock events from the counterparty spoke.
//   - Relay cryptographic proofs to trigger settlement or refund on the local spoke.
//
// Implementations:
//   - StubRelay (this package)   — in-memory mock for development and testing.
//   - CactiRelay (future)        — real Hyperledger Cacti integration.
type InteroperabilityPort interface {
	// SubscribeLockEvents registers a handler that will be called each time a
	// LogHTLCLocked event is detected on the counterparty spoke. The handler
	// runs in its own goroutine; cancel the context to stop the subscription.
	SubscribeLockEvents(ctx context.Context, handler func(proof InteroperabilityProof) error) error

	// SubscribeSettleEvents registers a handler that will be called each time a
	// LogHTLCClaimed event is detected on the counterparty spoke.
	SubscribeSettleEvents(ctx context.Context, handler func(proof InteroperabilityProof) error) error

	// RelayProof sends a cryptographic proof from the local spoke to the
	// counterparty spoke via the interoperability layer. Returns the relay
	// transaction ID.
	RelayProof(ctx context.Context, proof InteroperabilityProof) (relayTxID string, err error)

	// VerifyProof validates an incoming proof from the counterparty spoke.
	// Returns true if the proof is valid and can be trusted.
	VerifyProof(ctx context.Context, proof InteroperabilityProof) (bool, error)
}
