// SPDX-License-Identifier: Apache-2.0

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
	"fmt"
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
	InstitutionID   [32]byte
	Role            uint8
	Status          uint8
	ZkPointer       [32]byte
	CertFingerprint [32]byte
	LastUpdate      *big.Int
}

// RegistryWriter covers write operations on the IdentityRegistry contract.
type RegistryWriter interface {
	// RegisterParticipant registers a new participant on-chain with the given wallet
	// address, legal name, role, zero-knowledge proof pointer, and institution-level
	// identifier shared by every wallet of the same institution (see
	// InstitutionIDForParticipant — the contract rejects a zero id). The participant is
	// created in the Pending state (step 1 of the two-step onboarding); VerifyParticipant
	// is required before it can transact.
	RegisterParticipant(ctx context.Context, wallet, name, role string, zkPointer, institutionID [32]byte) (txHash string, err error)

	// VerifyParticipant promotes a previously registered participant from Pending
	// to Verified (step 2 of the two-step onboarding). The configured signer must
	// hold VERIFIER_ROLE. Reverts if the participant is not currently Pending.
	VerifyParticipant(ctx context.Context, wallet string) (txHash string, err error)

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

	// GetInstitutionID returns the institution-level identifier for a registered wallet,
	// or [32]byte{} if the wallet is not registered.
	GetInstitutionID(ctx context.Context, address string) ([32]byte, error)
}

// EnsureVerifiedParticipant runs the full two-step onboarding (register -> verify)
// for a participant that has already cleared the off-chain compliance flow, so it
// ends up Verified and able to transact. It is the single choke point services use
// to onboard on-chain (R1-10.6 / R2-10.6): registerParticipant alone only creates a
// Pending participant, which cannot transact (HTLC.lock reverts ParticipantNotVerified).
//
// It is idempotent and, critically, never demotes: a wallet that can already
// transact is left untouched (re-running onboarding must not reset a live
// participant to Pending and revert its in-flight HTLC locks/claims). For a wallet
// that is registered-but-Pending, it re-registers (harmless — same state) and verifies.
//
// The signer must hold GOVERNANCE_ROLE (register) and VERIFIER_ROLE (verify). In
// single-operator/local-dev the bootstrap admin holds both. In production these
// SHOULD be separated (see docs/runbooks/identity-registry-role-separation.md); a
// dedicated verifier signer is the residual required to make that split real
// end-to-end from the Go services.
func EnsureVerifiedParticipant(ctx context.Context, w RegistryWriter, wallet, name, role string, zkPointer, institutionID [32]byte) (txHash string, err error) {
	// No-demotion idempotency guard: skip entirely if already transactable.
	if reader, ok := w.(RegistryReader); ok {
		if can, cerr := reader.CanTransact(ctx, wallet); cerr == nil && can {
			return "", nil
		}
	}
	if _, err = w.RegisterParticipant(ctx, wallet, name, role, zkPointer, institutionID); err != nil {
		return "", err
	}
	txHash, err = w.VerifyParticipant(ctx, wallet)
	if err != nil {
		return txHash, fmt.Errorf("registry: verify after register for %s: %w", wallet, err)
	}
	return txHash, nil
}
