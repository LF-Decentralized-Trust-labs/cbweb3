// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"gorm.io/gorm"
)

// The reconciliation subtracts records from an on-chain balance, so the records side has to select
// exactly the right rows. Getting this wrong does not produce an error — it produces a wrong
// number that looks authoritative. Every case below is a class of row that LOOKS like held money
// and is not.

const (
	wToken      = "0xW0000000000000000000000000000000000000A"
	otherWToken = "0xW0000000000000000000000000000000000000B"
	// cbHolder is the Hub address being reconciled; cbSelf is the CB's own owner_bank_id.
	cbHolder = "0xHUBHOLDER0000000000000000000000000000AA"
	cbSelf   = "central-bank-brl"
)

type posOpt func(*domain.BridgedAssetPosition)

func mintTo(addr string) posOpt {
	return func(p *domain.BridgedAssetPosition) { p.MintToHubAddress = addr }
}

func outbound() posOpt {
	return func(p *domain.BridgedAssetPosition) { p.Direction = domain.BridgeDirectionOut }
}

func bridgeIn(db *gorm.DB, t *testing.T, id, bank, token, amount string, state domain.BridgeState, opts ...posOpt) {
	t.Helper()
	pos := &domain.BridgedAssetPosition{
		PositionID: id, OwnerBankID: bank, SpokeNetwork: "spoke-brl",
		NativeAsset: "0xNATIVE", MirroredAsset: token, MirroredAmount: amount,
		BridgeState: state, Leg: domain.BridgeLegSettlement,
		Direction: domain.BridgeDirectionIn,
	}
	for _, o := range opts {
		o(pos)
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed bridge-in %s: %v", id, err)
	}
}

func residueLeg(db *gorm.DB, t *testing.T, id, parent, bank, token, amount, burnTx string, state domain.BridgeState) {
	t.Helper()
	pos := &domain.BridgedAssetPosition{
		PositionID: id, OwnerBankID: bank, SpokeNetwork: "spoke-brl",
		NativeAsset: "0xNATIVE", MirroredAsset: token, MirroredAmount: amount,
		BridgeState: state, Leg: domain.BridgeLegResidue,
		Direction:        domain.BridgeDirectionOut,
		ParentPositionID: parent, HubBurnTxHash: burnTx,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed residue %s: %v", id, err)
	}
}

func hubSwap(db *gorm.DB, t *testing.T, positionID, amountIn string) {
	t.Helper()
	rec := &domain.CrossCurrencyHubSwap{
		BridgeInPositionID: positionID, CorrelationID: "corr-" + positionID,
		PayerBankID: "bank-a", PoolPair: "W-BRL-W-ARS",
		AmountOut: "1", AmountIn: amountIn, SwapTxHash: "0x" + positionID,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed hub swap for %s: %v", positionID, err)
	}
}

func reconRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	return repoDB(t, &domain.BridgedAssetPosition{}, &domain.CrossCurrencyHubSwap{})
}

func reconRepo(db *gorm.DB) *hubReconciliationRepository {
	return newHubReconciliationRepository(db, cbHolder, cbSelf)
}

func positionIDs(got []services.BridgeInContribution) []string {
	ids := make([]string, 0, len(got))
	for i := range got {
		ids = append(ids, got[i].PositionID)
	}
	return ids
}

func TestInFlightBridgeIns_SelectsOnlyActiveSettlementsOfThisToken(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p-active", "bank-a", wToken, "1000", domain.BridgeStateActive)
	// Not yet minted: nothing of this is on the Hub, so counting it would invent an expectation.
	bridgeIn(db, t, "p-locking", "bank-a", wToken, "500", domain.BridgeStateLocking)
	// Another CB's currency — this CB cannot reconcile it and must not mix it in.
	bridgeIn(db, t, "p-other-token", "bank-a", otherWToken, "700", domain.BridgeStateActive)
	// A residue leg is not a bridge-in; counting it would double the same money.
	residueLeg(db, t, "r-standalone", "p-active", "bank-a", wToken, "200", "", domain.BridgeStateActive)

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	if len(got) != 1 || got[0].PositionID != "p-active" {
		t.Fatalf("selected %v, want only p-active", positionIDs(got))
	}
	if got[0].Minted != "1000" || (got[0].Consumed != "" && got[0].Consumed != "0") {
		t.Fatalf("unexpected contribution: %+v", got[0])
	}
}

// A bridge-OUT is created with leg=SETTLEMENT, reaches ACTIVE and carries this CB's own W-token —
// every other predicate matches it. Its tokens are held at the SOURCE CB's address, so counting it
// inflates the expectation and hides a real shortfall behind it.
func TestInFlightBridgeIns_ExcludesBridgeOuts(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p-in", "bank-a", wToken, "1000", domain.BridgeStateActive)
	bridgeIn(db, t, "p-out", "bank-b", wToken, "4000", domain.BridgeStateActive, outbound())

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	if len(got) != 1 || got[0].PositionID != "p-in" {
		t.Fatalf("selected %v, want only p-in — an inbound bridge-out was counted as held", positionIDs(got))
	}
}

// The CB's own lock-mints are its own liquidity, typically deployed into a pool. They are not an
// obligation toward a bank, and nothing records where they went, so treating them as in-flight
// would report an expectation that never resolves.
func TestInFlightBridgeIns_ExcludesTheCBsOwnPositions(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p-bank", "bank-a", wToken, "1000", domain.BridgeStateActive)
	bridgeIn(db, t, "p-own", cbSelf, wToken, "9000", domain.BridgeStateActive)

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	if len(got) != 1 || got[0].PositionID != "p-bank" {
		t.Fatalf("selected %v, want only p-bank", positionIDs(got))
	}
}

// mint_to_hub_address is where the W-token actually landed. Empty means the executor default,
// which is this holder; a different address means another gateway holds it.
func TestInFlightBridgeIns_OnlyCountsWhatLandedOnThisAddress(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p-default", "bank-a", wToken, "100", domain.BridgeStateActive)
	bridgeIn(db, t, "p-explicit", "bank-a", wToken, "200", domain.BridgeStateActive, mintTo(cbHolder))
	// Case must not decide it: the same address is written checksummed in one place and lowercase
	// in another depending on which client produced it.
	bridgeIn(db, t, "p-lowercase", "bank-a", wToken, "300", domain.BridgeStateActive,
		mintTo("0xhubholder0000000000000000000000000000aa"))
	bridgeIn(db, t, "p-elsewhere", "bank-a", wToken, "400", domain.BridgeStateActive,
		mintTo("0xSOMEOTHERGATEWAY000000000000000000000BB"))

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	ids := positionIDs(got)
	if len(ids) != 3 {
		t.Fatalf("selected %v, want the three that landed here", ids)
	}
	for _, id := range ids {
		if id == "p-elsewhere" {
			t.Fatalf("counted p-elsewhere: W-token minted to another gateway is not on this address")
		}
	}
}

// The realized cost comes from this CB's own record of the trade it executed.
func TestInFlightBridgeIns_ReportsTheRealizedCost(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p1", "bank-a", wToken, "3000", domain.BridgeStateActive)
	hubSwap(db, t, "p1", "1001")

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	if len(got) != 1 || got[0].Consumed != "1001" {
		t.Fatalf("expected consumed=1001, got %+v", got)
	}
}

// A residue only stops occupying the Hub address once its burn is CONFIRMED. The state is the
// test, not the tx hash: the hash is written as a pre-confirmation intent, so reading it would
// report money as gone while it is still sitting there.
func TestInFlightBridgeIns_CountsOnlyConfirmedResidues(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p1", "bank-a", wToken, "4000", domain.BridgeStateActive)
	hubSwap(db, t, "p1", "1000")
	residueLeg(db, t, "r-burned", "p1", "bank-a", wToken, "1200", "0xburn", domain.BridgeStateBurned)
	residueLeg(db, t, "r-released", "p1", "bank-a", wToken, "300", "0xburn2", domain.BridgeStateReleased)
	// Broadcast but not mined: the hash is already there and the value is not gone.
	residueLeg(db, t, "r-broadcast", "p1", "bank-a", wToken, "800", "0xpending", domain.BridgeStateBurning)

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one bridge-in, got %v", positionIDs(got))
	}
	if got[0].Returned != "1500" {
		t.Fatalf("returned = %s, want 1500 (the confirmed 1200 + 300 only)", got[0].Returned)
	}
}

// A residue burned against ANOTHER bridge-in must not reduce this one's outstanding amount.
func TestInFlightBridgeIns_DoesNotCreditAnotherPositionsResidue(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p1", "bank-a", wToken, "1000", domain.BridgeStateActive)
	bridgeIn(db, t, "p2", "bank-b", wToken, "1000", domain.BridgeStateActive)
	residueLeg(db, t, "r-of-p2", "p2", "bank-b", wToken, "400", "0xb", domain.BridgeStateReleased)

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	byID := map[string]string{}
	for i := range got {
		byID[got[i].PositionID] = got[i].Returned
	}
	if byID["p1"] != "0" {
		t.Fatalf("p1 returned = %s, want 0 — another position's residue was credited to it", byID["p1"])
	}
	if byID["p2"] != "400" {
		t.Fatalf("p2 returned = %s, want 400", byID["p2"])
	}
}

func TestStrandedPositions_ListsOnlyReconciliationRequiredOfThisToken(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p-active", "bank-a", wToken, "1000", domain.BridgeStateActive)
	bridgeIn(db, t, "p-stuck", "bank-a", wToken, "900", domain.BridgeStateReconciliationRequired)
	residueLeg(db, t, "r-stuck", "p-active", "bank-b", wToken, "250", "", domain.BridgeStateReconciliationRequired)
	bridgeIn(db, t, "p-other", "bank-a", otherWToken, "800", domain.BridgeStateReconciliationRequired)

	got, err := r.StrandedPositions(context.Background(), wToken)
	if err != nil {
		t.Fatalf("stranded: %v", err)
	}
	if len(got) != 2 {
		ids := make([]string, 0, len(got))
		for i := range got {
			ids = append(ids, got[i].PositionID)
		}
		t.Fatalf("selected %v, want p-stuck and r-stuck", ids)
	}
	byID := map[string]string{}
	dirByID := map[string]string{}
	for i := range got {
		byID[got[i].PositionID] = got[i].Amount
		dirByID[got[i].PositionID] = got[i].Direction
	}
	if byID["p-stuck"] != "900" || byID["r-stuck"] != "250" {
		t.Fatalf("unexpected amounts: %+v", byID)
	}
	// Direction is what tells an operator whether the value is on the address (a stuck burn) or
	// never reached it (a stuck mint).
	if dirByID["p-stuck"] != string(domain.BridgeDirectionIn) || dirByID["r-stuck"] != string(domain.BridgeDirectionOut) {
		t.Fatalf("direction not carried through: %+v", dirByID)
	}
}

// A stranded residue leaves its PARENT still accounting for the same amount — the parent's
// returned total does not include it. This is why the service reports stranded items instead of
// subtracting them: doing both would count the money twice.
func TestInFlightBridgeIns_StrandedResidueStaysInTheParentsExpectation(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	bridgeIn(db, t, "p1", "bank-a", wToken, "1000", domain.BridgeStateActive)
	hubSwap(db, t, "p1", "750")
	residueLeg(db, t, "r-given-up", "p1", "bank-a", wToken, "250", "", domain.BridgeStateReconciliationRequired)

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one bridge-in, got %v", positionIDs(got))
	}
	if got[0].Returned != "0" {
		t.Fatalf("returned = %s, want 0 — a given-up residue never left the Hub address", got[0].Returned)
	}
	// 1000 minted − 750 consumed − 0 returned = the 250 still sitting there, which the stranded
	// list names separately.
	stranded, err := r.StrandedPositions(context.Background(), wToken)
	if err != nil {
		t.Fatalf("stranded: %v", err)
	}
	if len(stranded) != 1 || stranded[0].Amount != "250" {
		t.Fatalf("stranded = %+v, want the same 250", stranded)
	}
}

func TestInFlightBridgeIns_EmptyWhenNothingMatches(t *testing.T) {
	db := reconRepoDB(t)
	r := reconRepo(db)

	got, err := r.InFlightBridgeIns(context.Background(), wToken)
	if err != nil {
		t.Fatalf("in-flight: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected nothing, got %+v", got)
	}
}
