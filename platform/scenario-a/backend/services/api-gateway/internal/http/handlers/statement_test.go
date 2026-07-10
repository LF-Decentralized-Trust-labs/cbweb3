// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/gofiber/fiber/v2"
)

type fakeMovementSource struct {
	deposits []paymentadapter.DepositRecord
	escrows  []paymentadapter.EscrowRecord
	redeems  []paymentadapter.RedeemRecord
	err      error
}

func (f *fakeMovementSource) FetchDeposits(context.Context) ([]paymentadapter.DepositRecord, error) {
	return f.deposits, f.err
}
func (f *fakeMovementSource) FetchEscrows(context.Context) ([]paymentadapter.EscrowRecord, error) {
	return f.escrows, f.err
}
func (f *fakeMovementSource) FetchRedeems(context.Context) ([]paymentadapter.RedeemRecord, error) {
	return f.redeems, f.err
}

func doStatement(t *testing.T, src MovementSource) []Movement {
	t.Helper()
	app := fiber.New()
	app.Get("/api/v1/statement", NewStatementHandler(src).GetStatement)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/statement", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Movements []Movement `json:"movements"`
		Total     int        `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != len(body.Movements) {
		t.Errorf("total = %d, len(movements) = %d", body.Total, len(body.Movements))
	}
	return body.Movements
}

func TestStatement_ConsolidatesAndClassifies(t *testing.T) {
	src := &fakeMovementSource{
		deposits: []paymentadapter.DepositRecord{
			{ID: "d1", Amount: "1000", Status: "DEPOSIT_STATUS_APPROVED", MintTxHash: "0xmint", CreatedAt: "2026-07-10T10:00:00Z"},
			{ID: "d2", Amount: "500", Status: "DEPOSIT_STATUS_PENDING", CreatedAt: "2026-07-10T11:00:00Z"}, // excluded
		},
		escrows: []paymentadapter.EscrowRecord{
			{ID: "e1", Amount: "300", Status: "ESCROW_STATUS_APPROVED", BurnTxHash: "0xburn", MintTxHash: "0xtmint", CreatedAt: "2026-07-10T12:00:00Z"},
		},
		redeems: []paymentadapter.RedeemRecord{
			{ID: "r1", Amount: "200", Status: "REDEEM_STATUS_APPROVED", ZetoTransferTxHash: "0xzeto", FiatMintTxHash: "0xfmint", CreatedAt: "2026-07-10T09:00:00Z"},
		},
	}

	movements := doStatement(t, src)

	// 1 deposit + 2 escrow legs + 2 redeem legs = 5 (pending deposit excluded).
	if len(movements) != 5 {
		t.Fatalf("expected 5 movements, got %d: %+v", len(movements), movements)
	}

	// Most-recent-first ordering: escrow (12:00) legs, then deposit (10:00), then redeem (09:00) legs.
	if movements[0].Timestamp != "2026-07-10T12:00:00Z" {
		t.Errorf("first movement ts = %q, want escrow 12:00", movements[0].Timestamp)
	}
	if last := movements[len(movements)-1].Timestamp; last != "2026-07-10T09:00:00Z" {
		t.Errorf("last movement ts = %q, want redeem 09:00", last)
	}

	// Verify each leg's direction/token by kind.
	var sawDepositCredit, sawTokDebitFiat, sawTokCreditTcebm, sawRedeemDebitTcebm, sawRedeemCreditFiat bool
	for _, m := range movements {
		switch {
		case m.Kind == "deposit":
			sawDepositCredit = m.Direction == directionCredit && m.Token == tokenFiat && m.Amount == "1000"
		case m.Kind == "tokenisation" && m.Token == tokenFiat:
			sawTokDebitFiat = m.Direction == directionDebit
		case m.Kind == "tokenisation" && m.Token == tokenTCeBM:
			sawTokCreditTcebm = m.Direction == directionCredit
		case m.Kind == "redeem" && m.Token == tokenTCeBM:
			sawRedeemDebitTcebm = m.Direction == directionDebit
		case m.Kind == "redeem" && m.Token == tokenFiat:
			sawRedeemCreditFiat = m.Direction == directionCredit
		}
	}
	if !sawDepositCredit {
		t.Error("deposit should be a credit of fCeBM")
	}
	if !sawTokDebitFiat || !sawTokCreditTcebm {
		t.Error("tokenisation should debit fCeBM and credit tCeBM")
	}
	if !sawRedeemDebitTcebm || !sawRedeemCreditFiat {
		t.Error("redeem should debit tCeBM and credit fCeBM")
	}
}

func TestStatement_EmptyWhenNoSettledRecords(t *testing.T) {
	src := &fakeMovementSource{
		deposits: []paymentadapter.DepositRecord{{ID: "d1", Status: "DEPOSIT_STATUS_REJECTED"}},
	}
	if movements := doStatement(t, src); len(movements) != 0 {
		t.Errorf("expected no movements, got %d", len(movements))
	}
}

func TestStatement_SourceErrorReturns502(t *testing.T) {
	app := fiber.New()
	src := &fakeMovementSource{err: errors.New("central bank down")}
	app.Get("/api/v1/statement", NewStatementHandler(src).GetStatement)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/statement", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}
