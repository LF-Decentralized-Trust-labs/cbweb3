// Package app — on-chain counterpart commit source for cooperative liquidity discovery.
// Reads the Hub LiquidityCommitRegistry (the only cross-CB source of truth) so a CB
// gateway can surface a counterpart central bank's open PENDING commit on the opposite
// side of its pool, enabling independent coordination without out-of-band signalling.
package app

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/ethereum/go-ethereum/common"
)

// fxRateReader reads an FX rate from the Hub ManualOracle. getRate(token0, token1)
// returns "token1 per token0" scaled to `decimals`. Satisfied by *ManualOracleClient.
type fxRateReader interface {
	GetRate(ctx context.Context, token0, token1 common.Address) (*big.Int, uint8, error)
}

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

	// FX suggestion inputs (optional). When rates is nil, no suggested_match_amount
	// is computed and the field is left empty.
	rates            fxRateReader
	ownToken         common.Address // this gateway's own W-token (the side it deposits)
	counterpartToken common.Address // the counterpart side's W-token
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

// withFXSuggestion enables suggested_match_amount computation from the ManualOracle.
// ownToken is this gateway's W-token (the side it deposits); counterpartToken is the
// opposite side's W-token. No-op-friendly: pass a nil reader to disable.
func (s *onChainCounterpartSource) withFXSuggestion(rates fxRateReader, ownToken, counterpartToken common.Address) *onChainCounterpartSource {
	s.rates = rates
	s.ownToken = ownToken
	s.counterpartToken = counterpartToken
	return s
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
		Side:                 sideLabel(oppositeSide),
		SignerAddress:        commit.Signer.Hex(),
		Amount:               amount,
		SuggestedMatchAmount: s.suggestMatchAmount(ctx, commit.Amount),
		ExpiresAt:            time.Unix(int64(commit.ExpiresAt), 0).UTC(),
		OnChainCommitID:      CommitIDToHex(id),
	}, nil
}

// suggestMatchAmount returns the amount this gateway should deposit on its own side to
// match a counterpart deposit of counterpartAmount, at the current FX rate. Since
// getRate(token0, token1) = "token1 per token0", the counterpart holds counterpartToken
// and we want ownToken, so:  own = counterpartAmount * getRate(counterpartToken, ownToken) / 10^decimals.
// Returns "" (no suggestion) when the oracle is unconfigured, the rate is unset, or inputs
// are missing — the UI falls back to free entry. Best-effort: never fails the status read.
func (s *onChainCounterpartSource) suggestMatchAmount(ctx context.Context, counterpartAmount *big.Int) string {
	if s.rates == nil || counterpartAmount == nil || counterpartAmount.Sign() <= 0 {
		return ""
	}
	if (s.ownToken == common.Address{}) || (s.counterpartToken == common.Address{}) {
		return ""
	}
	rate, decimals, err := s.rates.GetRate(ctx, s.counterpartToken, s.ownToken)
	if err != nil {
		// RateNotSet / unreachable oracle — degrade gracefully to no suggestion.
		log.Printf("[counterpart] FX suggestion unavailable (getRate): %v", err)
		return ""
	}
	if rate == nil || rate.Sign() <= 0 {
		return ""
	}
	// own = counterpartAmount * rate / 10^decimals
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	out := new(big.Int).Mul(counterpartAmount, rate)
	out.Div(out, scale)
	return out.String()
}

// sideLabel maps the on-chain CommitSide enum (0=A, 1=B) to its string label.
func sideLabel(side uint8) string {
	if side == 1 {
		return "B"
	}
	return "A"
}
