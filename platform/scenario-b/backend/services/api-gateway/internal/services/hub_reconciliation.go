// SPDX-License-Identifier: Apache-2.0

// Package services provides the Hub reconciliation for a central bank's own sovereign W-token.
//
// WHAT THIS ANSWERS. A CB holds its banks' bridged money in ONE Hub address — an omnibus
// account. The banks themselves never hold W-token: it is minted to the CB, spent by the CB in
// the AMM trade, and burned by the CB when the unspent part goes back. So the balance on that
// address is not an asset of anyone's; it is the CB's OBLIGATION toward banks whose reserves were
// consumed and whose payment has not fully closed.
//
// Because every completed payment consumes its input and returns the remainder, the payment side
// of that balance is ZERO at rest. That makes the invariant exact rather than a heuristic:
//
//	on-chain balance − Σ(what each in-flight payment still legitimately has there) = 0
//
// WHAT IS DELIBERATELY OUT OF SCOPE, and why it is not a defect. The same address also holds the
// CB's OWN W-token — liquidity it minted for itself and deployed into an AMM pool. Withdrawing
// from a pool returns that W-token to the address and nothing burns it back down (no such path
// exists today), so it rests there indefinitely and is not an obligation toward any bank. Those
// positions carry the CB's own owner_bank_id and are excluded from the expectation. They therefore
// show up in Unexplained, which is honest: the figure means "not attributable to a bank payment",
// and a CB reading its own report knows its own liquidity. Netting it off would require a
// per-position record of pool deployments that does not exist, and inventing one from balances is
// the heuristic this feature replaces.
//
// A payment's contribution depends on its stage, which is the derivation the residue decision
// record already defined (net_consumed = bridged − Σ residue legs):
//
//	minted, not yet swapped        -> the whole mirrored_amount
//	swapped, residue not yet back  -> mirrored_amount − amount_in
//	residue returned               -> zero
//
// Anything left over is reported as UNEXPLAINED. That figure is the product: not "money is
// missing" — the value is on-chain and visible — but "this CB cannot say which payment it belongs
// to", which for an omnibus account is the reportable condition.
//
// SCOPE. This CB's own W-token only, where its records are complete: it minted it, it swapped it,
// it burns it. A foreign W-token received as swap output is burned by the OTHER CB on bridge-out,
// an act this CB does not record, so it cannot be reconciled from here and is deliberately left
// out rather than guessed at.
//
// A residue whose enqueue failed leaves NO position on this CB, so it cannot appear as a
// candidate — it surfaces as unexplained, which is exactly right: the CB sees an amount it cannot
// account for, and the payer's own gateway knows which swap it belongs to (RETURN_ESCALATED).
package services

import (
	"context"
	"fmt"
	"math/big"
)

// BridgeInContribution is one ACTIVE bridge-in and what it still accounts for on the Hub.
type BridgeInContribution struct {
	PositionID  string
	OwnerBankID string
	// Minted is what this CB put on the Hub for the payment; Consumed is the realized amount_in
	// of the AMM trade it funded; Returned is the part already burned back to the payer.
	Minted   string
	Consumed string
	Returned string
}

// StrandedItem is a position this CB's own records already flag as needing attention.
//
// Reported, never SUBTRACTED. Most of these are already inside the in-flight figure — a residue
// leg whose burn never confirmed leaves its parent still accounting for the same amount — so
// subtracting them too would drive the result negative in exactly the failure the report exists to
// name. Direction distinguishes a stuck mint (nothing reached the address) from a stuck burn
// (still sitting on it).
type StrandedItem struct {
	PositionID  string `json:"position_id"`
	OwnerBankID string `json:"owner_bank_id"`
	Leg         string `json:"leg"`
	Direction   string `json:"direction"`
	Amount      string `json:"amount"`
	BridgeState string `json:"bridge_state"`
}

// ReconciliationInputs is everything the arithmetic needs, so the computation itself stays pure.
type ReconciliationInputs struct {
	WToken         string
	HolderAddress  string
	OnChainBalance string
	InFlight       []BridgeInContribution
	Stranded       []StrandedItem
}

// BankExposure is what this CB still owes a single bank, aggregated across its in-flight payments.
type BankExposure struct {
	OwnerBankID string `json:"owner_bank_id"`
	Amount      string `json:"amount"`
	Positions   int    `json:"positions"`
}

// ReconciliationReport is the answer: what is on-chain, what the records account for, and what
// they do not.
type ReconciliationReport struct {
	WToken         string `json:"w_token"`
	HolderAddress  string `json:"holder_address"`
	OnChainBalance string `json:"on_chain_balance"`
	// ExpectedInFlight is the sum of what in-flight payments legitimately still have on the Hub.
	ExpectedInFlight string `json:"expected_in_flight"`
	// Unexplained is on-chain minus expected. Zero is the healthy state; anything else is a
	// reportable condition, not necessarily a loss. Stranded items are NOT subtracted: they
	// overlap the in-flight figure, so netting them would misreport the very failure they name.
	Unexplained string `json:"unexplained"`
	// StrandedTotal is the value already flagged by this CB's own records, for context. It is not
	// part of the subtraction above and may well be counted inside ExpectedInFlight.
	StrandedTotal string         `json:"stranded_total"`
	PerBank       []BankExposure `json:"per_bank"`
	Stranded      []StrandedItem `json:"stranded,omitempty"`
	// Balanced reports whether Unexplained is exactly zero.
	Balanced bool `json:"balanced"`
}

// Reconcile computes the report. Pure: no clock, no chain, no database — the whole meaning of
// this feature is one subtraction, and it is worth being able to test it exhaustively.
//
// Amounts are base-unit decimal strings and are added with big.Int: at 18 decimals these exceed
// float64 and int64, and a reconciliation that rounds is not a reconciliation.
func Reconcile(in ReconciliationInputs) (ReconciliationReport, error) {
	balance, ok := new(big.Int).SetString(trimOrZero(in.OnChainBalance), 10)
	if !ok {
		return ReconciliationReport{}, fmt.Errorf("unparseable on-chain balance %q", in.OnChainBalance)
	}

	expected := new(big.Int)
	perBank := map[string]*big.Int{}
	perBankCount := map[string]int{}
	order := []string{}

	for _, c := range in.InFlight {
		minted, ok := new(big.Int).SetString(trimOrZero(c.Minted), 10)
		if !ok {
			return ReconciliationReport{}, fmt.Errorf("position %s: unparseable minted amount %q", c.PositionID, c.Minted)
		}
		consumed, ok := new(big.Int).SetString(trimOrZero(c.Consumed), 10)
		if !ok {
			return ReconciliationReport{}, fmt.Errorf("position %s: unparseable consumed amount %q", c.PositionID, c.Consumed)
		}
		returned, ok := new(big.Int).SetString(trimOrZero(c.Returned), 10)
		if !ok {
			return ReconciliationReport{}, fmt.Errorf("position %s: unparseable returned amount %q", c.PositionID, c.Returned)
		}

		still := new(big.Int).Sub(minted, consumed)
		still.Sub(still, returned)
		// A negative contribution means the records claim more left than ever arrived — a
		// bookkeeping contradiction, not a small balance. Surface it instead of letting it
		// quietly offset another position's shortfall.
		if still.Sign() < 0 {
			return ReconciliationReport{}, fmt.Errorf(
				"position %s accounts for a negative amount (minted %s − consumed %s − returned %s) — reconciliation inputs are inconsistent",
				c.PositionID, minted, consumed, returned)
		}

		expected.Add(expected, still)
		if _, seen := perBank[c.OwnerBankID]; !seen {
			perBank[c.OwnerBankID] = new(big.Int)
			order = append(order, c.OwnerBankID)
		}
		perBank[c.OwnerBankID].Add(perBank[c.OwnerBankID], still)
		perBankCount[c.OwnerBankID]++
	}

	strandedTotal := new(big.Int)
	for _, s := range in.Stranded {
		amt, ok := new(big.Int).SetString(trimOrZero(s.Amount), 10)
		if !ok {
			return ReconciliationReport{}, fmt.Errorf("stranded position %s: unparseable amount %q", s.PositionID, s.Amount)
		}
		strandedTotal.Add(strandedTotal, amt)
	}

	// Only the in-flight expectation is netted off. See StrandedItem: the stranded set overlaps
	// this expectation, so subtracting it as well produced a negative figure precisely when a
	// residue burn had been given up on — the case the report is for.
	unexplained := new(big.Int).Sub(balance, expected)

	// Only banks this CB actually still owes something. A bridge-in stays ACTIVE for life and
	// contributes zero once its payment has settled, so without this filter every bank that ever
	// transacted would be listed forever at amount 0 — a list that grows without bound and says
	// nothing. The positions count is kept, since for a bank with a real exposure it says across
	// how many payments that exposure is spread.
	exposures := make([]BankExposure, 0, len(order))
	for _, bank := range order {
		if perBank[bank].Sign() == 0 {
			continue
		}
		exposures = append(exposures, BankExposure{
			OwnerBankID: bank,
			Amount:      perBank[bank].String(),
			Positions:   perBankCount[bank],
		})
	}

	return ReconciliationReport{
		WToken:           in.WToken,
		HolderAddress:    in.HolderAddress,
		OnChainBalance:   balance.String(),
		ExpectedInFlight: expected.String(),
		Unexplained:      unexplained.String(),
		StrandedTotal:    strandedTotal.String(),
		PerBank:          exposures,
		Stranded:         in.Stranded,
		Balanced:         unexplained.Sign() == 0,
	}, nil
}

// trimOrZero treats an empty amount as zero: legacy rows and never-swapped positions leave these
// columns blank, and blank means "nothing happened", not "unknown".
func trimOrZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// HubBalanceReader reads an ERC-20 balance on the Hub chain.
type HubBalanceReader interface {
	BalanceOf(ctx context.Context, tokenAddress, holder string) (string, error)
}

// ReconciliationRepository supplies the records side of the subtraction.
type ReconciliationRepository interface {
	// InFlightBridgeIns returns every ACTIVE settlement bridge-in of this W-token together with
	// what its payment has consumed and already returned.
	InFlightBridgeIns(ctx context.Context, wToken string) ([]BridgeInContribution, error)
	// StrandedPositions returns positions of this W-token that this CB's own records already
	// flag as needing attention (the relayer gave up on them).
	StrandedPositions(ctx context.Context, wToken string) ([]StrandedItem, error)
}

// HubReconciliationService assembles the inputs and runs the arithmetic.
type HubReconciliationService struct {
	balances HubBalanceReader
	repo     ReconciliationRepository
	wToken   string
	holder   string
}

// NewHubReconciliationService wires the service for one CB: its own W-token and its own Hub
// address. Returns nil when either is unknown — a gateway that is not an issuing CB has nothing
// to reconcile.
func NewHubReconciliationService(balances HubBalanceReader, repo ReconciliationRepository, wToken, holder string) *HubReconciliationService {
	if balances == nil || repo == nil || wToken == "" || holder == "" {
		return nil
	}
	return &HubReconciliationService{balances: balances, repo: repo, wToken: wToken, holder: holder}
}

// Reconcile reads the chain and the records and returns the report.
func (s *HubReconciliationService) Reconcile(ctx context.Context) (ReconciliationReport, error) {
	balance, err := s.balances.BalanceOf(ctx, s.wToken, s.holder)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("read Hub balance of %s at %s: %w", s.wToken, s.holder, err)
	}
	inFlight, err := s.repo.InFlightBridgeIns(ctx, s.wToken)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("read in-flight bridge-ins: %w", err)
	}
	stranded, err := s.repo.StrandedPositions(ctx, s.wToken)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("read stranded positions: %w", err)
	}
	return Reconcile(ReconciliationInputs{
		WToken:         s.wToken,
		HolderAddress:  s.holder,
		OnChainBalance: balance,
		InFlight:       inFlight,
		Stranded:       stranded,
	})
}
