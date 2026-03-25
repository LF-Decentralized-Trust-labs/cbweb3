// Package registry provides interfaces and implementations for interacting with
// the ParticipantRegistry smart contract on Hyperledger Besu.
//
// RegistryWriter covers write operations (register, activate, deactivate).
// RegistryReader covers read-only queries (authorization check, role lookup).
//
// Both BesuClient and NoopRegistryClient implement the full set of interfaces.
// Services should declare only the interface they need:
//   - auth-service: RegistryWriter + RegistryReader
//   - compliance-orchestrator: RegistryWriter only
package registry

import "context"

// RegistryWriter covers write operations on the ParticipantRegistry contract.
type RegistryWriter interface {
	// SetParticipant registers or updates a participant on-chain.
	// active=true activates; active=false deactivates (sets role code to 0).
	SetParticipant(ctx context.Context, wallet, role string, active bool) (txHash string, err error)

	// SetCertFingerprint stores the SHA-256 fingerprint of a participant's
	// X.509 certificate on-chain, binding the PKI identity to the wallet.
	SetCertFingerprint(ctx context.Context, wallet string, fingerprint [32]byte) (txHash string, err error)
}

// RegistryReader covers read-only queries on the ParticipantRegistry contract.
type RegistryReader interface {
	// IsMemberAuthorized returns true if the wallet is registered and active.
	IsMemberAuthorized(ctx context.Context, address string) (bool, error)

	// GetMemberRole returns the uint8 role code registered for the address.
	GetMemberRole(ctx context.Context, address string) (uint8, error)

	// GetCertFingerprint returns the SHA-256 fingerprint stored on-chain
	// for the given address. Returns [32]byte{} if not set.
	GetCertFingerprint(ctx context.Context, address string) ([32]byte, error)
}
