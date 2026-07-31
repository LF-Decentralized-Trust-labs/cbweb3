// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"
)

// The whole feature is one subtraction, so the subtraction is worth testing exhaustively. Amounts
// here are 18-decimal base units, the size the production path actually carries — a reconciliation
// that rounds is not a reconciliation.
const (
	oneT   = "1000000000000000000"   // 1
	twoT   = "2000000000000000000"   // 2
	threeT = "3000000000000000000"   // 3
	tenT   = "10000000000000000000"  // 10
	hugeT  = "123456789012345678901" // > int64, > float64 exact range
)

func inflight(pos, bank, minted, consumed, returned string) BridgeInContribution {
	return BridgeInContribution{
		PositionID: pos, OwnerBankID: bank,
		Minted: minted, Consumed: consumed, Returned: returned,
	}
}

// Every payment settled: the input was consumed and the remainder came back, so the CB should be
// holding nothing at all. This is the healthy resting state the invariant is built on.
func TestReconcile_SettledPaymentsExpectAZeroBalance(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		WToken:         "0xW",
		HolderAddress:  "0xCB",
		OnChainBalance: "0",
		InFlight: []BridgeInContribution{
			inflight("p1", "bank-a", threeT, oneT, twoT), // 3 minted, 1 spent, 2 returned
			inflight("p2", "bank-b", tenT, tenT, "0"),    // consumed the whole cap
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.ExpectedInFlight != "0" || rep.Unexplained != "0" || !rep.Balanced {
		t.Fatalf("expected a balanced zero report, got %+v", rep)
	}
}

// Mid-flight is the case that must NOT alarm: a payment that has been swapped but whose residue
// has not returned legitimately leaves (minted − consumed) sitting on the Hub. This is also the
// transient over-debit window measured live at 5-10s.
func TestReconcile_MidFlightResidueIsExpectedNotUnexplained(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: twoT, // the unreturned residue is still there
		InFlight: []BridgeInContribution{
			inflight("p1", "bank-a", threeT, oneT, "0"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.ExpectedInFlight != twoT {
		t.Fatalf("expected %s in flight, got %s", twoT, rep.ExpectedInFlight)
	}
	if rep.Unexplained != "0" || !rep.Balanced {
		t.Fatalf("a mid-flight residue must not read as unexplained: %+v", rep)
	}
}

// A bridge-in that has not been swapped yet accounts for its whole minted amount.
func TestReconcile_UnswappedBridgeInAccountsForTheWholeMint(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: threeT,
		InFlight:       []BridgeInContribution{inflight("p1", "bank-a", threeT, "", "")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.ExpectedInFlight != threeT || !rep.Balanced {
		t.Fatalf("empty consumed/returned must count as zero: %+v", rep)
	}
}

// The product of the feature: value on-chain that the records cannot attribute. A residue whose
// enqueue failed leaves no position on this CB, so it lands here — which is the point.
func TestReconcile_UnattributableBalanceIsReported(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: twoT, // 2 on chain
		InFlight:       []BridgeInContribution{inflight("p1", "bank-a", oneT, oneT, "0")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.ExpectedInFlight != "0" {
		t.Fatalf("expected nothing in flight, got %s", rep.ExpectedInFlight)
	}
	if rep.Unexplained != twoT {
		t.Fatalf("unexplained = %s, want %s", rep.Unexplained, twoT)
	}
	if rep.Balanced {
		t.Fatalf("a non-zero unexplained amount must not report as balanced")
	}
}

// A position the relayer gave up on is money this CB can NAME, so it is reported alongside the
// figure — but it is not subtracted from it. The stranded set overlaps the in-flight expectation
// (see below), and netting both would count the same money twice.
func TestReconcile_StrandedPositionsAreReportedNotSubtracted(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: threeT,
		InFlight:       []BridgeInContribution{inflight("p1", "bank-a", oneT, oneT, "0")},
		Stranded: []StrandedItem{
			{PositionID: "r1", OwnerBankID: "bank-a", Leg: "RESIDUE", Direction: "OUT", Amount: twoT, BridgeState: "RECONCILIATION_REQUIRED"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.StrandedTotal != twoT {
		t.Fatalf("stranded total = %s, want %s", rep.StrandedTotal, twoT)
	}
	if rep.Unexplained != threeT {
		t.Fatalf("unexplained = %s, want %s (3 on chain − 0 in flight; stranded is context, not a deduction)", rep.Unexplained, threeT)
	}
	if len(rep.Stranded) != 1 || rep.Stranded[0].PositionID != "r1" {
		t.Fatalf("the named item must be carried into the report: %+v", rep.Stranded)
	}
}

// The case that made the earlier subtraction wrong, and the reason for the one above. A residue the
// relayer gave up on is still on the Hub AND still inside its parent's outstanding amount — the
// parent's returned total does not include it. Subtracting the stranded item too produced a
// negative figure precisely in the failure the report exists to name.
func TestReconcile_StrandedResidueOverlapsItsParentAndDoesNotGoNegative(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: twoT, // the unreturned residue, still sitting there
		InFlight: []BridgeInContribution{
			inflight("p1", "bank-a", threeT, oneT, "0"), // still accounts for the same 2
		},
		Stranded: []StrandedItem{
			{PositionID: "r1", OwnerBankID: "bank-a", Leg: "RESIDUE", Direction: "OUT", Amount: twoT, BridgeState: "RECONCILIATION_REQUIRED"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Unexplained != "0" || !rep.Balanced {
		t.Fatalf("the balance is fully explained by the parent; got unexplained=%s (a subtraction of the stranded leg would read -%s)", rep.Unexplained, twoT)
	}
	if rep.StrandedTotal != twoT {
		t.Fatalf("the stuck leg must still be reported for the operator: %+v", rep)
	}
}

// Per-bank aggregation is what makes the figure reportable: an omnibus balance has to be
// attributable to the banks behind it.
func TestReconcile_AggregatesExposurePerBank(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: "6000000000000000000",
		InFlight: []BridgeInContribution{
			inflight("p1", "bank-a", twoT, "0", "0"),
			inflight("p2", "bank-a", oneT, "0", "0"),
			inflight("p3", "bank-b", threeT, "0", "0"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.PerBank) != 2 {
		t.Fatalf("expected two banks, got %+v", rep.PerBank)
	}
	byBank := map[string]BankExposure{}
	for _, e := range rep.PerBank {
		byBank[e.OwnerBankID] = e
	}
	if byBank["bank-a"].Amount != threeT || byBank["bank-a"].Positions != 2 {
		t.Fatalf("bank-a = %+v, want 3 across 2 positions", byBank["bank-a"])
	}
	if byBank["bank-b"].Amount != threeT || byBank["bank-b"].Positions != 1 {
		t.Fatalf("bank-b = %+v, want 3 across 1 position", byBank["bank-b"])
	}
	if !rep.Balanced {
		t.Fatalf("expected balanced, got unexplained=%s", rep.Unexplained)
	}
}

// A bridge-in stays ACTIVE for life, so a bank whose payments have all settled would otherwise be
// listed forever at zero. Observed live: a CB with two settled payments reported
// `per_bank: [{bank-itau, amount 0, positions 2}]`, a list that grows without bound and carries no
// information. Only real exposure is reportable.
func TestReconcile_DoesNotListBanksItOwesNothing(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: twoT,
		InFlight: []BridgeInContribution{
			inflight("p-settled", "bank-a", threeT, oneT, twoT), // fully accounted for
			inflight("p-open", "bank-b", threeT, oneT, "0"),     // still owes 2
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.PerBank) != 1 || rep.PerBank[0].OwnerBankID != "bank-b" {
		t.Fatalf("per_bank = %+v, want only bank-b — a settled bank must not be listed at zero", rep.PerBank)
	}
	// The filter is presentational: the subtraction itself must be untouched.
	if rep.ExpectedInFlight != twoT || !rep.Balanced {
		t.Fatalf("the arithmetic changed: %+v", rep)
	}
}

// A balance SHORT of what the records expect is as much a finding as an excess, and it is the
// worse one: value that should be there is not.
func TestReconcile_ShortBalanceReportsNegativeUnexplained(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: oneT,
		InFlight:       []BridgeInContribution{inflight("p1", "bank-a", threeT, "0", "0")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Unexplained != "-2000000000000000000" {
		t.Fatalf("unexplained = %s, want -2000000000000000000", rep.Unexplained)
	}
	if rep.Balanced {
		t.Fatalf("a shortfall must not report as balanced")
	}
}

// 18-decimal amounts exceed int64 and float64's exact range, so the arithmetic must be exact.
func TestReconcile_ExactAtLargeMagnitudes(t *testing.T) {
	rep, err := Reconcile(ReconciliationInputs{
		OnChainBalance: hugeT,
		InFlight:       []BridgeInContribution{inflight("p1", "bank-a", hugeT, "0", "0")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rep.Balanced || rep.Unexplained != "0" {
		t.Fatalf("large magnitudes must reconcile exactly, got %+v", rep)
	}

	// One base unit off must be visible, not rounded away.
	off, err := Reconcile(ReconciliationInputs{
		OnChainBalance: "123456789012345678902",
		InFlight:       []BridgeInContribution{inflight("p1", "bank-a", hugeT, "0", "0")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if off.Unexplained != "1" {
		t.Fatalf("a one-unit discrepancy must survive, got %q", off.Unexplained)
	}
}

// A position claiming more went out than ever came in is a contradiction in the records. Letting
// it through would let one position's impossible surplus mask another's real shortfall.
func TestReconcile_RefusesContradictoryPositions(t *testing.T) {
	_, err := Reconcile(ReconciliationInputs{
		OnChainBalance: "0",
		InFlight:       []BridgeInContribution{inflight("p1", "bank-a", oneT, twoT, "0")},
	})
	if err == nil {
		t.Fatalf("expected a negative contribution to be refused")
	}
}

func TestReconcile_RefusesUnparseableAmounts(t *testing.T) {
	cases := map[string]ReconciliationInputs{
		"balance":  {OnChainBalance: "not-a-number"},
		"minted":   {OnChainBalance: "0", InFlight: []BridgeInContribution{inflight("p1", "b", "x", "0", "0")}},
		"consumed": {OnChainBalance: "0", InFlight: []BridgeInContribution{inflight("p1", "b", oneT, "x", "0")}},
		"returned": {OnChainBalance: "0", InFlight: []BridgeInContribution{inflight("p1", "b", oneT, "0", "x")}},
		"stranded": {OnChainBalance: "0", Stranded: []StrandedItem{{PositionID: "r1", Amount: "x"}}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Reconcile(in); err == nil {
				t.Fatalf("expected an unparseable %s to be refused rather than treated as zero", name)
			}
		})
	}
}

// --- service wiring ---

type fakeBalances struct {
	balance string
	err     error
	token   string
	holder  string
}

func (f *fakeBalances) BalanceOf(_ context.Context, token, holder string) (string, error) {
	f.token, f.holder = token, holder
	return f.balance, f.err
}

type fakeReconRepo struct {
	inFlight    []BridgeInContribution
	stranded    []StrandedItem
	inFlightErr error
	strandedErr error
	gotToken    string
}

func (f *fakeReconRepo) InFlightBridgeIns(_ context.Context, wToken string) ([]BridgeInContribution, error) {
	f.gotToken = wToken
	return f.inFlight, f.inFlightErr
}

func (f *fakeReconRepo) StrandedPositions(context.Context, string) ([]StrandedItem, error) {
	return f.stranded, f.strandedErr
}

func TestHubReconciliationService_ReadsItsOwnTokenAndAddress(t *testing.T) {
	bal := &fakeBalances{balance: twoT}
	repo := &fakeReconRepo{inFlight: []BridgeInContribution{inflight("p1", "bank-a", threeT, oneT, "0")}}
	svc := NewHubReconciliationService(bal, repo, "0xWTOKEN", "0xCBHUB")
	if svc == nil {
		t.Fatalf("service should be wired")
	}

	rep, err := svc.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bal.token != "0xWTOKEN" || bal.holder != "0xCBHUB" {
		t.Fatalf("read the wrong balance: token=%s holder=%s", bal.token, bal.holder)
	}
	if repo.gotToken != "0xWTOKEN" {
		t.Fatalf("records queried for the wrong token: %s", repo.gotToken)
	}
	if !rep.Balanced {
		t.Fatalf("expected balanced, got %+v", rep)
	}
}

// A gateway that is not an issuing CB has nothing to reconcile, and must not pretend a balanced
// report — that would be a green light nobody computed.
func TestNewHubReconciliationService_NilWithoutItsOwnTokenOrAddress(t *testing.T) {
	bal, repo := &fakeBalances{}, &fakeReconRepo{}
	for name, svc := range map[string]*HubReconciliationService{
		"no token":    NewHubReconciliationService(bal, repo, "", "0xCB"),
		"no holder":   NewHubReconciliationService(bal, repo, "0xW", ""),
		"no balances": NewHubReconciliationService(nil, repo, "0xW", "0xCB"),
		"no repo":     NewHubReconciliationService(bal, nil, "0xW", "0xCB"),
	} {
		if svc != nil {
			t.Fatalf("%s: expected no service", name)
		}
	}
}

// A read failure must surface, never degrade into a zero that reads as balanced.
func TestHubReconciliationService_SurfacesReadFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		bal  *fakeBalances
		repo *fakeReconRepo
	}{
		"chain":     {&fakeBalances{err: errors.New("rpc down")}, &fakeReconRepo{}},
		"in-flight": {&fakeBalances{balance: "0"}, &fakeReconRepo{inFlightErr: errors.New("db down")}},
		"stranded":  {&fakeBalances{balance: "0"}, &fakeReconRepo{strandedErr: errors.New("db down")}},
	} {
		t.Run(name, func(t *testing.T) {
			svc := NewHubReconciliationService(tc.bal, tc.repo, "0xW", "0xCB")
			if _, err := svc.Reconcile(context.Background()); err == nil {
				t.Fatalf("expected the %s read failure to surface", name)
			}
		})
	}
}
