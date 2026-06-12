// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

// AMMTokenPreparer prepares Hub tCeBM tokens for an addLiquidity call.
// FR-018: interface uses a single `amount` field; side detection is done by the adapter.
type AMMTokenPreparer interface {
	// MintAndApproveForAMM mints the token the signer holds CENTRAL_BANK_ROLE on
	// to their own address and approves the AMM to spend it.
	MintAndApproveForAMM(ctx context.Context, amount string) error
	// MintToForAMM mints the token the signer holds CENTRAL_BANK_ROLE on to recipient.
	MintToForAMM(ctx context.Context, recipient, amount string) error
	// ApproveAMM approves the AMM contract to spend `amount` of the token indicated by side.
	// side must be "A", "B", or "" (auto-detect; valid only for central bank callers).
	ApproveAMM(ctx context.Context, amount, side string) error
}

// CentralBankChecker checks on-chain whether a given Ethereum address belongs to a CB
// (via IdentityRegistry.getCentralBankOf or equivalent). Used by anti-G5-cross guard (T016).
type CentralBankChecker interface {
	// IsCentralBankAddress returns true if the address is registered as a CB signer.
	// Implementations may do an EVM call or a DB lookup.
	IsCentralBankAddress(ctx context.Context, address string) (bool, error)
}

// TokenHandler handles Hub tCeBM token operations (mint + approve) used as a
// prerequisite for central-bank liquidity provision on the Hub AMM.
type TokenHandler struct {
	preparer    AMMTokenPreparer
	cbChecker   CentralBankChecker // optional; enables anti-G5-cross guard (T016 / 007)
	approveSide string             // FR-013: "A"|"B" derived from BANK_CODE; when set, side is not read from payload
}

// NewTokenHandler creates a new TokenHandler.
func NewTokenHandler(p AMMTokenPreparer) *TokenHandler {
	return &TokenHandler{preparer: p}
}

// NewTokenHandlerWithCBChecker creates a TokenHandler with the anti-G5-cross guard enabled (T016).
func NewTokenHandlerWithCBChecker(p AMMTokenPreparer, checker CentralBankChecker) *TokenHandler {
	return &TokenHandler{preparer: p, cbChecker: checker}
}

// NewTokenHandlerWithConfig creates a TokenHandler with optional CB checker and approveSide config (FR-013).
// approveSide must be "A", "B", or "" (empty disables auto-detection).
func NewTokenHandlerWithConfig(p AMMTokenPreparer, checker CentralBankChecker, approveSide string) *TokenHandler {
	return &TokenHandler{preparer: p, cbChecker: checker, approveSide: approveSide}
}

// MintAndApprove handles POST /api/v2/amm/token/mint-and-approve.
// FR-018: accepts { "amount": "...", "recipient": "..." (optional) }.
// Rejects deprecated fields amount_a / amount_b with HTTP 400.
// T016 (007-bridge-based-cb-liquidity): when recipient is non-empty and a CentralBankChecker
// is configured, rejects minting to another CB with HTTP 403 CROSS_CB_MINT_PROHIBITED.
func (h *TokenHandler) MintAndApprove(c *fiber.Ctx) error {
	// Detect deprecated fields before parsing into the typed struct.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if _, hasA := raw["amount_a"]; hasA {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "use 'amount' instead of 'amount_a'/'amount_b'"})
	}
	if _, hasB := raw["amount_b"]; hasB {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "use 'amount' instead of 'amount_a'/'amount_b'"})
	}

	var req struct {
		Amount    string `json:"amount"`
		Recipient string `json:"recipient"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "amount is required and must be a non-negative integer string"})
	}

	// T016: anti-G5-cross guard — reject minting to a CB address.
	if req.Recipient != "" && h.cbChecker != nil {
		isCB, checkErr := h.cbChecker.IsCentralBankAddress(c.UserContext(), req.Recipient)
		if checkErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "recipient identity check failed: " + checkErr.Error(),
			})
		}
		if isCB {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "minting to another Central Bank address is prohibited (anti-G5-cross policy)",
				"code":  "CROSS_CB_MINT_PROHIBITED",
			})
		}
	}

	var err error
	if req.Recipient != "" {
		err = h.preparer.MintToForAMM(c.UserContext(), req.Recipient, req.Amount)
	} else {
		err = h.preparer.MintAndApproveForAMM(c.UserContext(), req.Amount)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "amm token prepare failed: " + err.Error()})
	}

	resp := fiber.Map{"status": "ok", "amount": req.Amount}
	if req.Recipient != "" {
		resp["recipient"] = req.Recipient
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// ApproveAMM handles POST /api/v2/amm/token/approve-amm.
// FR-018: accepts { "amount": "...", "side": "A"|"B" (optional for CBs, required for banks) }.
// FR-013: when approveSide is configured (BANK_CODE), side is derived from config and payload side is ignored.
// Rejects deprecated fields amount_a / amount_b with HTTP 400.
func (h *TokenHandler) ApproveAMM(c *fiber.Ctx) error {
	// Detect deprecated fields before parsing into the typed struct.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &raw); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if _, hasA := raw["amount_a"]; hasA {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "use 'amount' instead of 'amount_a'/'amount_b'"})
	}
	if _, hasB := raw["amount_b"]; hasB {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "use 'amount' instead of 'amount_a'/'amount_b'"})
	}

	var req struct {
		Amount string `json:"amount"`
		Side   string `json:"side"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "amount is required and must be a non-negative integer string"})
	}

	// FR-013: when BANK_CODE is configured, derive side from config and ignore payload side entirely.
	// CB G5-cross (explicit side override) is unaffected: CBs have approveSide="" and pass side explicitly.
	side := req.Side
	if h.approveSide != "" {
		side = h.approveSide
	}

	if side != "" && side != "A" && side != "B" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "side must be 'A' or 'B'"})
	}

	if err := h.preparer.ApproveAMM(c.UserContext(), req.Amount, side); err != nil {
		if err.Error() == "token_prepare: side is required for non-central-bank callers" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "BANK_CODE not configured on this gateway — set BANK_CODE env var",
				"code":  "COMMIT_SIDE_NOT_CONFIGURED",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "amm approve failed: " + err.Error()})
	}

	resp := fiber.Map{"status": "ok", "amount": req.Amount}
	if side != "" {
		resp["side"] = side
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}
