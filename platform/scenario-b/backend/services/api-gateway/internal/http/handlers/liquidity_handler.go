// SPDX-License-Identifier: Apache-2.0

// Package handlers provides liquidity provision HTTP handlers for Scenario B (FR-027).
// Extended for 005-cooperative-liquidity: commit-reveal, pool status enrichment.
// Extended for 007-bridge-based-cb-liquidity: sovereign CB liquidity, on-chain commit gate.
package handlers

import (
	"context"
	"errors"
	"log"
	"math/big"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// LiquidityServiceIface is the interface consumed by LiquidityHandler.
type LiquidityServiceIface interface {
	AddLiquidity(ctx context.Context, req services.LiquidityProvisionRequest) (*services.LPResult, error)
	RemoveLiquidity(ctx context.Context, req services.LiquidityRemoveRequest) (*services.LPResult, error)
	RegisterCommit(ctx context.Context, req services.CommitRequest) (*services.CommitResult, error)
	ListCommits(ctx context.Context, poolPair, status string) ([]domain.PoolCommit, error)
	GetCommit(ctx context.Context, commitID string) (*domain.PoolCommit, error)
	CancelCommit(ctx context.Context, commitID, providerID string) error
}

// SovereignBridgeCheckerIface supports the bridge_state gate for sovereign CB pairs (T008).
type SovereignBridgeCheckerIface interface {
	// IsSovereignPair returns true if the poolPair is configured as a sovereign pair.
	IsSovereignPair(poolPair string) bool
	// HasActiveBridgePosition returns true if the CB identified by ownerBankID has at
	// least one BridgedAssetPosition with bridge_state = ACTIVE.
	HasActiveBridgePosition(ctx context.Context, ownerBankID string) (bool, error)
}

// OnChainCommitRegistrarIface calls LiquidityCommitRegistry.registerCommit on the Hub (T010).
type OnChainCommitRegistrarIface interface {
	RegisterCommit(ctx context.Context, poolPair string, side uint8, amount *big.Int, wTokenAddress string) ([]byte, error)
}

// SovereignLiquidityServiceIface handles the matched-commit execution for sovereign pairs (T012/T013).
// Alias of services.SovereignLiquidityServiceIface — use that type directly in handler wiring.
type SovereignLiquidityServiceIface = services.SovereignLiquidityServiceIface

// SovereignExecuteRequest re-exported from services to keep handler code concise.
type SovereignExecuteRequest = services.SovereignExecuteRequest

// LPPositionReaderIface is the interface for reading LiquidityPosition records.
type LPPositionReaderIface interface {
	FindByPoolPair(ctx context.Context, poolPair string) ([]domain.LiquidityPosition, error)
	FindByProviderAndPoolPair(ctx context.Context, providerID, poolPair string) ([]domain.LiquidityPosition, error)
}

// LPBalanceReaderIface reads the gateway's on-chain LP-share position (specs/013-amm-lp-shares).
// An empty holder resolves to the gateway's configured signer — the CB itself (decisions D4/D7).
type LPBalanceReaderIface interface {
	LPBalanceOf(ctx context.Context, holder string) (*big.Int, error)
	LPTotalSupply(ctx context.Context) (*big.Int, error)
}

// PairSideResolverIface derives which side ("A"/"B") this central bank owns for a
// pool_pair, from the on-chain CENTRAL_BANK_ROLE on the pair's tokens. This replaces
// the brittle BANK_CODE→side mapping: the side is an on-chain property of (pair, CB),
// so any CB (any naming) is handled and the operator never picks a side.
type PairSideResolverIface interface {
	SideForSigner(ctx context.Context, poolPair string) (string, error)
}

// LiquidityHandler handles add/remove liquidity operations.
type LiquidityHandler struct {
	svc LiquidityServiceIface
	// Optional sovereign extensions (007-bridge-based-cb-liquidity).
	bridgeChecker SovereignBridgeCheckerIface
	lcrRegistrar  OnChainCommitRegistrarIface
	sovereignSvc  SovereignLiquidityServiceIface
	lpRepo        LPPositionReaderIface // for GET /liquidity/positions (008-fix-cb-liquidity)
	lpBalance     LPBalanceReaderIface  // for GET /lp-balance (013-amm-lp-shares)
	sideResolver  PairSideResolverIface // derives the CB's side per pair (on-chain role)
	// Simplified API config (008-fix-cb-liquidity)
	commitSide       string
	wTokenAddress    string
	localCBHubSigner string // LOCAL_CB_HUB_SIGNER — address that receives lock-mint tokens (for balance checks)
	fallbackBankCode string // BANK_CODE fallback for provider resolution when JWT carries no BankID (CB service accounts)
}

// SetFallbackBankCode configures the BANK_CODE fallback used to resolve the
// provider id when JWT claims carry no BankID (e.g. CB service accounts),
// mirroring the bridge handler's behavior.
func (h *LiquidityHandler) SetFallbackBankCode(bankCode string) *LiquidityHandler {
	h.fallbackBankCode = bankCode
	return h
}

// NewLiquidityHandler creates a LiquidityHandler (cooperative mode only).
func NewLiquidityHandler(svc LiquidityServiceIface) *LiquidityHandler {
	return &LiquidityHandler{svc: svc}
}

// WithLPBalanceReader enables GET /api/v2/amm/lp-balance (on-chain CBW3-LP position, 013).
func (h *LiquidityHandler) WithLPBalanceReader(r LPBalanceReaderIface) *LiquidityHandler {
	h.lpBalance = r
	return h
}

// WithSideResolver wires the on-chain per-pair side resolver so CommitLiquidity
// derives the CB's side from the pair instead of the BANK_CODE mapping.
func (h *LiquidityHandler) WithSideResolver(r PairSideResolverIface) *LiquidityHandler {
	h.sideResolver = r
	return h
}

// NewLiquidityHandlerSovereign creates a LiquidityHandler with sovereign extensions
// for 007-bridge-based-cb-liquidity and simplified API support (008-fix-cb-liquidity).
func NewLiquidityHandlerSovereign(
	svc LiquidityServiceIface,
	bridgeChecker SovereignBridgeCheckerIface,
	lcrRegistrar OnChainCommitRegistrarIface,
	sovereignSvc SovereignLiquidityServiceIface,
	lpRepo LPPositionReaderIface,
	commitSide string, // A or B (derived from BANK_CODE)
	wTokenAddress string, // Hub W-tCeBM address
	localCBHubSigner string, // LOCAL_CB_HUB_SIGNER — address that receives lock-mint tokens
) *LiquidityHandler {
	return &LiquidityHandler{
		svc:              svc,
		bridgeChecker:    bridgeChecker,
		lcrRegistrar:     lcrRegistrar,
		sovereignSvc:     sovereignSvc,
		lpRepo:           lpRepo,
		commitSide:       commitSide,
		wTokenAddress:    wTokenAddress,
		localCBHubSigner: localCBHubSigner,
	}
}

// AddLiquidity handles POST /api/v2/amm/liquidity/add (FR-027).
//
// NOTE: this handler is not currently wired to any route — the dual-sided add was
// replaced by the sovereign escrow-and-finalize flow (deposit-side/finalize). It is
// kept defensively hardened so it cannot reintroduce the R2-H-9/H-10 defect if ever
// re-mounted: the provider identity is derived from the authenticated JWT, never
// from the body.
func (h *LiquidityHandler) AddLiquidity(c *fiber.Ctx) error {
	var req struct {
		PoolPair     string `json:"pool_pair"`
		TokenAAmount string `json:"token_a_amount"`
		TokenBAmount string `json:"token_b_amount"`
		// Deprecated: provider_bank_id is derived from the authenticated JWT, never
		// trusted from the body (R2-H-9/H-10).
		ProviderBankID string `json:"provider_bank_id,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" || req.TokenAAmount == "" || req.TokenBAmount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair, token_a_amount, token_b_amount required"})
	}

	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}
	providerID := claims.BankID
	if providerID == "" {
		providerID = h.fallbackBankCode
	}
	if providerID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}
	if req.ProviderBankID != "" && req.ProviderBankID != providerID {
		log.Printf("[liquidity] deprecated provider_bank_id payload (%s) ignored; using authenticated identity %s", req.ProviderBankID, providerID)
	}

	result, err := h.svc.AddLiquidity(c.Context(), services.LiquidityProvisionRequest{
		PoolPair:       req.PoolPair,
		ProviderBankID: providerID,
		TokenAAmount:   req.TokenAAmount,
		TokenBAmount:   req.TokenBAmount,
	})
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// RemoveLiquidity handles POST /api/v2/amm/liquidity/remove (FR-027).
// Routes to LEGACY or PROPORTIONAL withdrawal based on deposit_side (D7).
// T015 (007-bridge-based-cb-liquidity): validates provider_id == JWT BankID.
func (h *LiquidityHandler) RemoveLiquidity(c *fiber.Ctx) error {
	var req struct {
		PoolPair       string `json:"pool_pair"`
		ProviderBankID string `json:"provider_bank_id"`
		LPID           string `json:"lp_id"`
		// FractionBps selects how much of the position to withdraw, in basis points
		// (10000 = 100%). Omitted/0 → full withdrawal (backward compatible).
		FractionBps int `json:"fraction_bps,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" || req.ProviderBankID == "" || req.LPID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair, provider_bank_id, lp_id required"})
	}
	if req.FractionBps < 0 || req.FractionBps > 10000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "fraction_bps must be between 1 and 10000"})
	}

	// T015: validate provider_bank_id == JWT BankID (FR-009).
	if claims, ok := c.Locals("claims").(domain.TokenClaims); ok && claims.BankID != "" {
		if req.ProviderBankID != claims.BankID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "provider_bank_id mismatch — must match authenticated CB identity",
				"code":  "PROVIDER_ID_MISMATCH",
			})
		}
	}

	result, err := h.svc.RemoveLiquidity(c.Context(), services.LiquidityRemoveRequest{
		PoolPair:       req.PoolPair,
		ProviderBankID: req.ProviderBankID,
		LPID:           req.LPID,
		FractionBps:    req.FractionBps,
	})
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// CommitLiquidity handles POST /api/v2/amm/liquidity/commit (T013 / FR-001).
// Registers a single-sided deposit intent (commit-reveal phase 1).
// Returns 201 on success, 409 COMMIT_ALREADY_EXISTS / SAME_PROVIDER_BOTH_SIDES on conflict.
//
// Simplified API (008-fix-cb-liquidity): accepts {"pool_pair": "...", "amount": "..."} and derives:
//   - provider_id from JWT claims.BankID
//   - side from config (A or B based on BANK_CODE)
//   - w_token_address from config
//
// For sovereign CB pairs (007-bridge-based-cb-liquidity):
//   - T009: validates that provider_id matches the authenticated JWT client_id (BankID).
//   - T008: checks that the CB has an ACTIVE bridge position before committing.
//   - T010: registers the commit on-chain via LiquidityCommitRegistry and persists the bytes32 commitId.
func (h *LiquidityHandler) CommitLiquidity(c *fiber.Ctx) error {
	var req struct {
		PoolPair string `json:"pool_pair"`
		Amount   string `json:"amount"`
		// Deprecated fields (backward compatibility - will be removed in future version)
		ProviderID    string `json:"provider_id,omitempty"`
		Side          string `json:"side,omitempty"`
		WTokenAddress string `json:"w_token_address,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair and amount required"})
	}

	// Derive provider_id from JWT (simplified API). CB service-account tokens carry
	// no BankID, so fall back to the configured BANK_CODE — mirroring the bridge
	// handler — otherwise a CB providing liquidity would be rejected with 401.
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}
	providerID := claims.BankID
	if providerID == "" {
		providerID = h.fallbackBankCode
		if providerID != "" {
			log.Printf("[liquidity] BankID missing in claims; using configured BANK_CODE fallback: %s", providerID)
		}
	}
	if providerID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}

	// Side is an on-chain property of (pair, CB): the token this CB is the central
	// bank of. Resolve it from the pair first (works for any CB, no BANK_CODE map),
	// then fall back to config/request only if the resolver is unavailable.
	side := ""
	if req.PoolPair != "" && h.sideResolver != nil {
		if s, rerr := h.sideResolver.SideForSigner(c.Context(), req.PoolPair); rerr == nil {
			side = s
		} else {
			log.Printf("[liquidity] side resolve for %s failed, falling back to config: %v", req.PoolPair, rerr)
		}
	}
	wTokenAddr := h.wTokenAddress

	// Backward compatibility: config/client-provided values if the pair resolver is unset.
	if side == "" {
		side = h.commitSide
	}
	if side == "" {
		side = req.Side
	}
	if wTokenAddr == "" {
		wTokenAddr = req.WTokenAddress
	}

	// Validate required fields after derivation
	if side == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "commit_side not configured — set BANK_CODE env var or provide 'side' in payload",
			"code":  "COMMIT_SIDE_NOT_CONFIGURED",
		})
	}
	if side != "A" && side != "B" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "side must be 'A' or 'B'"})
	}

	// Log deprecation warnings if client sends redundant fields
	if req.ProviderID != "" && req.ProviderID != providerID {
		log.Printf("[DEPRECATED] provider_id in payload (%s) differs from JWT (%s) - using JWT value", req.ProviderID, providerID)
	}
	if req.Side != "" && h.commitSide != "" {
		log.Printf("[DEPRECATED] side in payload - should be derived from BANK_CODE config")
	}
	if req.WTokenAddress != "" && h.wTokenAddress != "" {
		log.Printf("[DEPRECATED] w_token_address in payload - should be derived from server config")
	}

	// T009: validate provider_id == JWT BankID (now always true since we derive from JWT)
	// (kept for compatibility with existing validation logic)

	// T008: bridge_state gate (sovereign pairs only).
	if h.bridgeChecker != nil && h.bridgeChecker.IsSovereignPair(req.PoolPair) {
		active, err := h.bridgeChecker.HasActiveBridgePosition(c.Context(), providerID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !active {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "no active bridge position found for this side — wait for Relayer confirmation",
				"code":  "BRIDGE_POSITION_NOT_ACTIVE",
			})
		}
	}

	// Parse amount once — reused by balance check and on-chain registration below.
	amtBig, amtOk := new(big.Int).SetString(req.Amount, 10)
	if !amtOk {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid amount format"})
	}

	// FR-010: pre-flight W-tCeBM balance check for sovereign pairs — prevents committing
	// more than what was actually minted via lock-mint, which would leave the pool partially
	// funded (token A succeeds, token B fails on-chain with insufficient balance).
	// Fixed: use localCBHubSigner (recipient of lock-mint tokens) instead of claims.Wallet.
	if h.sovereignSvc != nil && h.bridgeChecker != nil && h.bridgeChecker.IsSovereignPair(req.PoolPair) {
		isTokenA := side == "A"
		holderAddr := h.localCBHubSigner
		if holderAddr == "" {
			holderAddr = claims.Wallet // fallback for non-sovereign mode
		}
		if balErr := h.sovereignSvc.CheckSufficientWTokenBalance(c.Context(), req.PoolPair, isTokenA, holderAddr, amtBig); balErr != nil {
			if services.IsInsufficientBalance(balErr) {
				return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
					"error": "insufficient W-tCeBM balance — lock-mint at least the commit amount before committing",
					"code":  "INSUFFICIENT_BALANCE",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": balErr.Error()})
		}
	}

	// T010: register on-chain with LiquidityCommitRegistry (sovereign pairs).
	var onChainCommitID []byte
	if h.lcrRegistrar != nil && wTokenAddr != "" {
		sideUint := uint8(0) // CommitSide.A
		if side == "B" {
			sideUint = 1
		}
		id, err := h.lcrRegistrar.RegisterCommit(c.Context(), req.PoolPair, sideUint, amtBig, wTokenAddr)
		if err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "on-chain commit registration failed: " + err.Error(),
				"code":  "ON_CHAIN_COMMIT_FAILED",
			})
		}
		onChainCommitID = id
	}

	result, err := h.svc.RegisterCommit(c.Context(), services.CommitRequest{
		PoolPair:        req.PoolPair,
		ProviderID:      providerID,
		Side:            domain.CommitSide(side),
		Amount:          req.Amount,
		OnChainCommitID: onChainCommitID,
	})
	if err != nil {
		var ce *services.CommitError
		if errors.As(err, &ce) {
			status := fiber.StatusConflict
			if ce.Code == "NOT_AUTHORIZED_LP" {
				status = fiber.StatusForbidden
			}
			return c.Status(status).JSON(fiber.Map{"error": ce.Message, "error_code": ce.Code})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// ListCommits handles GET /api/v2/amm/liquidity/commits?pool_pair=BRL-USD&status=PENDING (T014 / FR-001).
func (h *LiquidityHandler) ListCommits(c *fiber.Ctx) error {
	poolPair := c.Query("pool_pair")
	if poolPair == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair query parameter required"})
	}
	status := c.Query("status")

	commits, err := h.svc.ListCommits(c.Context(), poolPair, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"commits": commits, "count": len(commits)})
}

// GetCommit handles GET /api/v2/amm/liquidity/commits/:commit_id.
// Returns a single commit's current lifecycle state so the front-end can poll commit
// progress without blocking — the commit lifecycle is driven to completion server-side
// (DB + on-chain registry + Cacti watcher) independent of any open browser session.
func (h *LiquidityHandler) GetCommit(c *fiber.Ctx) error {
	commitID := c.Params("commit_id")
	if commitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "commit_id required"})
	}
	commit, err := h.svc.GetCommit(c.Context(), commitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if commit == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "commit not found", "commit_id": commitID})
	}
	return c.JSON(commit)
}

// CancelCommit handles DELETE /api/v2/amm/liquidity/commits/:commit_id (T015 / FR-001).
// Marks a PENDING commit as EXPIRED. Returns 409 if already MATCHED.
func (h *LiquidityHandler) CancelCommit(c *fiber.Ctx) error {
	commitID := c.Params("commit_id")
	if commitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "commit_id required"})
	}

	// R2-H-9 / R2-H-10: derive the provider identity from the authenticated JWT
	// (claims.BankID), never from the query parameter — mirroring CommitLiquidity.
	// CB service-account tokens carry no BankID, so fall back to the configured
	// BANK_CODE; otherwise fail closed with 401. A divergent query provider_id is
	// logged and ignored, not trusted.
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}
	providerID := claims.BankID
	if providerID == "" {
		providerID = h.fallbackBankCode
		if providerID != "" {
			log.Printf("[liquidity] BankID missing in claims; using configured BANK_CODE fallback: %s", providerID)
		}
	}
	if providerID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}
	if q := c.Query("provider_id"); q != "" && q != providerID {
		log.Printf("[liquidity] deprecated provider_id query (%s) ignored; using authenticated identity %s", q, providerID)
	}

	if err := h.svc.CancelCommit(c.Context(), commitID, providerID); err != nil {
		var ce *services.CommitError
		if errors.As(err, &ce) {
			status := fiber.StatusConflict
			if ce.Code == "COMMIT_NOT_FOUND" {
				status = fiber.StatusNotFound
			}
			if ce.Code == "NOT_AUTHORIZED_LP" {
				status = fiber.StatusForbidden
			}
			return c.Status(status).JSON(fiber.Map{"error": ce.Message, "error_code": ce.Code})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "commit cancelled", "commit_id": commitID})
}

// ExecuteMatchedCommit handles POST /internal/amm/execute-matched-commit (T012 / FR-003).
// Called by the Cacti LiquidityCommitWatcher when a CommitMatched event is detected on the Hub.
// Determines which signer (A or B) belongs to this gateway and delegates to SovereignLiquidityService.
// Returns {"status":"ignored"} if neither signer matches LOCAL_CB_HUB_SIGNER.
func (h *LiquidityHandler) ExecuteMatchedCommit(c *fiber.Ctx) error {
	if h.sovereignSvc == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "sovereign liquidity service not configured",
		})
	}

	var req SovereignExecuteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" || req.CommitIDA == "" || req.CommitIDB == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pool_pair, commit_id_a, commit_id_b required",
		})
	}

	if err := h.sovereignSvc.ExecuteMatchedCommit(c.Context(), req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// SovereignAddLiquidity handles POST /api/v2/amm/liquidity/sovereign-add (T021 / FR-010).
// Allows a sovereign CB to directly deposit into an already ACTIVE sovereign pair pool
// without going through the commit-reveal / bridge gate flow.
// This is used for top-up deposits after the initial bilateral match.
// Body: { "pool_pair": "W-BRL-ARS", "amount": "1000000000000000000", "is_token_a": true }
func (h *LiquidityHandler) SovereignAddLiquidity(c *fiber.Ctx) error {
	if h.sovereignSvc == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "sovereign liquidity service not configured",
		})
	}

	var req struct {
		PoolPair string `json:"pool_pair"`
		Amount   string `json:"amount"`
		IsTokenA bool   `json:"is_token_a"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PoolPair == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pool_pair and amount required"})
	}

	if h.bridgeChecker != nil && !h.bridgeChecker.IsSovereignPair(req.PoolPair) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pool_pair is not a sovereign pair",
			"code":  "NOT_SOVEREIGN_PAIR",
		})
	}

	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.BankID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authenticated claims"})
	}

	sideStr := "B"
	if req.IsTokenA {
		sideStr = "A"
	}

	// T021 / FR-010: pre-flight balance check — reject before any on-chain tx if the CB
	// does not hold enough W-tCeBM in the Hub (returns HTTP 422 INSUFFICIENT_BALANCE).
	// Fixed: use localCBHubSigner (recipient of lock-mint tokens) instead of claims.Wallet.
	amtBig, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid amount format"})
	}
	holderAddr := h.localCBHubSigner
	if holderAddr == "" {
		holderAddr = claims.Wallet // fallback for non-sovereign mode
	}
	if balErr := h.sovereignSvc.CheckSufficientWTokenBalance(c.Context(), req.PoolPair, req.IsTokenA, holderAddr, amtBig); balErr != nil {
		if services.IsInsufficientBalance(balErr) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "insufficient W-tCeBM balance for direct deposit",
				"code":  "INSUFFICIENT_BALANCE",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": balErr.Error()})
	}

	// Use the hub signer (the address that signs on-chain txs) so ExecuteMatchedCommit
	// correctly identifies the local side. claims.Wallet is the entity's besu address
	// which may differ from the hub signer used for Hub transactions.
	hubSigner := h.localCBHubSigner
	if hubSigner == "" {
		hubSigner = claims.Wallet // fallback for non-sovereign mode
	}
	syntheticReq := SovereignExecuteRequest{
		PoolPair:  req.PoolPair,
		CommitIDA: "direct-deposit-" + claims.BankID,
		CommitIDB: "direct-deposit-" + claims.BankID,
		AmountA:   "0",
		AmountB:   "0",
	}
	if sideStr == "A" {
		syntheticReq.AmountA = req.Amount
		syntheticReq.SignerA = hubSigner
	} else {
		syntheticReq.AmountB = req.Amount
		syntheticReq.SignerB = hubSigner
	}

	if err := h.sovereignSvc.ExecuteMatchedCommit(c.Context(), syntheticReq); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":    "ok",
		"pool_pair": req.PoolPair,
		"amount":    req.Amount,
		"side":      sideStr,
	})
}

// ListPositions handles GET /api/v2/amm/liquidity/positions?pool_pair=... (FR-028 / 008-fix-cb-liquidity).
// Returns all ACTIVE LP positions for the specified pool pair.
// Optional query parameter `provider_id` filters to a specific provider.
func (h *LiquidityHandler) ListPositions(c *fiber.Ctx) error {
	if h.lpRepo == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "LP position listing not enabled",
			"code":  "NOT_IMPLEMENTED",
		})
	}

	poolPair := c.Query("pool_pair")
	if poolPair == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pool_pair query parameter is required",
			"code":  "INVALID_REQUEST",
		})
	}

	providerID := c.Query("provider_id") // optional filter
	var positions []domain.LiquidityPosition
	var err error

	if providerID != "" {
		positions, err = h.lpRepo.FindByProviderAndPoolPair(c.Context(), providerID, poolPair)
	} else {
		positions, err = h.lpRepo.FindByPoolPair(c.Context(), poolPair)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve LP positions",
			"code":  "DATABASE_ERROR",
		})
	}

	// Transform to DTO for API response
	type LPPositionDTO struct {
		LPID              string  `json:"lp_id"`
		ProviderBankID    string  `json:"provider_bank_id"`
		PoolPair          string  `json:"pool_pair"`
		TokenAContributed string  `json:"token_a_contributed"`
		TokenBContributed string  `json:"token_b_contributed"`
		LPShares          string  `json:"lp_shares"`
		Status            string  `json:"status"`
		DepositSide       string  `json:"deposit_side"`
		AddedAt           string  `json:"added_at"`
		CommitID          *string `json:"commit_id,omitempty"`
	}

	dtos := make([]LPPositionDTO, 0, len(positions))
	for _, pos := range positions {
		dtos = append(dtos, LPPositionDTO{
			LPID:              pos.LPID,
			ProviderBankID:    pos.ProviderBankID,
			PoolPair:          pos.PoolPair,
			TokenAContributed: pos.TokenAContributed,
			TokenBContributed: pos.TokenBContributed,
			LPShares:          pos.LPShares,
			Status:            string(pos.Status),
			DepositSide:       string(pos.DepositSide),
			AddedAt:           pos.AddedAt.Format("2006-01-02T15:04:05Z07:00"),
			CommitID:          pos.CommitID,
		})
	}

	return c.JSON(fiber.Map{
		"pool_pair": poolPair,
		"positions": dtos,
		"count":     len(dtos),
	})
}

// GetLPBalance handles GET /api/v2/amm/lp-balance (specs/013-amm-lp-shares).
// Returns this gateway's on-chain CBW3-LP position: the CB's LP-share balance, the pool's
// total supply, and the resulting ownership percentage. The balance is read live from the
// AMM contract (source of truth), not from the off-chain position cache.
func (h *LiquidityHandler) GetLPBalance(c *fiber.Ctx) error {
	if h.lpBalance == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "lp-balance reader not configured (AMM client unavailable)",
		})
	}

	balance, err := h.lpBalance.LPBalanceOf(c.Context(), "")
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "read LP balance: " + err.Error()})
	}
	supply, err := h.lpBalance.LPTotalSupply(c.Context())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "read LP total supply: " + err.Error()})
	}

	sharePct := 0.0
	if supply != nil && supply.Sign() > 0 {
		bal := new(big.Float).SetInt(balance)
		sup := new(big.Float).SetInt(supply)
		pct, _ := new(big.Float).Quo(bal, sup).Float64()
		sharePct = pct * 100
	}

	return c.JSON(fiber.Map{
		"lp_shares":        balance.String(),
		"lp_total_supply":  supply.String(),
		"share_percentage": sharePct,
		"holder":           h.localCBHubSigner,
	})
}
