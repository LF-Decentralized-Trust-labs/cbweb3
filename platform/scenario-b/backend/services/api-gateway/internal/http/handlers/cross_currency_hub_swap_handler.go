// SPDX-License-Identifier: Apache-2.0

// Package handlers provides the delegated Hub AMM swap handler for the issuing CB.
//
// POST /internal/amm/cross-currency-hub-swap
//
// Step 2 of a cross-currency swap trades W-<source> for W-<target> on the Hub AMM. The AMM
// gates both sides on the Hub IdentityRegistry (onlyVerified(msg.sender) / onlyVerified(to))
// and only central banks hold a Hub identity, so a commercial bank cannot be msg.sender.
//
// Until now the bank ran the swap from its own process using the CB's private key. That is
// not sovereign concentration — it is the sovereign key leaving the sovereign. This endpoint
// completes the delegation model the other three legs already follow (bridge-in, bridge-out,
// residue-return): the bank sends a trigger, and the CB executes with its own signer inside
// its own process. The bank never needs a Hub key.
//
// Security model (same as R2-CR-6 / residue-return): the request is a trigger, not an
// instruction. What the caller may spend is bounded by facts the CB owns:
//   - the bridge-in position is the CB's own record of what it minted on the Hub, and it must
//     belong to the payer bank named in the request;
//   - max_amount_in may not exceed that position's mirrored_amount, so a delegation can never
//     reach beyond the W-<source> the CB minted for it — this is what keeps one bank's swap
//     off another bank's balance on the shared CB address;
//   - the realized cost is decoded from the on-chain LogSwap, never echoed from the request;
//   - the payer bank's participant status is re-checked here, at payment initiation, not only
//     at onboarding;
//   - each bridge-in position funds at most one swap (replay returns the recorded tx hash).
//
// Ownership is checked before the replay lookup on purpose: answering "duplicate" to a
// request naming another bank's position would disclose that position's state to a caller
// with no claim to it.
//
// Protected by the same X-Relay-Auth / per-CB signature middleware as bridge-in.
package handlers

import (
	"context"
	"log"
	"math/big"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// HubSwapExecutorIface executes the AMM swap on the Hub with this CB's own signer.
type HubSwapExecutorIface interface {
	Execute(ctx context.Context, req services.SwapRequest) (*services.SwapResult, error)
}

// HubSwapRecorderIface is the replay guard for delegated swaps. FindByBridgeInPosition
// returns (nil, nil) when the position has not funded a swap yet; Record persists the
// outcome and returns the stored record when a concurrent delegation won the race.
type HubSwapRecorderIface interface {
	FindByBridgeInPosition(ctx context.Context, positionID string) (*services.HubSwapRecord, error)
	Record(ctx context.Context, rec *services.HubSwapRecord) (*services.HubSwapRecord, error)
}

// SwapDirectionResolverIface reports whether buying targetCurrency on poolPair outputs the
// pair's TOKEN_A, so one sovereign pair serves both directions of a corridor.
type SwapDirectionResolverIface interface {
	OutputIsTokenA(ctx context.Context, poolPair, targetCurrency string) (bool, error)
}

// CrossCurrencyHubSwapHandler handles POST /internal/amm/cross-currency-hub-swap.
type CrossCurrencyHubSwapHandler struct {
	executor          HubSwapExecutorIface
	positionReader    BridgePositionDetailReaderIface
	recorder          HubSwapRecorderIface
	payerResolver     BeneficiaryResolverIface
	directionResolver SwapDirectionResolverIface
	wTokenAddress     string // W-<source> on Hub (this CB's sovereign W token)
	// hubSignerAddress is this CB's own Hub address: the msg.sender of the trade and the
	// holder of its output. Returned to the delegating bank so Step 3 can tell the
	// beneficiary CB where the W-<target> actually sits.
	hubSignerAddress string
}

// NewCrossCurrencyHubSwapHandler constructs the handler.
func NewCrossCurrencyHubSwapHandler(
	executor HubSwapExecutorIface,
	positionReader BridgePositionDetailReaderIface,
	recorder HubSwapRecorderIface,
	payerResolver BeneficiaryResolverIface,
	wTokenAddress string,
	hubSignerAddress string,
) *CrossCurrencyHubSwapHandler {
	return &CrossCurrencyHubSwapHandler{
		executor:         executor,
		positionReader:   positionReader,
		recorder:         recorder,
		payerResolver:    payerResolver,
		wTokenAddress:    wTokenAddress,
		hubSignerAddress: hubSignerAddress,
	}
}

// WithDirectionResolver attaches the per-pair direction resolver. Without it the handler
// falls back to the A→B orientation, matching the orchestrator's local path.
func (h *CrossCurrencyHubSwapHandler) WithDirectionResolver(r SwapDirectionResolverIface) *CrossCurrencyHubSwapHandler {
	h.directionResolver = r
	return h
}

// HandleHubSwap processes the inbound Step 2 delegation.
//
// Expected JSON body:
//
//	{
//	  "correlation_id":        "<uuid>",
//	  "payer_bank_id":         "bank-a",
//	  "beneficiary_bank_id":   "bank-b",
//	  "bridge_in_position_id": "<uuid>",
//	  "pool_pair":             "W-BRL-W-COP",
//	  "target_currency":       "COP",
//	  "amount_out":            "<wei>",
//	  "max_amount_in":         "<wei>"
//	}
func (h *CrossCurrencyHubSwapHandler) HandleHubSwap(c *fiber.Ctx) error {
	var req struct {
		CorrelationID      string `json:"correlation_id"`
		PayerBankID        string `json:"payer_bank_id"`
		BeneficiaryBankID  string `json:"beneficiary_bank_id"`
		BridgeInPositionID string `json:"bridge_in_position_id"`
		PoolPair           string `json:"pool_pair"`
		TargetCurrency     string `json:"target_currency"`
		AmountOut          string `json:"amount_out"`
		MaxAmountIn        string `json:"max_amount_in"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CorrelationID == "" || req.PayerBankID == "" || req.BridgeInPositionID == "" ||
		req.PoolPair == "" || req.AmountOut == "" || req.MaxAmountIn == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "correlation_id, payer_bank_id, bridge_in_position_id, pool_pair, amount_out, max_amount_in are required",
		})
	}

	if h.executor == nil {
		// Fail closed: without a signing AMM client this CB cannot execute the swap, and
		// reporting success would strand the payer's bridged W-<source> on the Hub.
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "hub swap execution not configured on this gateway (HUB_BESU_RPC_URL / PAIR_REGISTRY_CONTRACT_ADDRESS / SIGNER_PRIVATE_KEY)",
			"code":  "EXECUTOR_NOT_CONFIGURED",
		})
	}
	if h.positionReader == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "bridge position reader not configured on this gateway",
			"code":  "POSITION_READER_NOT_CONFIGURED",
		})
	}
	if h.recorder == nil {
		// Without the replay guard a retried delegation would swap twice against the same
		// bridged balance. Refuse rather than execute unguarded.
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "hub swap replay guard not configured on this gateway",
			"code":  "RECORDER_NOT_CONFIGURED",
		})
	}
	if h.wTokenAddress == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "CB sovereign token address not configured (W_TOKEN_ADDRESS)",
			"code":  "CB_NOT_CONFIGURED",
		})
	}
	if h.hubSignerAddress == "" {
		// The caller needs this address for Step 3; answering without it would leave the
		// beneficiary CB burning from the wrong place.
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "this CB's Hub signer address is not resolved (SIGNER_PRIVATE_KEY)",
			"code":  "HUB_SIGNER_NOT_CONFIGURED",
		})
	}

	// The bridge-in position is this CB's own record of what it minted on the Hub. It is the
	// only admissible source for how much this delegation may spend.
	pos, err := h.positionReader.GetPosition(c.Context(), req.BridgeInPositionID)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position not found: " + err.Error(),
			"code":  "POSITION_NOT_FOUND",
		})
	}
	// Ownership first — before any lookup that could leak the position's state to a caller
	// that has no claim to it.
	if !strings.EqualFold(strings.TrimSpace(pos.OwnerBankID), strings.TrimSpace(req.PayerBankID)) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position belongs to another bank",
			"code":  "POSITION_OWNER_MISMATCH",
		})
	}
	if !strings.EqualFold(pos.MirroredAsset, h.wTokenAddress) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position is not denominated in this CB's W-token",
			"code":  "POSITION_TOKEN_MISMATCH",
		})
	}
	if pos.Leg != "" && pos.Leg != string(domain.BridgeLegSettlement) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "referenced position is not a settlement bridge-in",
			"code":  "POSITION_LEG_MISMATCH",
		})
	}
	// Only a completed lock-mint can fund a swap: before ACTIVE there is no W-<source> on
	// the Hub to spend.
	if !strings.EqualFold(pos.BridgeState, "ACTIVE") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "bridge-in position is not ACTIVE (state " + pos.BridgeState + ")",
			"code":  "POSITION_NOT_ACTIVE",
		})
	}

	// Compliance gate at payment initiation (not only at onboarding): the payer bank must
	// still be an ACTIVE participant of this CB. In the sovereign model the Hub sees only the
	// CB's identity, so this check is what carries bank-level authorisation.
	if h.payerResolver != nil {
		if _, resolveErr := h.payerResolver.ResolveWalletAddress(c.Context(), req.PayerBankID); resolveErr != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "payer bank not found or not active: " + resolveErr.Error(),
				"code":  "PAYER_NOT_FOUND",
			})
		}
	}

	// Replay protection: a bridge-in position funds exactly one swap. The AMM swap is not
	// idempotent on-chain, so a second run would spend the payer's bridged balance twice.
	existing, findErr := h.recorder.FindByBridgeInPosition(c.Context(), req.BridgeInPositionID)
	if findErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "idempotency lookup failed: " + findErr.Error(),
		})
	}
	if existing != nil {
		log.Printf("[correlation_id=%s] hub-swap replay detected: position=%s already swapped (tx=%s)",
			sanitizeLogField(req.CorrelationID), sanitizeLogField(req.BridgeInPositionID), existing.SwapTxHash)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":              "duplicate",
			"swap_sender_address": h.hubSignerAddress,
			"swap_tx_hash":        existing.SwapTxHash,
			"amount_in":           existing.AmountIn,
			"correlation_id":      req.CorrelationID,
		})
	}

	// The spend ceiling is what this CB minted for this position — never what the request asks
	// for. This is the invariant that keeps a delegation from reaching into another bank's
	// W-<source> on the CB's Hub address.
	bridged, ok := new(big.Int).SetString(strings.TrimSpace(pos.MirroredAmount), 10)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "bridge-in position has an unparseable mirrored_amount",
		})
	}
	maxAmountIn, ok := new(big.Int).SetString(strings.TrimSpace(req.MaxAmountIn), 10)
	if !ok || maxAmountIn.Sign() <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "max_amount_in must be a positive integer",
			"code":  "INVALID_MAX_AMOUNT_IN",
		})
	}
	if maxAmountIn.Cmp(bridged) > 0 {
		log.Printf("[correlation_id=%s] hub-swap refused: max_amount_in %s exceeds bridged %s (position %s)",
			sanitizeLogField(req.CorrelationID), maxAmountIn.String(), bridged.String(), sanitizeLogField(pos.PositionID))
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "max_amount_in exceeds the amount bridged in for this position",
			"code":  "MAX_AMOUNT_IN_EXCEEDS_BRIDGED",
		})
	}
	amountOut, ok := new(big.Int).SetString(strings.TrimSpace(req.AmountOut), 10)
	if !ok || amountOut.Sign() <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "amount_out must be a positive integer",
			"code":  "INVALID_AMOUNT_OUT",
		})
	}

	// Resolve the corridor direction from the pair's on-chain orientation so one sovereign
	// pair serves both directions. Best-effort, mirroring the orchestrator's local path.
	outputIsTokenA := false
	if h.directionResolver != nil {
		if isA, dErr := h.directionResolver.OutputIsTokenA(c.Context(), req.PoolPair, req.TargetCurrency); dErr == nil {
			outputIsTokenA = isA
		} else {
			log.Printf("[correlation_id=%s] WARNING: could not resolve swap direction for pool %s target %s: %v (defaulting A→B)",
				sanitizeLogField(req.CorrelationID), sanitizeLogField(req.PoolPair), sanitizeLogField(req.TargetCurrency), dErr)
		}
	}

	result, err := h.executor.Execute(c.Context(), services.SwapRequest{
		Pair:           req.PoolPair,
		AmountOut:      amountOut.String(),
		MaxAmountIn:    maxAmountIn.String(),
		PayerID:        req.PayerBankID,
		BeneficiaryID:  req.BeneficiaryBankID,
		CorrelationID:  req.CorrelationID,
		OutputIsTokenA: outputIsTokenA,
	})
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "hub swap failed: " + err.Error(),
			"code":  "SWAP_FAILED",
		})
	}

	// The swap is on-chain. Persist before answering so a retry is recognised as a replay
	// even if the caller never sees this response.
	stored, recErr := h.recorder.Record(c.Context(), &services.HubSwapRecord{
		BridgeInPositionID: req.BridgeInPositionID,
		CorrelationID:      req.CorrelationID,
		PayerBankID:        req.PayerBankID,
		PoolPair:           req.PoolPair,
		AmountOut:          amountOut.String(),
		AmountIn:           result.AmountIn,
		SwapTxHash:         result.TxHash,
	})
	if recErr != nil {
		// The swap already happened; losing the record is a reconciliation problem, not a
		// reason to report failure and invite a second swap. Surface it loudly instead.
		log.Printf("[correlation_id=%s] CRITICAL: hub swap executed (tx=%s) but recording it failed — a retry could swap twice: %v",
			sanitizeLogField(req.CorrelationID), result.TxHash, recErr)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":              "executed_unrecorded",
			"swap_sender_address": h.hubSignerAddress,
			"swap_tx_hash":        result.TxHash,
			"amount_in":           result.AmountIn,
			"correlation_id":      req.CorrelationID,
			"warning":             "swap executed but not recorded; reconciliation required before any retry",
		})
	}

	log.Printf("[correlation_id=%s] hub swap executed for %s: pool=%s amount_out=%s amount_in=%s tx=%s",
		sanitizeLogField(req.CorrelationID), sanitizeLogField(req.PayerBankID),
		sanitizeLogField(req.PoolPair), amountOut.String(), stored.AmountIn, stored.SwapTxHash)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":              "executed",
		"swap_sender_address": h.hubSignerAddress,
		"swap_tx_hash":        stored.SwapTxHash,
		"amount_in":           stored.AmountIn,
		"correlation_id":      req.CorrelationID,
	})
}
