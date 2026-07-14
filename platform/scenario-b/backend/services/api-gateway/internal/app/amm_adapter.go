// SPDX-License-Identifier: Apache-2.0

// Package app provides adapters that bridge the amm.Client to the service-layer interfaces.
package app

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	ammclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/amm"
	tcebmclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/tcebm"
	"github.com/ethereum/go-ethereum/crypto"
)

// commitIDForPair derives the shared escrow commit id from the pool_pair string, so both
// sovereign CBs deposit against the same id without a registry: keccak256(pool_pair).
func commitIDForPair(poolPair string) [32]byte {
	return crypto.Keccak256Hash([]byte(poolPair))
}

// ammAdapter satisfies the service interfaces (AMMQuoter, AMMPoolReader, AMMSwapper,
// AMLiquidityAdder, AMMCircuitBreakerCaller) by resolving the AMM client for a given
// pool_pair DYNAMICALLY from the on-chain PairRegistry (per corridor). There is no
// default/bootstrap AMM: pair-carrying methods resolve per pool_pair, and the few
// genuinely pairless legacy operations (LP-share reads, withdrawal, sovereign commit
// calls that pass their AMM address explicitly) fall back to the "primary" pool — the
// first active pair (see TD-001 / pairAMMResolver.primaryClient).
type ammAdapter struct {
	resolver *pairAMMResolver
}

// clientFor resolves the AMM client for pair. An empty pair resolves to the primary
// pool (for pairless legacy ops). Returns an error rather than a nil client so callers
// never dereference nil when no pool can be resolved (e.g. no corridor opened yet).
func (a *ammAdapter) clientFor(ctx context.Context, pair string) (*ammclient.Client, error) {
	if a.resolver == nil {
		return nil, fmt.Errorf("amm: no pair resolver configured")
	}
	if pair != "" {
		return a.resolver.ammFor(ctx, pair)
	}
	return a.resolver.primaryClient(ctx)
}

// --- AMMQuoter ---

func (a *ammAdapter) QuoteExactOutput(ctx context.Context, pair, amountOut string) (string, string, int64, error) {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return "", "", 0, err
	}
	res, err := c.QuoteExactOutput(ctx, pair, amountOut)
	if err != nil {
		return "", "", 0, err
	}
	return res.RequiredInput, res.PriceImpact, res.QuoteTimestamp, nil
}

// --- AMMPoolReader ---

func (a *ammAdapter) GetPoolReserves(ctx context.Context, pair string) (string, string, float64, error) {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return "", "", 0, err
	}
	rA, rB, err := c.Reserves(ctx)
	if err != nil {
		return "", "", 0, err
	}
	fA := new(big.Float).SetInt(rA)
	fB := new(big.Float).SetInt(rB)
	sum := new(big.Float).Add(fA, fB)
	var ratio float64
	if sum.Sign() > 0 {
		r, _ := new(big.Float).Quo(fA, sum).Float64()
		ratio = r
	}
	return rA.String(), rB.String(), ratio, nil
}

// GetFeeBps reads the swap fee (basis points) for pair (T018 / FR-005). All corridor
// AMMs are deployed with the same constructor, so the rate is uniform; the pair still
// selects the correct pool now that no default AMM exists.
func (a *ammAdapter) GetFeeBps(ctx context.Context, pair string) (uint64, error) {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return 0, err
	}
	bps, err := c.FeeBps(ctx)
	if err != nil {
		return 0, err
	}
	return bps.Uint64(), nil
}

// GetFeeBpsForPair reads the swap fee for a specific pair (009-commercial-cross-currency-swap).
func (a *ammAdapter) GetFeeBpsForPair(ctx context.Context, pair string) (uint16, error) {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return 0, err
	}
	bpsBig, err := c.FeeBps(ctx)
	if err != nil {
		return 0, err
	}
	bps := bpsBig.Uint64()
	// #nosec G115 -- fee is basis points, bounded to [0,10000] by the AMM contract; fits uint16.
	return uint16(bps), nil
}

// --- AMMSwapper ---

func (a *ammAdapter) SwapExactOutput(ctx context.Context, pair, amountOut, maxAmountIn, payerID, beneficiaryID, zkPayer, zkBeneficiary string) (string, string, string, error) {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return "", "", "", err
	}
	res, err := c.SwapExactOutput(ctx, ammclient.SwapRequest{
		AmountOut:            amountOut,
		MaxAmountIn:          maxAmountIn,
		PayerID:              payerID,
		BeneficiaryID:        beneficiaryID,
		ZKPointerPayer:       zkPayer,
		ZKPointerBeneficiary: zkBeneficiary,
	})
	if err != nil {
		return "", "", "", err
	}
	return res.OrderID, res.TxHash, res.AmountIn, nil
}

// --- AMLiquidityAdder ---

func (a *ammAdapter) AddLiquidity(ctx context.Context, pair, providerID, tokenAAmount, tokenBAmount string) (string, error) {
	amtA, ok := new(big.Int).SetString(tokenAAmount, 10)
	if !ok {
		return "", fmt.Errorf("invalid token_a_amount: %s", tokenAAmount)
	}
	amtB, ok := new(big.Int).SetString(tokenBAmount, 10)
	if !ok {
		return "", fmt.Errorf("invalid token_b_amount: %s", tokenBAmount)
	}
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return "", err
	}
	if _, err := c.AddLiquidity(ctx, amtA, amtB); err != nil {
		return "", err
	}

	// Calculate lpShares as the geometric mean of the contributed amounts (sqrt(A * B)).
	// This represents the provider's proportional share of the constant-product pool.
	// For constant-product AMM without minted LP tokens, this is a standard calculation.
	fA := new(big.Float).SetInt(amtA)
	fB := new(big.Float).SetInt(amtB)
	product := new(big.Float).Mul(fA, fB)
	lpShare := new(big.Float).Sqrt(product)

	// Round to nearest integer (banker's rounding) and convert to string.
	lpShares := new(big.Int)
	lpShare.Int(lpShares)

	if lpShares.Sign() <= 0 {
		// Fallback: if rounding failed, use the smaller amount as the share.
		if amtA.Cmp(amtB) < 0 {
			lpShares = amtA
		} else {
			lpShares = amtB
		}
	}

	return lpShares.String(), nil
}

// RemoveLiquidityShares burns `shares` LP tokens of the configured signer and returns a single
// (home) currency, zap-swapping the other side (decision D1). The signer must own the shares —
// withdrawal authority lives with the share owner now that shares are on-chain (D4).
// Pairless legacy op: operates on the primary pool (TD-001).
func (a *ammAdapter) RemoveLiquidityShares(ctx context.Context, shares *big.Int, homeIsTokenA bool, minAmountOut *big.Int) (string, error) {
	if shares == nil || shares.Sign() <= 0 {
		return "", fmt.Errorf("shares must be positive")
	}
	if minAmountOut == nil {
		minAmountOut = big.NewInt(0)
	}
	c, err := a.clientFor(ctx, "")
	if err != nil {
		return "", err
	}
	amountOut, _, err := c.RemoveLiquidity(ctx, shares, homeIsTokenA, minAmountOut)
	if err != nil {
		return "", err
	}
	if amountOut == nil {
		// Burn succeeded but LogLiquidityRemoved was not decodable — never report a fake amount.
		return "0", nil
	}
	return amountOut.String(), nil
}

// LPBalanceOf returns the on-chain LP-share balance of holder (the source of truth for ownership).
// An empty holder resolves to the configured signer (the CB itself on a sovereign gateway, D4).
// Pairless legacy op: reads the primary pool (TD-001).
func (a *ammAdapter) LPBalanceOf(ctx context.Context, holder string) (*big.Int, error) {
	c, err := a.clientFor(ctx, "")
	if err != nil {
		return nil, err
	}
	return c.LPBalanceOf(ctx, holder)
}

// LPTotalSupply returns the total LP-share supply of the primary pool (pairless legacy op, TD-001).
func (a *ammAdapter) LPTotalSupply(ctx context.Context) (*big.Int, error) {
	c, err := a.clientFor(ctx, "")
	if err != nil {
		return nil, err
	}
	return c.LPTotalSupply(ctx)
}

// DepositForCommitAt escrows one side against a commit on an arbitrary AMM (sovereign flow).
// The AMM address is passed explicitly, so any resolved client provides the RPC/signer.
func (a *ammAdapter) DepositForCommitAt(ctx context.Context, ammAddress string, commitID [32]byte, isTokenA bool, amount *big.Int, shareRecipient string) error {
	c, err := a.clientFor(ctx, "")
	if err != nil {
		return err
	}
	return c.DepositForCommitAt(ctx, ammAddress, commitID, isTokenA, amount, shareRecipient)
}

// FinalizeCommitAt finalizes a commit on an arbitrary AMM (sovereign). Returns the minted share
// split (sharesA, sharesB) decoded from LogCommitFinalized on success; the error is returned
// verbatim so callers can distinguish "incomplete" (other side pending) from real failures.
func (a *ammAdapter) FinalizeCommitAt(ctx context.Context, ammAddress string, commitID [32]byte) (*big.Int, *big.Int, error) {
	c, err := a.clientFor(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	res, err := c.FinalizeCommitAt(ctx, ammAddress, commitID)
	if err != nil {
		return nil, nil, err
	}
	return res.SharesA, res.SharesB, nil
}

// TokenBalanceAt reads the ERC-20 balance of holderAddr for the token (A or B) of ammAddress.
// Used by SovereignAddLiquidity pre-flight check (FR-010 / T021).
func (a *ammAdapter) TokenBalanceAt(ctx context.Context, ammAddress string, isTokenA bool, holderAddr string) (*big.Int, error) {
	c, err := a.clientFor(ctx, "")
	if err != nil {
		return nil, err
	}
	return c.TokenBalanceAt(ctx, ammAddress, isTokenA, holderAddr)
}

// --- Sovereign per-pair escrow seed (simplified commit-reveal, no LCR) ---
// Each CB deposits ONLY its own side against the deterministic commit id
// keccak256(pool_pair); finalize funds reserves atomically once both sides are in;
// a CB can reclaim its own pending side before finalize. Side is auto-resolved.

// DepositSideForCommit escrows the caller CB's own side of poolPair (the token it is the
// central bank of) against the shared commit id. The W-token must already be minted to
// the signer (see AMMTokenPreparer). Returns the resolved side ("A"/"B").
func (a *ammAdapter) DepositSideForCommit(ctx context.Context, poolPair string, amount *big.Int) (string, error) {
	if a.resolver == nil {
		return "", fmt.Errorf("amm: no pair resolver configured")
	}
	side, err := a.resolver.SideForSigner(ctx, poolPair)
	if err != nil {
		return "", err
	}
	ammAddr, err := a.resolver.ammAddressFor(ctx, poolPair)
	if err != nil {
		return "", err
	}
	c, err := a.clientFor(ctx, poolPair)
	if err != nil {
		return "", err
	}
	if err := c.DepositForCommitAt(ctx, ammAddr, commitIDForPair(poolPair), side == "A", amount, ""); err != nil {
		return "", err
	}
	return side, nil
}

// FinalizeCommitForPair finalizes the escrow for poolPair once both sides are deposited,
// funding the pool reserves and minting LP shares to each side's depositor.
func (a *ammAdapter) FinalizeCommitForPair(ctx context.Context, poolPair string) (*ammclient.FinalizeResult, error) {
	if a.resolver == nil {
		return nil, fmt.Errorf("amm: no pair resolver configured")
	}
	ammAddr, err := a.resolver.ammAddressFor(ctx, poolPair)
	if err != nil {
		return nil, err
	}
	c, err := a.clientFor(ctx, poolPair)
	if err != nil {
		return nil, err
	}
	return c.FinalizeCommitAt(ctx, ammAddr, commitIDForPair(poolPair))
}

// CancelSideForCommit reclaims the caller CB's own pending side of poolPair (before finalize).
func (a *ammAdapter) CancelSideForCommit(ctx context.Context, poolPair string) (string, error) {
	if a.resolver == nil {
		return "", fmt.Errorf("amm: no pair resolver configured")
	}
	side, err := a.resolver.SideForSigner(ctx, poolPair)
	if err != nil {
		return "", err
	}
	ammAddr, err := a.resolver.ammAddressFor(ctx, poolPair)
	if err != nil {
		return "", err
	}
	c, err := a.clientFor(ctx, poolPair)
	if err != nil {
		return "", err
	}
	if err := c.CancelCommitDepositAt(ctx, ammAddr, commitIDForPair(poolPair), side == "A"); err != nil {
		return "", err
	}
	return side, nil
}

// GetCommitEscrow reads the escrow state for poolPair's shared commit id (for UI status).
func (a *ammAdapter) GetCommitEscrow(ctx context.Context, poolPair string) (*ammclient.EscrowState, error) {
	if a.resolver == nil {
		return nil, fmt.Errorf("amm: no pair resolver configured")
	}
	ammAddr, err := a.resolver.ammAddressFor(ctx, poolPair)
	if err != nil {
		return nil, err
	}
	c, err := a.clientFor(ctx, poolPair)
	if err != nil {
		return nil, err
	}
	return c.GetEscrowAt(ctx, ammAddr, commitIDForPair(poolPair))
}

// --- AMMCircuitBreakerCaller ---
// Circuit-breaker operations target the specific pool's AMM (per-pair). The service
// passes the pool_pair through from the governance request.

func (a *ammAdapter) PauseCircuitBreaker(ctx context.Context, pair string, signature []byte) (string, error) {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return "", err
	}
	reason := string(signature)
	if reason == "" {
		reason = "governance emergency pause"
	}
	return c.PauseCircuitBreaker(ctx, reason)
}

func (a *ammAdapter) ProposeResume(ctx context.Context, pair string, sig []byte) (string, error) {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return "", err
	}
	return c.ProposeResume(ctx)
}

func (a *ammAdapter) SignResume(ctx context.Context, pair, requestID string, sig []byte) error {
	c, err := a.clientFor(ctx, pair)
	if err != nil {
		return err
	}
	var proposalID [32]byte
	trimmed := strings.TrimPrefix(requestID, "0x")
	b, err := hex.DecodeString(trimmed)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("amm: invalid proposalId %q: %w", requestID, err)
	}
	copy(proposalID[:], b)
	_, err = c.SignResume(ctx, proposalID)
	return err
}

func (a *ammAdapter) ExecuteResume(ctx context.Context, pair, requestID string) error {
	// The AMM auto-unpauses atomically in signResume when quorum is met.
	// ExecuteResume is a no-op at the on-chain level.
	return nil
}

// --- tokenPrepareAdapter ---

// tokenPrepareAdapter implements the AMMTokenPreparer interface used by the token
// handler to mint Hub tCeBM tokens to the signer's own address and approve the AMM
// contract to spend them — a prerequisite for addLiquidity.
//
// Per-pair by default: when a caller passes a pool_pair, the pair's W-tokens + AMM are
// resolved on-chain from the PairRegistry. The env-configured tokenA/tokenB clients are
// retained only to detect which side the signer is the central bank of (CENTRAL_BANK_ROLE)
// and to serve the legacy empty-pool_pair mint path on single-pair CB gateways.
type tokenPrepareAdapter struct {
	tokenA  *tcebmclient.Client
	tokenB  *tcebmclient.Client
	sideIsA bool // true → signer is issuer of TOKEN_A; false → TOKEN_B
	isCB    bool // true → signer holds CENTRAL_BANK_ROLE on some token
	// resolver enables DYNAMIC per-pair mint/approve: when a caller passes a
	// non-empty pool_pair, the pair's W-tokens + AMM are resolved on-chain from
	// the PairRegistry (there is no env-configured default AMM anymore).
	resolver *pairAMMResolver
}

// tokenForPair resolves the (token client, amm address) for poolPair + side via the
// on-chain PairRegistry. The legacy empty-pool_pair path (single-pair CB gateways) picks
// the side the signer is CB of; it has no AMM address to approve since no default AMM
// exists, so callers on that path can only mint-to (not approve).
func (a *tokenPrepareAdapter) tokenForPair(ctx context.Context, poolPair, side string) (*tcebmclient.Client, string, error) {
	if poolPair != "" && a.resolver != nil {
		tokA, tokB, err := a.resolver.tokensFor(ctx, poolPair)
		if err != nil {
			return nil, "", err
		}
		ammAddr, err := a.resolver.ammAddressFor(ctx, poolPair)
		if err != nil {
			return nil, "", err
		}
		// Auto-derive the side from the pair when the caller does not pin one: a CB
		// operates only its own currency, so the side is the token it is the central
		// bank of (on-chain CENTRAL_BANK_ROLE). No manual side picker needed.
		if side == "" {
			side, err = a.resolver.SideForSigner(ctx, poolPair)
			if err != nil {
				return nil, "", err
			}
		}
		switch side {
		case "B":
			return tokB, ammAddr, nil
		case "A":
			return tokA, ammAddr, nil
		default:
			return nil, "", fmt.Errorf("token_prepare: side must be 'A' or 'B', got %q", side)
		}
	}
	// Legacy single-pair mint (no pool_pair): pick the side the signer is CB of. There is
	// no default AMM address to approve, so this path supports mint-to only.
	if !a.isCB {
		return nil, "", fmt.Errorf("token_prepare: signer has no CENTRAL_BANK_ROLE on configured tokens")
	}
	tok := a.tokenB
	if a.sideIsA {
		tok = a.tokenA
	}
	return tok, "", nil
}

// NewTokenPrepareAdapter constructs a tokenPrepareAdapter and detects which token
// the signer is the central bank of by calling HasCentralBankRole on both contracts.
func NewTokenPrepareAdapter(ctx context.Context, tokenA, tokenB *tcebmclient.Client, resolver *pairAMMResolver) (*tokenPrepareAdapter, error) {
	isA, err := tokenA.HasCentralBankRole(ctx)
	if err != nil {
		return nil, fmt.Errorf("token_prepare: check CENTRAL_BANK_ROLE on TOKEN_A: %w", err)
	}
	isB, err := tokenB.HasCentralBankRole(ctx)
	if err != nil {
		return nil, fmt.Errorf("token_prepare: check CENTRAL_BANK_ROLE on TOKEN_B: %w", err)
	}
	return &tokenPrepareAdapter{
		tokenA:   tokenA,
		tokenB:   tokenB,
		sideIsA:  isA,
		isCB:     isA || isB,
		resolver: resolver,
	}, nil
}

// MintAndApproveForAMM mints `amount` of poolPair's `side` token to the signer,
// then approves that pair's AMM to spend it. (FR-018; dynamic per-pair)
func (a *tokenPrepareAdapter) MintAndApproveForAMM(ctx context.Context, poolPair, side, amount string) error {
	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return fmt.Errorf("token_prepare: invalid amount %q", amount)
	}
	if amt.Sign() <= 0 {
		return nil
	}
	tok, ammAddr, err := a.tokenForPair(ctx, poolPair, side)
	if err != nil {
		return err
	}
	if ammAddr == "" {
		return fmt.Errorf("token_prepare: a pool_pair is required to approve an AMM (no default AMM)")
	}
	signerAddr := tok.SignerAddress()
	if signerAddr == "" {
		return fmt.Errorf("token_prepare: signer key not configured")
	}
	if _, err := tok.Mint(ctx, signerAddr, amt); err != nil {
		return fmt.Errorf("token_prepare: mint: %w", err)
	}
	if _, err := tok.Approve(ctx, ammAddr, amt); err != nil {
		return fmt.Errorf("token_prepare: approve: %w", err)
	}
	return nil
}

// MintToForAMM mints `amount` of poolPair's `side` token directly to recipient.
// The recipient must call approve-amm separately. (FR-018)
func (a *tokenPrepareAdapter) MintToForAMM(ctx context.Context, poolPair, side, recipient, amount string) error {
	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return fmt.Errorf("token_prepare: invalid amount %q", amount)
	}
	if amt.Sign() <= 0 {
		return nil
	}
	tok, _, err := a.tokenForPair(ctx, poolPair, side)
	if err != nil {
		return err
	}
	if _, err := tok.Mint(ctx, recipient, amt); err != nil {
		return fmt.Errorf("token_prepare: mint to %s: %w", recipient, err)
	}
	return nil
}

// ApproveAMM approves poolPair's AMM to spend `amount` of the token indicated by
// side ("A"/"B", or "" to auto-detect on legacy single-pair CBs). (FR-018)
func (a *tokenPrepareAdapter) ApproveAMM(ctx context.Context, poolPair, amount, side string) error {
	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return fmt.Errorf("token_prepare: invalid amount %q", amount)
	}
	// Legacy single-pair: preserve the "side required for non-CB" contract.
	if poolPair == "" || a.resolver == nil {
		if side == "" && !a.isCB {
			return fmt.Errorf("token_prepare: side is required for non-central-bank callers")
		}
		if side != "" && side != "A" && side != "B" {
			return fmt.Errorf("token_prepare: side must be 'A' or 'B', got %q", side)
		}
	}
	tok, ammAddr, err := a.tokenForPair(ctx, poolPair, side)
	if err != nil {
		return err
	}
	if ammAddr == "" {
		return fmt.Errorf("token_prepare: a pool_pair is required to approve an AMM (no default AMM)")
	}
	if _, err := tok.Approve(ctx, ammAddr, amt); err != nil {
		return fmt.Errorf("token_prepare: approve: %w", err)
	}
	return nil
}
