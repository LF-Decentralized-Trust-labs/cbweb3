// Package app — on-chain counterpart commit source for cooperative liquidity discovery.
// Reads the Hub LiquidityCommitRegistry (the only cross-CB source of truth) so a CB
// gateway can surface a counterpart central bank's open PENDING commit on the opposite
// side of its pool, enabling independent coordination without out-of-band signalling.
package app

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// commitStatusPending is the on-chain CommitStatus enum value for PENDING (A=0 in CommitSide,
// PENDING=0 in CommitStatus — see liquidityCommitRegistryABI).
const commitStatusPending uint8 = 0

// commitRegistryReader is the read surface of LiquidityCommitRegistryClient used for
// counterpart discovery. Defined as an interface for testability.
type commitRegistryReader interface {
	GetPendingCommit(ctx context.Context, poolPair string, side uint8) ([32]byte, error)
	GetCommit(ctx context.Context, commitId [32]byte) (CommitView, error)
}

// onChainCounterpartSource implements services.CounterpartSource by querying the Hub
// LiquidityCommitRegistry for a PENDING commit on the side opposite to this gateway's own.
type onChainCounterpartSource struct {
	client  commitRegistryReader
	ownSide uint8 // 0 = A, 1 = B — the side this gateway commits.
}

// newOnChainCounterpartSource constructs an onChainCounterpartSource. ownSide is the
// gateway's configured CommitSide ("A" or "B"); the source queries the opposite side.
func newOnChainCounterpartSource(client commitRegistryReader, ownSide string) *onChainCounterpartSource {
	side := uint8(0)
	if ownSide == "B" {
		side = 1
	}
	return &onChainCounterpartSource{client: client, ownSide: side}
}

// CounterpartCommit returns the counterpart's PENDING commit on the opposite side of the
// given pool, or nil when none exists. Identity is the raw signer address only.
func (s *onChainCounterpartSource) CounterpartCommit(ctx context.Context, poolPair string) (*services.CounterpartCommit, error) {
	oppositeSide := uint8(1) - s.ownSide

	id, err := s.client.GetPendingCommit(ctx, poolPair, oppositeSide)
	if err != nil {
		return nil, err
	}
	if id == ([32]byte{}) {
		return nil, nil
	}

	commit, err := s.client.GetCommit(ctx, id)
	if err != nil {
		return nil, err
	}
	if commit.Status != commitStatusPending {
		return nil, nil
	}

	amount := "0"
	if commit.Amount != nil {
		amount = commit.Amount.String()
	}

	return &services.CounterpartCommit{
		Side:            sideLabel(oppositeSide),
		SignerAddress:   commit.Signer.Hex(),
		Amount:          amount,
		ExpiresAt:       time.Unix(int64(commit.ExpiresAt), 0).UTC(),
		OnChainCommitID: CommitIDToHex(id),
	}, nil
}

// sideLabel maps the on-chain CommitSide enum (0=A, 1=B) to its string label.
func sideLabel(side uint8) string {
	if side == 1 {
		return "B"
	}
	return "A"
}
