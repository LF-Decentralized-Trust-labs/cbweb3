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
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

type fakeHTLCSource struct {
	locks []paymentadapter.HTLCStatus
	err   error
}

func (f *fakeHTLCSource) SearchHTLC(context.Context, string, string, string, string) ([]paymentadapter.HTLCStatus, error) {
	return f.locks, f.err
}

type fakePvPCreditSource struct {
	credits []PvPCredit
	err     error
}

func (f *fakePvPCreditSource) FetchPvPCredits(context.Context, string) ([]PvPCredit, error) {
	return f.credits, f.err
}

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

// The local orchestrator source surfaces only the caller's SENT legs (debits):
// an orchestrator holds only the legs it locked, so the receiver side never lands
// here. Received legs (credits) come from the Central Bank instead — see
// TestStatement_IncludesReceivedPvPCredits.
func TestStatement_IncludesSentPvPLegsAsDebits(t *testing.T) {
	htlc := &fakeHTLCSource{locks: []paymentadapter.HTLCStatus{
		// bank-a is the sender → value sent (debit).
		{ContractID: "c1", Sender: "alice@spoke-a-bank-a", Receiver: "bob@spoke-b-bank-b", State: "HTLC_STATE_SETTLED", Amount: "700", CreatedAt: "2026-07-11T10:00:00Z"},
		// bank-a is the receiver → NOT surfaced by the local source (credits come from the CB).
		{ContractID: "c2", Sender: "carol@spoke-b-bank-c", Receiver: "dave@spoke-a-bank-a", State: "HTLC_STATE_SETTLED", Amount: "300", CreatedAt: "2026-07-11T11:00:00Z"},
		// Not settled → excluded.
		{ContractID: "c3", Sender: "x@spoke-a-bank-a", Receiver: "y@spoke-b-bank-b", State: "HTLC_STATE_LOCKED", Amount: "50", CreatedAt: "2026-07-11T12:00:00Z"},
		// bank-a not the sender → excluded.
		{ContractID: "c4", Sender: "p@spoke-a-bank-x", Receiver: "q@spoke-b-bank-y", State: "HTLC_STATE_SETTLED", Amount: "99", CreatedAt: "2026-07-11T13:00:00Z"},
	}}

	h := NewStatementHandler(&fakeMovementSource{}).WithHTLCSource(htlc, "bank-a")
	movements := doStatementWithClaims(t, h, "bank-a")

	// Only c1 (bank-a as sender) qualifies as a debit.
	if len(movements) != 1 {
		t.Fatalf("expected 1 pvp debit, got %d: %+v", len(movements), movements)
	}
	m := movements[0]
	if m.Kind != kindPvP {
		t.Errorf("unexpected kind %q", m.Kind)
	}
	// An inter-bank PvP settlement moves tokenized reserve value → tCeBM.
	if m.Token != tokenTCeBM {
		t.Errorf("pvp movement token = %q, want %q", m.Token, tokenTCeBM)
	}
	if m.Reference != "c1" || m.Direction != directionDebit || m.Amount != "700" {
		t.Errorf("unexpected debit leg: %+v", m)
	}
}

// Received legs (credits) are sourced from the Central Bank, which derives them
// from the settled FX agreements it aggregates, scoped to this bank.
func TestStatement_IncludesReceivedPvPCredits(t *testing.T) {
	credits := &fakePvPCreditSource{credits: []PvPCredit{
		{Reference: "trade-1", Amount: "700", SettledAt: "2026-07-11T10:05:00Z"},
		{Reference: "trade-2", Amount: "300", SettledAt: "2026-07-11T11:05:00Z"},
	}}

	h := NewStatementHandler(&fakeMovementSource{}).WithPvPCreditSource(credits, "bank-c")
	movements := doStatementWithClaims(t, h, "bank-c")

	if len(movements) != 2 {
		t.Fatalf("expected 2 pvp credits, got %d: %+v", len(movements), movements)
	}
	seen := map[string]Movement{}
	for _, m := range movements {
		if m.Kind != kindPvP {
			t.Errorf("unexpected kind %q", m.Kind)
		}
		if m.Token != tokenTCeBM {
			t.Errorf("credit token = %q, want %q", m.Token, tokenTCeBM)
		}
		if m.Direction != directionCredit {
			t.Errorf("expected credit, got %q for %q", m.Direction, m.Reference)
		}
		seen[m.Reference] = m
	}
	if seen["trade-1"].Amount != "700" || seen["trade-2"].Amount != "300" {
		t.Errorf("unexpected credit amounts: %+v", seen)
	}
}

func TestStatement_PvPDebitSourceErrorReturns502(t *testing.T) {
	h := NewStatementHandler(&fakeMovementSource{}).
		WithHTLCSource(&fakeHTLCSource{err: errors.New("orchestrator down")}, "bank-a")
	assertStatementStatus(t, h, "bank-a", http.StatusBadGateway)
}

func TestStatement_PvPCreditSourceErrorReturns502(t *testing.T) {
	h := NewStatementHandler(&fakeMovementSource{}).
		WithPvPCreditSource(&fakePvPCreditSource{err: errors.New("central bank down")}, "bank-c")
	assertStatementStatus(t, h, "bank-c", http.StatusBadGateway)
}

// doStatementWithClaims drives GetStatement with a BankID claim and returns the
// decoded movements, asserting a 200.
func doStatementWithClaims(t *testing.T, h *StatementHandler, bankID string) []Movement {
	t.Helper()
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "u", BankID: bankID})
		return c.Next()
	})
	app.Get("/api/v1/statement", h.GetStatement)

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
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Movements
}

// assertStatementStatus drives GetStatement with a BankID claim and asserts the
// HTTP status code.
func assertStatementStatus(t *testing.T, h *StatementHandler, bankID string, want int) {
	t.Helper()
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "u", BankID: bankID})
		return c.Next()
	})
	app.Get("/api/v1/statement", h.GetStatement)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/statement", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Errorf("status = %d, want %d", resp.StatusCode, want)
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
