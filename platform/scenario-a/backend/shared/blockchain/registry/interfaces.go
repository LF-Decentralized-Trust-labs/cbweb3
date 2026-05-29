// Package registry provides interfaces and implementations for interacting with
// the IdentityRegistry smart contract on Hyperledger Besu.
//
// RegistryWriter covers write operations (register, update status, set cert).
// RegistryReader covers read-only queries (canTransact, isWhitelisted, getParticipant).
//
// Both BesuClient and NoopRegistryClient implement the full set of interfaces.
// Services should declare only the interface they need:
//   - auth-service: RegistryWriter + RegistryReader
//   - compliance-orchestrator: RegistryWriter only
package registry

import (
	"context"
	"math/big"
)

// ParticipantRole mirrors the IdentityRegistryLibrary.ParticipantRole enum.
const (
	RoleNone           uint8 = 0
	RoleTreasury       uint8 = 1
	RoleGovernance     uint8 = 2
	RoleCentralBank    uint8 = 3
	RoleCommercialBank uint8 = 4
)

// KycStatus mirrors the IdentityRegistryLibrary.KycStatus enum.
const (
	KycStatusNone      uint8 = 0
	KycStatusPending   uint8 = 1
	KycStatusVerified  uint8 = 2
	KycStatusSuspended uint8 = 3
	KycStatusExpired   uint8 = 4
)

// OnChainParticipant is the Go representation of the Solidity Participant struct.
type OnChainParticipant struct {
	LegalName       string
	Role            uint8
	Status          uint8
	ZkPointer       [32]byte
	CertFingerprint [32]byte
	LastUpdate      *big.Int
}

// RegistryWriter covers write operations on the IdentityRegistry contract.
type RegistryWriter interface {
	// RegisterParticipant registers a new participant on-chain with the given
	// wallet address, legal name, role, and zero-knowledge proof pointer.
	RegisterParticipant(ctx context.Context, wallet, name, role string, zkPointer [32]byte) (txHash string, err error)

	// UpdateStatus changes the KYC status of an on-chain participant.
	UpdateStatus(ctx context.Context, wallet string, status uint8) (txHash string, err error)

	// SetCertFingerprint stores the SHA-256 fingerprint of a participant's
	// X.509 certificate on-chain, binding the PKI identity to the wallet.
	SetCertFingerprint(ctx context.Context, wallet string, fingerprint [32]byte) (txHash string, err error)
}

// RegistryReader covers read-only queries on the IdentityRegistry contract.
type RegistryReader interface {
	// CanTransact returns true if the wallet is Verified and has a non-NONE role.
	CanTransact(ctx context.Context, address string) (bool, error)

	// IsWhitelisted returns true if the wallet has Verified KYC status.
	IsWhitelisted(ctx context.Context, address string) (bool, error)

	// GetParticipant returns the full on-chain participant profile.
	GetParticipant(ctx context.Context, address string) (OnChainParticipant, error)

	// GetCertFingerprint returns the SHA-256 fingerprint stored on-chain
	// for the given address. Returns [32]byte{} if not set.
	GetCertFingerprint(ctx context.Context, address string) ([32]byte, error)
}
