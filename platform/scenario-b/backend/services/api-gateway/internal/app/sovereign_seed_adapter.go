// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"math/big"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/ethereum/go-ethereum/common"
)

// sovereignSeedAdapter adapts the per-pair escrow methods of ammAdapter to the
// handler's SovereignSeedEscrow interface (string in/out, handler-facing status type).
type sovereignSeedAdapter struct {
	a *ammAdapter
}

func (s *sovereignSeedAdapter) DepositSideForCommit(ctx context.Context, poolPair, amount string) (string, error) {
	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount %q", amount)
	}
	return s.a.DepositSideForCommit(ctx, poolPair, amt)
}

func (s *sovereignSeedAdapter) FinalizeCommitForPair(ctx context.Context, poolPair string) (string, string, error) {
	res, err := s.a.FinalizeCommitForPair(ctx, poolPair)
	if err != nil {
		return "", "", err
	}
	sharesA, sharesB := "0", "0"
	if res.SharesA != nil {
		sharesA = res.SharesA.String()
	}
	if res.SharesB != nil {
		sharesB = res.SharesB.String()
	}
	return sharesA, sharesB, nil
}

func (s *sovereignSeedAdapter) CancelSideForCommit(ctx context.Context, poolPair string) (string, error) {
	return s.a.CancelSideForCommit(ctx, poolPair)
}

func (s *sovereignSeedAdapter) GetCommitEscrow(ctx context.Context, poolPair string) (*handlers.EscrowStatus, error) {
	e, err := s.a.GetCommitEscrow(ctx, poolPair)
	if err != nil {
		return nil, err
	}
	amtA, amtB := "0", "0"
	if e.AmountA != nil {
		amtA = e.AmountA.String()
	}
	if e.AmountB != nil {
		amtB = e.AmountB.String()
	}
	return &handlers.EscrowStatus{
		SideADeposited: e.DepositorA != (common.Address{}),
		SideBDeposited: e.DepositorB != (common.Address{}),
		AmountA:        amtA,
		AmountB:        amtB,
		Finalized:      e.Finalized,
	}, nil
}
