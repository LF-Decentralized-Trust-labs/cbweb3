// SPDX-License-Identifier: Apache-2.0

// Package amm provides a thin client over the AutomatedMarketMaker contract's
// asymmetric circuit breaker on Hyperledger Besu.
//
// The on-chain breaker is deliberately asymmetric (R2-H-2):
//   - pause  = 1-of-N fail-safe: any single governance-capable Central Bank may halt.
//   - resume = 2-of-N quorum: a resume proposal needs signatures from two DISTINCT
//     governance keys before the AMM restarts. A single key can never resume alone.
//
// The Breaker interface exposes exactly what the compliance service needs to drive
// this model from the Governance Portal path — no more. Resume is modelled as a
// single high-level "vote": ResumeVote signs the active proposal for the current
// pause epoch, or opens one if none exists. Because the contract rejects a second
// signature from the same signer (and binds proposals to the pause epoch), one
// operator/key can contribute at most one signature and therefore can never resume
// unilaterally. Distinct Central Banks — typically separate compliance instances,
// each holding its own CB_PRIVATE_KEY, sharing only the chain — converge to quorum
// through the on-chain events, not through any shared database.
package amm

import (
	"context"
	"errors"
)

// ErrNotPaused is returned by ResumeVote when the breaker is not engaged, so there
// is nothing to resume.
var ErrNotPaused = errors.New("amm: circuit breaker is not paused")

// ErrNoSigner is returned when a write is attempted without a configured signer.
var ErrNoSigner = errors.New("amm: no transaction signer configured")

// Breaker is the minimal on-chain circuit-breaker surface used by the compliance
// service. Implementations are the live BesuBreaker and, in tests, an in-memory fake.
type Breaker interface {
	// Pause engages the breaker (1-of-N fail-safe). Returns the mined tx hash.
	Pause(ctx context.Context, reason string) (txHash string, err error)

	// ResumeVote casts this signer's 2-of-N resume vote: it signs the current pause
	// epoch's active resume proposal, or opens a new one if none exists yet. It never
	// resumes on its own — quorum requires a second, distinct governance signer.
	// Returns the mined tx hash of the action taken (propose or sign).
	ResumeVote(ctx context.Context) (txHash string, err error)

	// IsPaused reports the on-chain breaker state — the single source of truth that
	// the compliance service mirrors into its system-parameter store.
	IsPaused(ctx context.Context) (bool, error)
}
