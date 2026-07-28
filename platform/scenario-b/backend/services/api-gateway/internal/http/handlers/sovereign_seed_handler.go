// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

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

// SovereignSeedHandler exposes the simplified sovereign liquidity seeding:
//
//	POST /api/v2/amm/liquidity/deposit-side  {pool_pair, amount}
//	POST /api/v2/amm/liquidity/finalize      {pool_pair}
//	POST /api/v2/amm/liquidity/reclaim-side  {pool_pair}
//	GET  /api/v2/amm/liquidity/escrow?pool_pair=...
type SovereignSeedHandler struct {
	preparer AMMTokenPreparer    // mints the CB's own side + approves the pool's AMM
	escrow   SovereignSeedEscrow // deposit/finalize/cancel/read the AMM escrow
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
