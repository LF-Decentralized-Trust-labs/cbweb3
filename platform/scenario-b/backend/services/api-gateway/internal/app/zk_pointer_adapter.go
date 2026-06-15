// Package app provides the ZK pointer adapter that bridges ZKPointerGate to the handler interface.
package app

import (
	"context"
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// zkPointerAdapter adapts services.ZKPointerGate to handlers.ZKPointerVerifier.
type zkPointerAdapter struct {
	gate *services.ZKPointerGate
}

func newZKPointerAdapter(gate *services.ZKPointerGate) handlers.ZKPointerVerifier {
	return &zkPointerAdapter{gate: gate}
}

func (a *zkPointerAdapter) GetZKPointerRecord(ctx context.Context, bankID, commitmentHash string) (*handlers.ZKPointerLookup, error) {
	rec, err := a.gate.GetZKPointerRecord(ctx, bankID, commitmentHash)
	if err != nil {
		if errors.Is(err, services.ErrZKPointerGateNotFound) {
			return nil, handlers.ErrZKPointerNotFound
		}
		return nil, err
	}
	return &handlers.ZKPointerLookup{
		PointerID:      rec.PointerID,
		CommitmentHash: rec.CommitmentHash,
		State:          rec.State,
		ExpiresAt:      rec.ExpiresAt,
	}, nil
}
