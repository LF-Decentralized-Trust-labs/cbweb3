// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

// SovereignSeedEscrow drives the AMM's escrow-and-finalize primitives per pool_pair,
// with the side auto-resolved on-chain. It backs the sovereign seeding flow (each CB
// deposits ONLY its own currency; finalize funds reserves atomically once both sides
// are in; a CB can reclaim its own pending side before finalize) WITHOUT the LCR.
type SovereignSeedEscrow interface {
	DepositSideForCommit(ctx context.Context, poolPair, amount string) (side string, err error)
	FinalizeCommitForPair(ctx context.Context, poolPair string) (sharesA, sharesB string, err error)
	CancelSideForCommit(ctx context.Context, poolPair string) (side string, err error)
	GetCommitEscrow(ctx context.Context, poolPair string) (*EscrowStatus, error)
}

// EscrowStatus is a UI-facing view of a pool's escrow commit state.
type EscrowStatus struct {
	SideADeposited bool   `json:"side_a_deposited"`
	SideBDeposited bool   `json:"side_b_deposited"`
	AmountA        string `json:"amount_a"`
	AmountB        string `json:"amount_b"`
	Finalized      bool   `json:"finalized"`
}

// LPPositionWriter persists the depositing CB's own liquidity position.
//
// Without it a sovereignly-seeded pool has no position row, and /liquidity/remove —
// which is keyed by lp_id — has nothing to address: a central bank could fund a pool
// and then not withdraw from it through the API. The commit/LCR flow used to write
// this row on its way through ExecuteMatchedCommit; escrow-and-finalize replaced that
// path and did not carry the write over.
type LPPositionWriter interface {
	Create(ctx context.Context, pos *domain.LiquidityPosition) error
}

// SovereignSeedHandler exposes the simplified sovereign liquidity seeding:
//
//	POST /api/v2/amm/liquidity/deposit-side  {pool_pair, amount}
//	POST /api/v2/amm/liquidity/finalize      {pool_pair}
//	POST /api/v2/amm/liquidity/reclaim-side  {pool_pair}
//	GET  /api/v2/amm/liquidity/escrow?pool_pair=...
type SovereignSeedHandler struct {
	preparer AMMTokenPreparer    // mints the CB's own side + approves the pool's AMM
	escrow   SovereignSeedEscrow // deposit/finalize/cancel/read the AMM escrow
	lpWriter LPPositionWriter    // optional — records this CB's own position on deposit
	bankCode string              // this gateway's own CB code, the position's provider
}

// WithLPPositions records the depositing CB's own liquidity position, so the pool it
// seeds is withdrawable through /liquidity/remove. Optional: without it the handler
// behaves exactly as before.
func (h *SovereignSeedHandler) WithLPPositions(w LPPositionWriter, bankCode string) *SovereignSeedHandler {
	h.lpWriter = w
	h.bankCode = bankCode
	return h
}

// NewSovereignSeedHandler wires the token preparer and the escrow driver.
func NewSovereignSeedHandler(preparer AMMTokenPreparer, escrow SovereignSeedEscrow) *SovereignSeedHandler {
	return &SovereignSeedHandler{preparer: preparer, escrow: escrow}
}

// DepositSide mints the caller CB's own side of the pair and escrows it against the
// pool's shared commit. The side is auto-resolved from the pair (no picker).
func (h *SovereignSeedHandler) DepositSide(c *fiber.Ctx) error {
	var req struct {
		PoolPair string `json:"pool_pair"`
		Amount   string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair and amount are required"})
	}
	// 1) Mint the CB's own W-token (it holds CENTRAL_BANK_ROLE on its own side) and
	//    approve the pool's AMM. Side "" ⇒ auto-resolved on-chain from the pair.
	if err := h.preparer.MintAndApproveForAMM(c.Context(), req.PoolPair, "", req.Amount); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error(), "error_code": "MINT_APPROVE_FAILED"})
	}
	// 2) Escrow the side against the pool's shared commit.
	side, err := h.escrow.DepositSideForCommit(c.Context(), req.PoolPair, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error(), "error_code": "DEPOSIT_FAILED"})
	}
	// 3) Record THIS CB's own position — its own side, on its own gateway, which is the
	//    same sovereignty boundary the deposit itself respects. lp_shares is left at "0"
	//    on purpose: the share count is only known after the counterparty finalizes, and
	//    the withdrawal path already falls back to this gateway's on-chain LP balance
	//    when the recorded value is zero (see LiquidityProvisionService.removeShares).
	h.recordPosition(c.Context(), req.PoolPair, side, req.Amount)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"pool_pair": req.PoolPair, "side": side, "amount": req.Amount, "status": "DEPOSITED"})
}

// Finalize funds the pool atomically once both sides are escrowed. Reverts (surfaced as
// 409) while the counterpart side is still pending.
func (h *SovereignSeedHandler) Finalize(c *fiber.Ctx) error {
	var req struct {
		PoolPair string `json:"pool_pair"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair is required"})
	}
	sharesA, sharesB, err := h.escrow.FinalizeCommitForPair(c.Context(), req.PoolPair)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":      err.Error(),
			"error_code": "FINALIZE_INCOMPLETE",
			"hint":       "both sides must be deposited before finalize",
		})
	}
	return c.JSON(fiber.Map{"pool_pair": req.PoolPair, "status": "FINALIZED", "shares_a": sharesA, "shares_b": sharesB})
}

// ReclaimSide pulls the caller CB's own pending side back (before finalize).
func (h *SovereignSeedHandler) ReclaimSide(c *fiber.Ctx) error {
	var req struct {
		PoolPair string `json:"pool_pair"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair is required"})
	}
	side, err := h.escrow.CancelSideForCommit(c.Context(), req.PoolPair)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error(), "error_code": "RECLAIM_FAILED"})
	}
	return c.JSON(fiber.Map{"pool_pair": req.PoolPair, "side": side, "status": "RECLAIMED"})
}

// EscrowState returns the current escrow commit state for the pool (for UI polling).
func (h *SovereignSeedHandler) EscrowState(c *fiber.Ctx) error {
	poolPair := c.Query("pool_pair")
	if poolPair == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair query parameter is required"})
	}
	st, err := h.escrow.GetCommitEscrow(c.Context(), poolPair)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(st)
}

// recordPosition writes the caller CB's liquidity position for a deposited side.
//
// Best-effort by design: the escrow deposit has already succeeded on-chain when this
// runs, so failing the request here would report a failure for value that did move.
// A duplicate key (the CB re-deposits after a reclaim) is equally ignored — the row
// is keyed per pool and provider, and one is all the withdrawal path needs.
func (h *SovereignSeedHandler) recordPosition(ctx context.Context, poolPair, side, amount string) {
	if h.lpWriter == nil {
		return
	}
	tokenA, tokenB := amount, "0"
	depositSide := domain.DepositSideA
	if side == string(domain.DepositSideB) {
		tokenA, tokenB = "0", amount
		depositSide = domain.DepositSideB
	}
	_ = h.lpWriter.Create(ctx, &domain.LiquidityPosition{
		LPID:              poolPair + "-" + side + "-sovereign",
		ProviderBankID:    h.bankCode,
		PoolPair:          poolPair,
		TokenAContributed: tokenA,
		TokenBContributed: tokenB,
		LPShares:          "0",
		Status:            domain.LPStatusActive,
		DepositSide:       depositSide,
		AddedAt:           time.Now().UTC(),
	})
}
