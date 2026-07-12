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
)

// ammAdapter wraps amm.Client to satisfy the service interfaces:
// AMMQuoter, AMMPoolReader, AMMSwapper, AMLiquidityAdder, AMMCircuitBreakerCaller.
//
// When resolver != nil, pair-carrying methods resolve the AMM client DYNAMICALLY
// from the on-chain PairRegistry (per pool_pair), so a corridor opened at runtime
// works without gateway env/restart. `c` is the fallback (single-pair) client used
// by pairless methods and when the resolver can't resolve a pair.
type ammAdapter struct {
	c        *ammclient.Client
	resolver *pairAMMResolver
}

// clientFor resolves the AMM client for pair (dynamic per-pair), falling back to
// the configured default client when no resolver is wired or the pair is absent.
func (a *ammAdapter) clientFor(ctx context.Context, pair string) *ammclient.Client {
	if a.resolver != nil && pair != "" {
		if c, err := a.resolver.ammFor(ctx, pair); err == nil && c != nil {
			return c
		}
	}
	return a.c
}

// --- AMMQuoter ---

func (a *ammAdapter) QuoteExactOutput(ctx context.Context, pair, amountOut string) (string, string, int64, error) {
	res, err := a.clientFor(ctx, pair).QuoteExactOutput(ctx, pair, amountOut)
	if err != nil {
		return "", "", 0, err
	}
	return res.RequiredInput, res.PriceImpact, res.QuoteTimestamp, nil
}

// --- AMMPoolReader ---

func (a *ammAdapter) GetPoolReserves(ctx context.Context, pair string) (string, string, float64, error) {
	rA, rB, err := a.clientFor(ctx, pair).Reserves(ctx)
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

// GetFeeBps reads the current swap fee rate from the AMM contract (T018 / FR-005).
func (a *ammAdapter) GetFeeBps(ctx context.Context) (uint64, error) {
	bps, err := a.c.FeeBps(ctx)
	if err != nil {
		return 0, err
	}
	return bps.Uint64(), nil
}

// GetFeeBpsForPair reads the swap fee for a specific pair (009-commercial-cross-currency-swap).
// Currently returns the global fee (single-pair AMM), but signature supports future multi-pair.
func (a *ammAdapter) GetFeeBpsForPair(ctx context.Context, pair string) (uint16, error) {
	bpsBig, err := a.clientFor(ctx, pair).FeeBps(ctx)
	if err != nil {
		return 0, err
	}
	bps := bpsBig.Uint64()
	// #nosec G115 -- fee is basis points, bounded to [0,10000] by the AMM contract; fits uint16.
	return uint16(bps), nil
}

// --- AMMSwapper ---

func (a *ammAdapter) SwapExactOutput(ctx context.Context, pair, amountOut, maxAmountIn, payerID, beneficiaryID, zkPayer, zkBeneficiary string) (string, string, string, error) {
	res, err := a.clientFor(ctx, pair).SwapExactOutput(ctx, ammclient.SwapRequest{
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
	if _, err := a.clientFor(ctx, pair).AddLiquidity(ctx, amtA, amtB); err != nil {
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
func (a *ammAdapter) RemoveLiquidityShares(ctx context.Context, shares *big.Int, homeIsTokenA bool, minAmountOut *big.Int) (string, error) {
	if shares == nil || shares.Sign() <= 0 {
		return "", fmt.Errorf("shares must be positive")
	}
	if minAmountOut == nil {
		minAmountOut = big.NewInt(0)
	}
	amountOut, _, err := a.c.RemoveLiquidity(ctx, shares, homeIsTokenA, minAmountOut)
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
func (a *ammAdapter) LPBalanceOf(ctx context.Context, holder string) (*big.Int, error) {
	return a.c.LPBalanceOf(ctx, holder)
}

// LPTotalSupply returns the total LP-share supply of the configured AMM pool.
func (a *ammAdapter) LPTotalSupply(ctx context.Context) (*big.Int, error) {
	return a.c.LPTotalSupply(ctx)
}

// DepositForCommitAt escrows one side against a commit on an arbitrary AMM (sovereign flow).
func (a *ammAdapter) DepositForCommitAt(ctx context.Context, ammAddress string, commitID [32]byte, isTokenA bool, amount *big.Int, shareRecipient string) error {
	return a.c.DepositForCommitAt(ctx, ammAddress, commitID, isTokenA, amount, shareRecipient)
}

// FinalizeCommitAt finalizes a commit on an arbitrary AMM (sovereign). Returns the minted share
// split (sharesA, sharesB) decoded from LogCommitFinalized on success; the error is returned
// verbatim so callers can distinguish "incomplete" (other side pending) from real failures.
func (a *ammAdapter) FinalizeCommitAt(ctx context.Context, ammAddress string, commitID [32]byte) (*big.Int, *big.Int, error) {
	res, err := a.c.FinalizeCommitAt(ctx, ammAddress, commitID)
	if err != nil {
		return nil, nil, err
	}
	return res.SharesA, res.SharesB, nil
}

// TokenBalanceAt reads the ERC-20 balance of holderAddr for the token (A or B) of ammAddress.
// Used by SovereignAddLiquidity pre-flight check (FR-010 / T021).
func (a *ammAdapter) TokenBalanceAt(ctx context.Context, ammAddress string, isTokenA bool, holderAddr string) (*big.Int, error) {
	return a.c.TokenBalanceAt(ctx, ammAddress, isTokenA, holderAddr)
}

// --- AMMCircuitBreakerCaller ---

func (a *ammAdapter) PauseCircuitBreaker(ctx context.Context, signature []byte) (string, error) {
	reason := string(signature)
	if reason == "" {
		reason = "governance emergency pause"
	}
	return a.c.PauseCircuitBreaker(ctx, reason)
}

func (a *ammAdapter) ProposeResume(ctx context.Context, sig []byte) (string, error) {
	return a.c.ProposeResume(ctx)
}

func (a *ammAdapter) SignResume(ctx context.Context, requestID string, sig []byte) error {
	var proposalID [32]byte
	trimmed := strings.TrimPrefix(requestID, "0x")
	b, err := hex.DecodeString(trimmed)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("amm: invalid proposalId %q: %w", requestID, err)
	}
	copy(proposalID[:], b)
	_, err = a.c.SignResume(ctx, proposalID)
	return err
}

func (a *ammAdapter) ExecuteResume(ctx context.Context, requestID string) error {
	// The AMM auto-unpauses atomically in signResume when quorum is met.
	// ExecuteResume is a no-op at the on-chain level.
	return nil
}

// --- tokenPrepareAdapter ---

// tokenPrepareAdapter implements the AMMTokenPreparer interface used by the
// token handler to mint Hub tCeBM tokens to the signer's own address and approve
// the AMM contract to spend them — a required prerequisite for addLiquidity.
//
// Design constraint — one-gateway-one-pair:
// Each api-gateway deployment is configured for exactly ONE token pair via
// HUB_TOKEN_A_ADDRESS / HUB_TOKEN_B_ADDRESS / AMM_CONTRACT_ADDRESS env vars.
// There is no runtime routing by pool_pair at the adapter layer. To support a
// second pair (e.g. BRL-EUR), deploy a separate gateway instance with its own
// env configuration pointing to the corresponding token and AMM contracts.
// The pool_pair field in the domain model (PoolCommit, LiquidityPosition, etc.)
// is a DB-level label for filtering; it does NOT drive contract address selection.
//
// FR-018 — single-amount payload:
// sideIsA is true when the signer holds CENTRAL_BANK_ROLE on TOKEN_A, false for TOKEN_B.
// isCB is true when the signer holds CENTRAL_BANK_ROLE on at least one configured token.
// Both fields are populated once at construction via HasCentralBankRole (view call, no gas).
type tokenPrepareAdapter struct {
	tokenA  *tcebmclient.Client
	tokenB  *tcebmclient.Client
	ammAddr string
	sideIsA bool // true → signer is issuer of TOKEN_A; false → TOKEN_B
	isCB    bool // true → signer holds CENTRAL_BANK_ROLE on some token
	// resolver enables DYNAMIC per-pair mint/approve: when a caller passes a
	// non-empty pool_pair, the pair's W-tokens + AMM are resolved on-chain from
	// the PairRegistry instead of the env-configured single pair.
	resolver *pairAMMResolver
}

// tokenForPair resolves the (token client, amm address) for poolPair + side via
// the on-chain PairRegistry. Falls back to the legacy env-configured single pair
// when poolPair is empty or no resolver is wired.
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
		switch side {
		case "B":
			return tokB, ammAddr, nil
		case "A", "":
			return tokA, ammAddr, nil
		default:
			return nil, "", fmt.Errorf("token_prepare: side must be 'A' or 'B', got %q", side)
		}
	}
	// Legacy single-pair: pick the side the signer is CB of.
	if !a.isCB {
		return nil, "", fmt.Errorf("token_prepare: signer has no CENTRAL_BANK_ROLE on configured tokens")
	}
	tok := a.tokenB
	if a.sideIsA {
		tok = a.tokenA
	}
	return tok, a.ammAddr, nil
}

// NewTokenPrepareAdapter constructs a tokenPrepareAdapter and detects which token
// the signer is the central bank of by calling HasCentralBankRole on both contracts.
func NewTokenPrepareAdapter(ctx context.Context, tokenA, tokenB *tcebmclient.Client, ammAddr string, resolver *pairAMMResolver) (*tokenPrepareAdapter, error) {
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
		ammAddr:  ammAddr,
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
	if _, err := tok.Approve(ctx, ammAddr, amt); err != nil {
		return fmt.Errorf("token_prepare: approve: %w", err)
	}
	return nil
}
