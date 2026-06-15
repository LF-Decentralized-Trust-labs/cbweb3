// SPDX-License-Identifier: Apache-2.0

// Package handlers provides the cross-currency bridge-in handler for the issuing CB.
//
// POST /internal/amm/cross-currency-bridge-in
//
// Called by a commercial bank's CrossCurrencySwapOrchestrator (Step 1) via the spoke's
// CENTRAL_BANK_API_URL. Minting wrapped sovereign money (W-<source>) on the Hub requires
// CENTRAL_BANK_ROLE, which only the issuing CB holds — so the CB performs the lock-mint on
// its own relayer (with its own signer) on behalf of the payer bank.
//
// Reserve Tokenisation enforcement:
// The payer bank MUST hold tCeBM obtained via the Reserve Tokenisation flow (ApproveEscrow:
// fCeBM burned → tCeBM minted). This handler verifies the bank's tCeBM balance before
// proceeding. The relayer executor then burns the bank's tCeBM on the spoke and mints
// W-<source> on the Hub — no new tCeBM is created.
//
// This is the bridge-in counterpart of CrossCurrencyBridgeOutHandler. It is synchronous:
// it enqueues the lock-mint and blocks until the bridge position reaches ACTIVE, because
// the AMM swap (Step 2) cannot proceed until the W-<source> tokens exist on the Hub.
//
// Protected by X-Relay-Auth middleware (same secret as cross-currency-bridge-out).
package handlers

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// CrossCurrencyLockMintEnqueuerIface enqueues a sovereign lock-mint (CB relayer mints W-<source>).
// extras[0] = mintToHubAddress, extras[1] = burnFromSpokeAddress (payer bank's wallet).
type CrossCurrencyLockMintEnqueuerIface interface {
	LockAndEnqueue(
		ctx context.Context,
		ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount, correlationID string,
		extras ...string,
	) (*services.BridgePositionResult, error)
}

// BridgeStateReaderIface reads the current lifecycle state of a bridge position.
type BridgeStateReaderIface interface {
	GetBridgeState(ctx context.Context, positionID string) (domain.BridgeState, error)
}

// PayerBalanceCheckerIface reads the tCeBM balance of a given on-chain address.
// Used to verify the payer bank has sufficient tokenized reserves before bridge-in.
type PayerBalanceCheckerIface interface {
	GetBalanceOf(ctx context.Context, address string) (string, error)
}

// PayerWalletResolverIface resolves a bank_code to its registered on-chain wallet address.
type PayerWalletResolverIface interface {
	ResolveWalletAddress(ctx context.Context, bankCode string) (string, error)
}

// CrossCurrencyBridgeInHandler handles POST /internal/amm/cross-currency-bridge-in.
type CrossCurrencyBridgeInHandler struct {
	lockMintEnqueuer CrossCurrencyLockMintEnqueuerIface
	stateReader      BridgeStateReaderIface
	// balanceChecker and walletResolver enforce Reserve Tokenisation: the payer bank must
	// hold tCeBM before a bridge-in is allowed. Both are optional (nil = no enforcement,
	// for CB self-service sovereign positions that set their own mint path).
	balanceChecker PayerBalanceCheckerIface
	walletResolver PayerWalletResolverIface
	wTokenAddress    string // W-<source> on Hub (this CB's sovereign W token, e.g. W-BRL)
	fiatTokenAddress string // tCeBM-<source> on this CB's spoke (native asset)
	spokeNetwork     string // "spoke-a"
	// activeTimeout bounds how long to wait for the lock-mint to reach ACTIVE.
	activeTimeout time.Duration
}

// NewCrossCurrencyBridgeInHandler constructs the handler.
func NewCrossCurrencyBridgeInHandler(
	lockMintEnqueuer CrossCurrencyLockMintEnqueuerIface,
	stateReader BridgeStateReaderIface,
	wTokenAddress, fiatTokenAddress, spokeNetwork string,
) *CrossCurrencyBridgeInHandler {
	return &CrossCurrencyBridgeInHandler{
		lockMintEnqueuer: lockMintEnqueuer,
		stateReader:      stateReader,
		wTokenAddress:    wTokenAddress,
		fiatTokenAddress: fiatTokenAddress,
		spokeNetwork:     spokeNetwork,
		activeTimeout:    120 * time.Second,
	}
}

// WithReserveTokenisationEnforcement attaches the balance checker and wallet resolver so
// the handler can verify the payer bank holds tCeBM before enqueuing the bridge-in.
func (h *CrossCurrencyBridgeInHandler) WithReserveTokenisationEnforcement(
	balanceChecker PayerBalanceCheckerIface,
	walletResolver PayerWalletResolverIface,
) *CrossCurrencyBridgeInHandler {
	h.balanceChecker = balanceChecker
	h.walletResolver = walletResolver
	return h
}

// HandleBridgeIn processes the inbound bridge-in delegation from a commercial bank.
//
// Expected JSON body:
//
//	{
//	  "correlation_id":     "<uuid>",
//	  "payer_bank_id":      "bank-a",
//	  "source_currency":    "BRL",
//	  "amount":             "1000",
//	  "spoke_in":           "spoke-a",
//	  "swap_sender_address": "0x..."
//	}
func (h *CrossCurrencyBridgeInHandler) HandleBridgeIn(c *fiber.Ctx) error {
	var req struct {
		CorrelationID  string `json:"correlation_id"`
		PayerBankID    string `json:"payer_bank_id"`
		SourceCurrency string `json:"source_currency"`
		Amount         string `json:"amount"`
		SpokeIn        string `json:"spoke_in"`
		// SwapSenderAddress is the initiating gateway's Hub swap signer — where the AMM swap
		// (Step 2) is executed from and where bridge-out (Step 3) later burns. The CB mints
		// W-<source> to this address so the swap has tokens to spend.
		SwapSenderAddress string `json:"swap_sender_address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CorrelationID == "" || req.PayerBankID == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "correlation_id, payer_bank_id, amount are required",
		})
	}

	if h.wTokenAddress == "" || h.fiatTokenAddress == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "CB sovereign token addresses not configured (W_TOKEN_ADDRESS / TOKEN_ADDRESS)",
			"code":  "CB_NOT_CONFIGURED",
		})
	}

	spokeIn := strings.TrimSpace(req.SpokeIn)
	if spokeIn == "" {
		spokeIn = h.spokeNetwork
	}

	// ── Reserve Tokenisation enforcement ──────────────────────────────────────────────
	// Resolve the payer bank's spoke wallet and verify it holds enough tCeBM. The CB
	// MUST NOT mint new tCeBM here — the bank must have converted fCeBM via ApproveEscrow.
	// If walletResolver or balanceChecker are nil (sovereign CB self-service path), skip.
	payerWallet := ""
	if h.walletResolver != nil && h.balanceChecker != nil {
		wallet, resolveErr := h.walletResolver.ResolveWalletAddress(c.Context(), req.PayerBankID)
		if resolveErr != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "payer bank not found or not active: " + resolveErr.Error(),
				"code":  "PAYER_NOT_FOUND",
			})
		}
		payerWallet = wallet

		balStr, balErr := h.balanceChecker.GetBalanceOf(c.Context(), payerWallet)
		if balErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "could not read payer tCeBM balance: " + balErr.Error(),
			})
		}
		bal, ok := new(big.Int).SetString(strings.TrimSpace(balStr), 10)
		if !ok || bal == nil {
			bal = new(big.Int)
		}
		required, ok2 := new(big.Int).SetString(strings.TrimSpace(req.Amount), 10)
		if !ok2 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid amount"})
		}
		if bal.Cmp(required) < 0 {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": fmt.Sprintf(
					"insufficient tokenized reserves: bank %s holds %s tCeBM, need %s — complete Reserve Tokenisation first",
					req.PayerBankID, bal.String(), required.String(),
				),
				"code":    "INSUFFICIENT_TOKENIZED_RESERVES",
				"balance": bal.String(),
				"required": required.String(),
			})
		}
	}

	// Enqueue lock-mint. extras[0] = Hub mint recipient, extras[1] = bank's spoke wallet
	// (executor burns from here instead of auto-minting — enforces reserve backing).
	pos, err := h.lockMintEnqueuer.LockAndEnqueue(
		c.Context(),
		req.PayerBankID,
		spokeIn,
		h.fiatTokenAddress,
		h.wTokenAddress,
		req.Amount,
		req.CorrelationID,
		strings.TrimSpace(req.SwapSenderAddress), // extras[0] = mintToHubAddress
		payerWallet,                               // extras[1] = burnFromSpokeAddress
	)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in lock-mint enqueue failed: " + err.Error(),
		})
	}

	// Synchronously wait for the CB relayer to drive the position to ACTIVE. The commercial
	// orchestrator depends on W-<source> existing before it runs the AMM swap (Step 2).
	state, waitErr := h.waitForActive(c.Context(), pos.PositionID)
	if waitErr != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":        "bridge-in did not reach ACTIVE: " + waitErr.Error(),
			"position_id":  pos.PositionID,
			"bridge_state": string(state),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":         "active",
		"position_id":    pos.PositionID,
		"bridge_state":   string(state),
		"correlation_id": req.CorrelationID,
	})
}

// waitForActive polls the bridge position until ACTIVE, RECONCILIATION_REQUIRED, or timeout.
func (h *CrossCurrencyBridgeInHandler) waitForActive(ctx context.Context, positionID string) (domain.BridgeState, error) {
	const interval = 2 * time.Second
	deadline := time.Now().Add(h.activeTimeout)
	var last domain.BridgeState
	for {
		state, err := h.stateReader.GetBridgeState(ctx, positionID)
		if err != nil {
			return last, err
		}
		last = state
		if state == domain.BridgeStateActive {
			return state, nil
		}
		if state == domain.BridgeStateReconciliationRequired {
			return state, fmt.Errorf("bridge position %s requires reconciliation", positionID)
		}
		if time.Now().After(deadline) {
			return state, fmt.Errorf("timeout waiting for ACTIVE on position %s (last state: %s)", positionID, state)
		}
		select {
		case <-ctx.Done():
			return state, ctx.Err()
		case <-time.After(interval):
		}
	}
}
