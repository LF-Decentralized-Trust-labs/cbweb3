// Package services provides the PoolStatusGate that enforces FR-011:
// swaps MUST be blocked when the pool is not in ACTIVE state.
package services

import "context"

// PoolStatusGate checks whether an AMM pool is in ACTIVE state before allowing a swap (FR-011).
// A pool is ACTIVE when both reserves are non-zero. EMPTY and PENDING_COUNTERPART states
// must cause the swap to be rejected with POOL_NOT_ACTIVE.
type PoolStatusGate interface {
	IsActive(ctx context.Context, pair string) (bool, error)
}

// poolStatusGate is the concrete implementation backed by an AMMPoolReader.
type poolStatusGate struct {
	reader AMMPoolReader
}

// NewPoolStatusGate returns a PoolStatusGate that reads live reserves from the AMM contract.
func NewPoolStatusGate(reader AMMPoolReader) PoolStatusGate {
	return &poolStatusGate{reader: reader}
}

// IsActive returns true only when both reserveA and reserveB are non-zero (ACTIVE state).
// Any error reading reserves is propagated — the caller must treat it as a gate failure.
func (g *poolStatusGate) IsActive(ctx context.Context, pair string) (bool, error) {
	rA, rB, _, err := g.reader.GetPoolReserves(ctx, pair)
	if err != nil {
		return false, err
	}
	aActive := rA != "" && rA != "0"
	bActive := rB != "" && rB != "0"
	return aActive && bActive, nil
}
