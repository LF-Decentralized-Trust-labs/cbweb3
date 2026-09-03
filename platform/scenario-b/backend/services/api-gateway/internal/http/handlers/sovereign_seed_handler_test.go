// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/gofiber/fiber/v2"
)

// stubSeedPreparer accepts any mint-and-approve.
type stubSeedPreparer struct{ err error }

func (s *stubSeedPreparer) MintAndApproveForAMM(_ context.Context, _, _, _ string) error {
	return s.err
}

func (s *stubSeedPreparer) MintToForAMM(_ context.Context, _, _, _, _ string) error { return nil }

func (s *stubSeedPreparer) ApproveAMM(_ context.Context, _, _, _ string) error { return nil }

// stubSeedEscrow returns a fixed side for the deposit.
type stubSeedEscrow struct {
	side string
	err  error
}

func (s *stubSeedEscrow) DepositSideForCommit(_ context.Context, _, _ string) (string, error) {
	return s.side, s.err
}

func (s *stubSeedEscrow) FinalizeCommitForPair(_ context.Context, _ string) (string, string, error) {
	return "1", "1", nil
}

func (s *stubSeedEscrow) CancelSideForCommit(_ context.Context, _ string) (string, error) {
	return s.side, nil
}

func (s *stubSeedEscrow) GetCommitEscrow(_ context.Context, _ string) (*handlers.EscrowStatus, error) {
	return &handlers.EscrowStatus{}, nil
}

// recordingLPWriter captures what the handler persists.
type recordingLPWriter struct {
	got []*domain.LiquidityPosition
	err error
}

func (w *recordingLPWriter) Create(_ context.Context, pos *domain.LiquidityPosition) error {
	w.got = append(w.got, pos)
	return w.err
}

func postDepositSide(t *testing.T, app *fiber.App, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/api/v2/amm/liquidity/deposit-side", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func newSeedApp(escrow *stubSeedEscrow, w handlers.LPPositionWriter, bankCode string) *fiber.App {
	h := handlers.NewSovereignSeedHandler(&stubSeedPreparer{}, escrow)
	if w != nil {
		h = h.WithLPPositions(w, bankCode)
	}
	app := fiber.New()
	app.Post("/api/v2/amm/liquidity/deposit-side", h.DepositSide)
	return app
}

// A sovereign deposit must leave a position row behind. Without one, the pool the CB
// just funded cannot be withdrawn from: /liquidity/remove is keyed by lp_id, and only
// this row carries it.
func TestDepositSide_RecordsTheDepositingCBsPosition(t *testing.T) {
	w := &recordingLPWriter{}
	app := newSeedApp(&stubSeedEscrow{side: "A"}, w, "central_bank_brazil")

	resp := postDepositSide(t, app, `{"pool_pair":"W-BRL-W-ARS","amount":"1000"}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(w.got) != 1 {
		t.Fatalf("expected exactly 1 position written, got %d", len(w.got))
	}

	pos := w.got[0]
	if pos.PoolPair != "W-BRL-W-ARS" {
		t.Errorf("pool_pair: got %q", pos.PoolPair)
	}
	if pos.ProviderBankID != "central_bank_brazil" {
		t.Errorf("provider must be this gateway's own CB: got %q", pos.ProviderBankID)
	}
	if pos.LPID == "" {
		t.Error("lp_id must be set — /liquidity/remove is keyed by it")
	}
	if pos.Status != domain.LPStatusActive {
		t.Errorf("status: got %q, want ACTIVE", pos.Status)
	}
	// Side A contributes token A only. Getting this backwards would send the
	// withdrawal out in the wrong currency: removeShares reads deposit_side to
	// decide which side is the provider's home currency.
	if pos.DepositSide != domain.DepositSideA {
		t.Errorf("deposit_side: got %q, want A", pos.DepositSide)
	}
	if pos.TokenAContributed != "1000" || pos.TokenBContributed != "0" {
		t.Errorf("contributions: got A=%q B=%q, want A=1000 B=0",
			pos.TokenAContributed, pos.TokenBContributed)
	}
	// Shares are unknown until the counterparty finalizes; removeShares falls back to
	// the gateway's on-chain LP balance when this is zero.
	if pos.LPShares != "0" {
		t.Errorf("lp_shares: got %q, want 0 (resolved on-chain at withdrawal)", pos.LPShares)
	}
}

// Side B is the mirror: contribution on token B, and the home currency that a
// withdrawal pays out in.
func TestDepositSide_SideBRecordsTokenBContribution(t *testing.T) {
	w := &recordingLPWriter{}
	app := newSeedApp(&stubSeedEscrow{side: "B"}, w, "central_bank_argentina")

	if resp := postDepositSide(t, app, `{"pool_pair":"W-BRL-W-ARS","amount":"287000"}`); resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(w.got) != 1 {
		t.Fatalf("expected 1 position, got %d", len(w.got))
	}
	pos := w.got[0]
	if pos.DepositSide != domain.DepositSideB {
		t.Errorf("deposit_side: got %q, want B", pos.DepositSide)
	}
	if pos.TokenAContributed != "0" || pos.TokenBContributed != "287000" {
		t.Errorf("contributions: got A=%q B=%q, want A=0 B=287000",
			pos.TokenAContributed, pos.TokenBContributed)
	}
}

// The escrow deposit has already moved value on-chain by the time the row is written,
// so a bookkeeping failure must not be reported as a failed deposit.
func TestDepositSide_PositionWriteFailureDoesNotFailTheDeposit(t *testing.T) {
	w := &recordingLPWriter{err: errors.New("duplicate key")}
	app := newSeedApp(&stubSeedEscrow{side: "A"}, w, "central_bank_brazil")

	resp := postDepositSide(t, app, `{"pool_pair":"W-BRL-W-ARS","amount":"1000"}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("a failed position write must not fail the deposit: got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "DEPOSITED" {
		t.Errorf("status: got %v, want DEPOSITED", body["status"])
	}
}

// A failed deposit must not leave a position claiming liquidity that was never escrowed.
func TestDepositSide_NoPositionWhenTheDepositFails(t *testing.T) {
	w := &recordingLPWriter{}
	app := newSeedApp(&stubSeedEscrow{side: "A", err: errors.New("escrow reverted")}, w, "central_bank_brazil")

	if resp := postDepositSide(t, app, `{"pool_pair":"W-BRL-W-ARS","amount":"1000"}`); resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
	if len(w.got) != 0 {
		t.Fatalf("no position may be recorded for a deposit that failed, got %d", len(w.got))
	}
}

// Without a writer the handler must behave exactly as before.
func TestDepositSide_WithoutWriterStillDeposits(t *testing.T) {
	app := newSeedApp(&stubSeedEscrow{side: "A"}, nil, "")
	if resp := postDepositSide(t, app, `{"pool_pair":"W-BRL-W-ARS","amount":"1000"}`); resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}
