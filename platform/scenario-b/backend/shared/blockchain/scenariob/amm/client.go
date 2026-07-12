// SPDX-License-Identifier: Apache-2.0

// Package amm provides an EVM client for the AutomatedMarketMaker Hub contract.
// It wraps go-ethereum/ethclient with a minimal embedded ABI so the API Gateway and
// Payment Orchestrator can call read-only methods (getAmountIn/isPaused/reserves) and
// submit state-changing transactions (swap, pause, proposeResume, signResume) without
// requiring abigen-generated bindings during build (T035 / T086).
package amm

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// erc20ABI is the minimal ERC-20 ABI for approve(), allowance(), and balanceOf().
// approve() is required before safeTransferFrom; allowance() lets ensureUnlimitedApproval
// skip re-approvals that are already in effect from a previous run; balanceOf() is used
// for pre-flight balance checks in SovereignAddLiquidity (FR-010 / T021).
const erc20ABI = `[{"type":"function","name":"approve","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},{"type":"function","name":"allowance","stateMutability":"view","inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},{"type":"function","name":"balanceOf","stateMutability":"view","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]}]`

// approvalFloor is the minimum allowance below which ensureUnlimitedApproval submits a
// new approve(max_uint256). 2^128 is effectively unlimited for any realistic token supply
// and lets the service survive a restart without re-approving when the prior allowance
// is still large.
var approvalFloor = new(big.Int).Lsh(big.NewInt(1), 128)

// maxUint256 is the conventional "unlimited" ERC-20 allowance.
var maxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// ABIJSON is the minimal IAutomatedMarketMaker ABI used by the Go client. Keep the method
// signatures in sync with contracts/src/interfaces/IAutomatedMarketMaker.sol.
const ABIJSON = `[
{"type":"function","name":"reserveA","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"reserveB","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"isPaused","stateMutability":"view","inputs":[],"outputs":[{"type":"bool"}]},
{"type":"function","name":"resumeQuorum","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"resumeSignatures","stateMutability":"view","inputs":[{"type":"bytes32"}],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"TOKEN_A","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"address"}]},
{"type":"function","name":"TOKEN_B","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"address"}]},
{"type":"function","name":"balanceOf","stateMutability":"view","inputs":[{"name":"account","type":"address"}],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"totalSupply","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"getAmountIn","stateMutability":"pure","inputs":[
  {"name":"reserveIn","type":"uint256"},
  {"name":"reserveOut","type":"uint256"},
  {"name":"amountOut","type":"uint256"}
],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"getAmountOut","stateMutability":"pure","inputs":[
  {"name":"amountIn","type":"uint256"},
  {"name":"reserveIn","type":"uint256"},
  {"name":"reserveOut","type":"uint256"},
  {"name":"feeBps_","type":"uint256"}
],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"swapTokensForExactTokens","stateMutability":"nonpayable","inputs":[
  {"name":"tokenIn","type":"address"},
  {"name":"tokenOut","type":"address"},
  {"name":"amountOut","type":"uint256"},
  {"name":"maxAmountIn","type":"uint256"},
  {"name":"to","type":"address"}
],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"addLiquidity","stateMutability":"nonpayable","inputs":[
  {"name":"amountA","type":"uint256"},
  {"name":"amountB","type":"uint256"}
],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"removeLiquidity","stateMutability":"nonpayable","inputs":[
  {"name":"shares","type":"uint256"},
  {"name":"tokenOut","type":"address"},
  {"name":"minAmountOut","type":"uint256"}
],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"removeLiquidityEmergency","stateMutability":"nonpayable","inputs":[
  {"name":"shares","type":"uint256"}
],"outputs":[{"type":"uint256"},{"type":"uint256"}]},
{"type":"function","name":"depositForCommit","stateMutability":"nonpayable","inputs":[
  {"name":"commitId","type":"bytes32"},
  {"name":"isTokenA","type":"bool"},
  {"name":"amount","type":"uint256"},
  {"name":"shareRecipient","type":"address"}
],"outputs":[]},
{"type":"function","name":"finalizeCommit","stateMutability":"nonpayable","inputs":[
  {"name":"commitId","type":"bytes32"}
],"outputs":[{"type":"uint256"},{"type":"uint256"}]},
{"type":"function","name":"cancelCommitDeposit","stateMutability":"nonpayable","inputs":[
  {"name":"commitId","type":"bytes32"},
  {"name":"isTokenA","type":"bool"}
],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"getEscrow","stateMutability":"view","inputs":[{"name":"commitId","type":"bytes32"}],"outputs":[
  {"name":"depositorA","type":"address"},
  {"name":"depositorB","type":"address"},
  {"name":"recipientA","type":"address"},
  {"name":"recipientB","type":"address"},
  {"name":"amountA","type":"uint256"},
  {"name":"amountB","type":"uint256"},
  {"name":"finalized","type":"bool"}
]},
{"type":"function","name":"feeBps","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"withdrawalFeeBps","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"setFeeBps","stateMutability":"nonpayable","inputs":[
  {"name":"newFeeBps","type":"uint256"}
],"outputs":[]},
{"type":"function","name":"setWithdrawalFeeBps","stateMutability":"nonpayable","inputs":[
  {"name":"newWithdrawalFeeBps","type":"uint256"}
],"outputs":[]},
{"type":"function","name":"pause","stateMutability":"nonpayable","inputs":[
  {"name":"reason","type":"string"}
],"outputs":[]},
{"type":"function","name":"proposeResume","stateMutability":"nonpayable","inputs":[],"outputs":[{"type":"bytes32"}]},
{"type":"function","name":"signResume","stateMutability":"nonpayable","inputs":[
  {"name":"proposalId","type":"bytes32"}
],"outputs":[]}
]`

// QuoteResult holds the result of a quote-exact-output call.
type QuoteResult struct {
	RequiredInput  string
	PriceImpact    string
	QuoteTimestamp int64
}

// SwapRequest carries parameters for a swap-exact-output call.
type SwapRequest struct {
	TokenIn              string
	TokenOut             string
	AmountOut            string
	MaxAmountIn          string
	To                   string
	PayerID              string
	BeneficiaryID        string
	ZKPointerPayer       string
	ZKPointerBeneficiary string
}

// SwapResult holds the result of a swap execution.
type SwapResult struct {
	TxHash   string
	AmountIn string
	OrderID  string
}

// Client is the EVM client for the AMM contract on the Hub. All methods are safe for
// concurrent use; the embedded ABI is immutable.
type Client struct {
	contract   common.Address
	ec         *ethclient.Client
	abi        abi.ABI
	erc20ABI   abi.ABI
	signer     *evm.Signer
	timeout    time.Duration
	tokenA     common.Address
	tokenB     common.Address
	approvalMu sync.Mutex
	approved   map[common.Address]bool // tokens with an in-effect unlimited allowance
}

// Config holds the connection parameters for the AMM client.
type Config struct {
	RPCURL          string
	ContractAddress string
	ChainID         int64
	PrivateKeyHex   string // optional: required only for state-changing calls
	Timeout         time.Duration
	TokenAAddress   string // Hub tCeBM-A token contract (required for swap)
	TokenBAddress   string // Hub tCeBM-B token contract (required for swap)
}

// NewClient opens an RPC connection and parses the ABI. If PrivateKeyHex is empty the client
// remains read-only.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.ContractAddress == "" {
		return nil, errors.New("ContractAddress is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, cfg.RPCURL, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	parsed, err := evm.ParseABI(ABIJSON)
	if err != nil {
		return nil, fmt.Errorf("parse AMM ABI: %w", err)
	}
	erc20Parsed, err := evm.ParseABI(erc20ABI)
	if err != nil {
		return nil, fmt.Errorf("parse ERC20 ABI: %w", err)
	}
	client := &Client{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		abi:      parsed,
		erc20ABI: erc20Parsed,
		timeout:  cfg.Timeout,
		tokenA:   common.HexToAddress(cfg.TokenAAddress),
		tokenB:   common.HexToAddress(cfg.TokenBAddress),
		approved: make(map[common.Address]bool),
	}
	if cfg.PrivateKeyHex != "" {
		signer, err := evm.NewSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
		if err != nil {
			return nil, err
		}
		client.signer = signer
	}
	// Eagerly resolve token addresses so AddLiquidity/Swap can approve them.
	_ = evm.Call(ctx, client.ec, client.contract, client.abi, "TOKEN_A", nil, &client.tokenA)
	_ = evm.Call(ctx, client.ec, client.contract, client.abi, "TOKEN_B", nil, &client.tokenB)
	return client, nil
}

// Close releases the underlying RPC connection.
func (c *Client) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// Reserves fetches (reserveA, reserveB) from the Hub AMM.
func (c *Client) Reserves(ctx context.Context) (*big.Int, *big.Int, error) {
	a := new(big.Int)
	b := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "reserveA", nil, a); err != nil {
		return nil, nil, err
	}
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "reserveB", nil, b); err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

// IsPaused reports whether the AMM is currently paused (SC-017).
func (c *Client) IsPaused(ctx context.Context) (bool, error) {
	var paused bool
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "isPaused", nil, &paused); err != nil {
		return false, err
	}
	return paused, nil
}

// ResumeQuorum returns the number of signatures required to resume (2-of-N).
func (c *Client) ResumeQuorum(ctx context.Context) (*big.Int, error) {
	out := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "resumeQuorum", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ResumeSignatures returns how many signatures a proposal has collected so far.
func (c *Client) ResumeSignatures(ctx context.Context, proposalID [32]byte) (*big.Int, error) {
	out := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "resumeSignatures", []interface{}{proposalID}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// QuoteExactOutput retrieves the required input amount for an exact-output swap. The
// `pair` parameter is used only for bookkeeping; the on-chain formula uses reserves.
// Returns the grossAmountIn (with fee-in-reserve applied) so callers can use it
// directly as max_amount_in without triggering AMM__SlippageExceeded.
func (c *Client) QuoteExactOutput(ctx context.Context, pair, amountOut string) (*QuoteResult, error) {
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amountOut), 10)
	if !ok {
		return nil, fmt.Errorf("invalid amountOut %q", amountOut)
	}
	reserveIn, reserveOut, err := c.Reserves(ctx)
	if err != nil {
		return nil, err
	}
	amountIn := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "getAmountIn",
		[]interface{}{reserveIn, reserveOut, amt}, amountIn); err != nil {
		return nil, err
	}
	// Apply the same fee-in-reserve formula as swapTokensForExactTokens:
	// grossAmountIn = (amountIn * 10000) / (10000 - feeBps) + 1
	// so the returned value can be used directly as max_amount_in.
	feeBps, err := c.FeeBps(ctx)
	if err != nil {
		return nil, fmt.Errorf("get feeBps for quote: %w", err)
	}
	divisor := new(big.Int).Sub(big.NewInt(10000), feeBps)
	grossAmountIn := new(big.Int).Mul(amountIn, big.NewInt(10000))
	grossAmountIn.Div(grossAmountIn, divisor)
	grossAmountIn.Add(grossAmountIn, big.NewInt(1))
	return &QuoteResult{
		RequiredInput:  grossAmountIn.String(),
		PriceImpact:    "0",
		QuoteTimestamp: time.Now().Unix(),
	}, nil
}

// SwapExactOutput submits a signed swapTokensForExactTokens transaction to the Hub.
func (c *Client) SwapExactOutput(ctx context.Context, req SwapRequest) (*SwapResult, error) {
	if c.signer == nil {
		return nil, errors.New("amm: swap requires a signing key; configure PrivateKeyHex")
	}
	amountOut, ok := new(big.Int).SetString(strings.TrimSpace(req.AmountOut), 10)
	if !ok {
		return nil, fmt.Errorf("invalid AmountOut %q", req.AmountOut)
	}
	maxAmountIn, ok := new(big.Int).SetString(strings.TrimSpace(req.MaxAmountIn), 10)
	if !ok {
		return nil, fmt.Errorf("invalid MaxAmountIn %q", req.MaxAmountIn)
	}
	// Resolve token addresses: default to config values if not explicitly provided.
	tokenIn := c.tokenA
	tokenOut := c.tokenB
	if req.TokenIn != "" {
		tokenIn = common.HexToAddress(req.TokenIn)
	}
	if req.TokenOut != "" {
		tokenOut = common.HexToAddress(req.TokenOut)
	}
	// Recipient defaults to signer's own address (registered in IdentityRegistry).
	to := c.signer.Address()
	if req.To != "" {
		to = common.HexToAddress(req.To)
	}
	if err := c.ensureUnlimitedApproval(ctx, tokenIn); err != nil {
		return nil, fmt.Errorf("approve tokenIn for swap: %w", err)
	}
	receipt, txHash, err := evm.SubmitTxReceipt(ctx, c.ec, c.signer, c.contract, c.abi,
		"swapTokensForExactTokens",
		tokenIn,
		tokenOut,
		amountOut,
		maxAmountIn,
		to,
	)
	if err != nil {
		return nil, err
	}
	// Realized input cost comes from the on-chain LogSwap event, not from MaxAmountIn.
	// Echoing the cap would make the orchestrator's post-trade slippage check a tautology
	// and persist an inflated amount_in. Fall back to the cap only if the event is somehow
	// absent — the swap already succeeded (receipt status checked), so we must not fail it.
	amountIn := req.MaxAmountIn
	if verified, perr := ParseSwapLogs(receipt.Logs, c.contract); perr == nil {
		amountIn = verified.AmountIn
	}
	return &SwapResult{TxHash: txHash, AmountIn: amountIn, OrderID: req.PayerID}, nil
}

// approveToken grants the AMM contract an exact allowance on a given ERC-20 token.
// Used by AddLiquidity, DepositForCommit, and DepositForCommitAt where the caller
// controls the amount being deposited. For the swap hot-path use ensureUnlimitedApproval.
func (c *Client) approveToken(ctx context.Context, token common.Address, amount *big.Int) error {
	if token == (common.Address{}) {
		return fmt.Errorf("amm: token address not resolved; check AMM contract address and RPC connectivity")
	}
	_, err := evm.SubmitTx(ctx, c.ec, c.signer, token, c.erc20ABI, "approve", c.contract, amount)
	return err
}

// ensureUnlimitedApproval approves max_uint256 for the swap hot-path if the current
// on-chain allowance is below approvalFloor. The result is cached in-process so
// subsequent swaps skip both the RPC read and the approve transaction.
//
// The entire check-and-approve sequence runs under approvalMu so concurrent swap
// goroutines cannot each submit a redundant approve transaction on the first call.
func (c *Client) ensureUnlimitedApproval(ctx context.Context, token common.Address) error {
	if token == (common.Address{}) {
		return fmt.Errorf("amm: token address not resolved; check AMM contract address and RPC connectivity")
	}
	if c.signer == nil {
		return fmt.Errorf("amm: signer required for approval")
	}

	c.approvalMu.Lock()
	defer c.approvalMu.Unlock()

	// Always read the on-chain allowance — the `approved` cache is only a fast-path
	// hint, never authoritative: another path (mint-and-approve, addLiquidity) can
	// have reset the allowance to a FINITE amount via approve(n), which overwrites a
	// prior unlimited approval. Trusting a stale cached `true` would skip the needed
	// re-approval and make the next swap revert with ERC20InsufficientAllowance.
	allowance := new(big.Int)
	if err := evm.Call(ctx, c.ec, token, c.erc20ABI, "allowance",
		[]interface{}{c.signer.Address(), c.contract}, allowance); err != nil {
		return fmt.Errorf("read allowance: %w", err)
	}
	if allowance.Cmp(approvalFloor) >= 0 {
		c.approved[token] = true
		return nil
	}

	if _, err := evm.SubmitTx(ctx, c.ec, c.signer, token, c.erc20ABI, "approve", c.contract, maxUint256); err != nil {
		return fmt.Errorf("approve max for swap: %w", err)
	}
	c.approved[token] = true
	return nil
}

// AddLiquidity approves both pool tokens and submits an addLiquidity transaction.
func (c *Client) AddLiquidity(ctx context.Context, amountA, amountB *big.Int) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: addLiquidity requires a signing key")
	}
	if err := c.approveToken(ctx, c.tokenA, amountA); err != nil {
		return "", fmt.Errorf("approve TOKEN_A: %w", err)
	}
	if err := c.approveToken(ctx, c.tokenB, amountB); err != nil {
		return "", fmt.Errorf("approve TOKEN_B: %w", err)
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "addLiquidity", amountA, amountB)
}

// RemoveLiquidity burns `shares` of the configured signer and returns a single home currency
// (homeIsTokenA selects TOKEN_A vs TOKEN_B) via the zap-out path (decision D1), enforcing
// `minAmountOut`. The signer must own the shares — withdrawal authority lives with the share
// owner now that shares are on-chain (decision D4). Returns the realized amountOut decoded from
// LogLiquidityRemoved, plus the transaction hash.
func (c *Client) RemoveLiquidity(ctx context.Context, shares *big.Int, homeIsTokenA bool, minAmountOut *big.Int) (amountOut *big.Int, txHash string, err error) {
	if c.signer == nil {
		return nil, "", errors.New("amm: removeLiquidity requires a signing key")
	}
	tokenOut := c.tokenB
	if homeIsTokenA {
		tokenOut = c.tokenA
	}
	// keccak256("LogLiquidityRemoved(address,uint256,address,uint256)")
	eventSig := crypto.Keccak256Hash([]byte("LogLiquidityRemoved(address,uint256,address,uint256)"))
	receipt, txHash, err := evm.SubmitTxReceipt(ctx, c.ec, c.signer, c.contract, c.abi,
		"removeLiquidity", shares, tokenOut, minAmountOut)
	if err != nil {
		return nil, "", err
	}
	for _, lg := range receipt.Logs {
		// topics = [sig, provider, tokenOut]; data = sharesBurned (32) + amountOut (32).
		if len(lg.Topics) >= 3 && lg.Topics[0] == eventSig && len(lg.Data) >= 64 {
			return new(big.Int).SetBytes(lg.Data[32:64]), txHash, nil
		}
	}
	// Burn succeeded but the event was not found (unexpected) — report success without the amount.
	return nil, txHash, nil
}

// LPBalanceOf returns the LP-share balance of `holder` on the configured AMM (the on-chain
// source of truth for pool ownership). An empty holder defaults to the configured signer's own
// address — on a sovereign CB gateway that is the CB itself (decision D4).
func (c *Client) LPBalanceOf(ctx context.Context, holder string) (*big.Int, error) {
	addr := c.resolveRecipient(holder)
	if addr == (common.Address{}) {
		return nil, errors.New("amm: LPBalanceOf requires a holder address or a configured signer")
	}
	out := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "balanceOf",
		[]interface{}{addr}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// LPTotalSupply returns the total LP-share supply on the configured AMM.
func (c *Client) LPTotalSupply(ctx context.Context) (*big.Int, error) {
	out := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "totalSupply", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// WithdrawalFeeBps returns the current withdrawal (zap-out) fee rate in basis points.
func (c *Client) WithdrawalFeeBps(ctx context.Context) (*big.Int, error) {
	out := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "withdrawalFeeBps", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DepositForCommit escrows the signer's single side against `commitID` on the configured AMM,
// crediting LP shares (on finalize) to `shareRecipient` (escrow-and-finalize, decision D6).
func (c *Client) DepositForCommit(ctx context.Context, commitID [32]byte, isTokenA bool, amount *big.Int, shareRecipient string) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: depositForCommit requires a signing key")
	}
	token := c.tokenB
	if isTokenA {
		token = c.tokenA
	}
	if err := c.approveToken(ctx, token, amount); err != nil {
		return "", fmt.Errorf("approve token for commit deposit: %w", err)
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi,
		"depositForCommit", commitID, isTokenA, amount, c.resolveRecipient(shareRecipient))
}

// resolveRecipient returns the recipient address, defaulting an empty string to the signer's own
// address — the depositing gateway is the intended share owner (sovereign CB per D4; commercial
// operator interim, with the off-chain ledger mapping shares back to banks).
func (c *Client) resolveRecipient(shareRecipient string) common.Address {
	if strings.TrimSpace(shareRecipient) == "" && c.signer != nil {
		return c.signer.Address()
	}
	return common.HexToAddress(shareRecipient)
}

// FinalizeCommit finalizes a fully-deposited commit on the configured AMM.
func (c *Client) FinalizeCommit(ctx context.Context, commitID [32]byte) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: finalizeCommit requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "finalizeCommit", commitID)
}

// CancelCommitDeposit reclaims an un-finalized escrowed side (refund/timeout) on the configured AMM.
func (c *Client) CancelCommitDeposit(ctx context.Context, commitID [32]byte, isTokenA bool) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: cancelCommitDeposit requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "cancelCommitDeposit", commitID, isTokenA)
}

// DepositForCommitAt escrows the signer's single side against `commitID` on an arbitrary AMM
// (sovereign flow: each CB gateway deposits its own currency to the matched pair's pool).
func (c *Client) DepositForCommitAt(ctx context.Context, ammAddress string, commitID [32]byte, isTokenA bool, amount *big.Int, shareRecipient string) error {
	if c.signer == nil {
		return errors.New("amm: depositForCommitAt requires a signing key")
	}
	contract := common.HexToAddress(ammAddress)
	var token common.Address
	if isTokenA {
		_ = evm.Call(ctx, c.ec, contract, c.abi, "TOKEN_A", nil, &token)
	} else {
		_ = evm.Call(ctx, c.ec, contract, c.abi, "TOKEN_B", nil, &token)
	}
	if (token == common.Address{}) {
		return fmt.Errorf("amm: could not resolve token address from AMM %s", ammAddress)
	}
	if _, err := evm.SubmitTx(ctx, c.ec, c.signer, token, c.erc20ABI, "approve", contract, amount); err != nil {
		return fmt.Errorf("approve token for sovereign commit deposit: %w", err)
	}
	if _, err := evm.SubmitTx(ctx, c.ec, c.signer, contract, c.abi,
		"depositForCommit", commitID, isTokenA, amount, c.resolveRecipient(shareRecipient)); err != nil {
		return fmt.Errorf("depositForCommitAt %s: %w", ammAddress, err)
	}
	return nil
}

// FinalizeResult carries the share mint recorded by LogCommitFinalized when a finalize succeeds.
type FinalizeResult struct {
	RecipientA common.Address
	RecipientB common.Address
	SharesA    *big.Int
	SharesB    *big.Int
}

// FinalizeCommitAt finalizes a commit on an arbitrary AMM (sovereign). A revert because the other
// side is not yet escrowed is surfaced to the caller, which may treat it as "pending finalize".
// On success it decodes the LogCommitFinalized event so callers can persist the minted shares.
func (c *Client) FinalizeCommitAt(ctx context.Context, ammAddress string, commitID [32]byte) (*FinalizeResult, error) {
	if c.signer == nil {
		return nil, errors.New("amm: finalizeCommitAt requires a signing key")
	}
	contract := common.HexToAddress(ammAddress)
	// keccak256("LogCommitFinalized(bytes32,address,address,uint256,uint256)")
	eventSig := crypto.Keccak256Hash([]byte("LogCommitFinalized(bytes32,address,address,uint256,uint256)"))
	receipt, _, err := evm.SubmitTxReceipt(ctx, c.ec, c.signer, contract, c.abi, "finalizeCommit", commitID)
	if err != nil {
		return nil, fmt.Errorf("finalizeCommitAt %s: %w", ammAddress, err)
	}
	for _, lg := range receipt.Logs {
		// data = abi(recipientA, recipientB, sharesA, sharesB) — 4 static 32-byte words.
		if len(lg.Topics) >= 2 && lg.Topics[0] == eventSig && len(lg.Data) >= 128 {
			return &FinalizeResult{
				RecipientA: common.BytesToAddress(lg.Data[0:32]),
				RecipientB: common.BytesToAddress(lg.Data[32:64]),
				SharesA:    new(big.Int).SetBytes(lg.Data[64:96]),
				SharesB:    new(big.Int).SetBytes(lg.Data[96:128]),
			}, nil
		}
	}
	// Finalize succeeded but the event was not found (unexpected) — report success without shares.
	return &FinalizeResult{}, nil
}

// TokenBalanceAt reads the ERC-20 balanceOf for the token held by ammAddress (TOKEN_A or TOKEN_B)
// for the given holderAddr. Used by the SovereignAddLiquidity pre-flight check (FR-010 / T021).
func (c *Client) TokenBalanceAt(ctx context.Context, ammAddress string, isTokenA bool, holderAddr string) (*big.Int, error) {
	contract := common.HexToAddress(ammAddress)
	var token common.Address
	tokenMethod := "TOKEN_A"
	if !isTokenA {
		tokenMethod = "TOKEN_B"
	}
	if err := evm.Call(ctx, c.ec, contract, c.abi, tokenMethod, nil, &token); err != nil {
		return nil, fmt.Errorf("amm: resolve %s from %s: %w", tokenMethod, ammAddress, err)
	}
	if (token == common.Address{}) {
		return nil, fmt.Errorf("amm: %s returned zero address for %s", tokenMethod, ammAddress)
	}
	balance := new(big.Int)
	holder := common.HexToAddress(holderAddr)
	if err := evm.Call(ctx, c.ec, token, c.erc20ABI, "balanceOf", []interface{}{holder}, balance); err != nil {
		return nil, fmt.Errorf("amm: balanceOf(%s) on token %s: %w", holderAddr, token.Hex(), err)
	}
	return balance, nil
}

// FeeBps returns the current fee rate in basis points from the contract.
func (c *Client) FeeBps(ctx context.Context) (*big.Int, error) {
	out := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "feeBps", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PauseCircuitBreaker invokes pause(reason) on the AMM contract (1-of-N Central Bank).
// The signature parameter is retained in the interface for audit trail parity with FR-056;
// the actual on-chain authorisation is enforced by IdentityRegistry.canGovern(msg.sender).
func (c *Client) PauseCircuitBreaker(ctx context.Context, reason string) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: pause requires a signing key")
	}
	if reason == "" {
		reason = "governance emergency pause"
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "pause", reason)
}

// ProposeResume submits a resume proposal and returns the resulting proposalId (hex string).
// It extracts the proposalId from the LogResumeProposed event emitted by the AMM.
func (c *Client) ProposeResume(ctx context.Context) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: proposeResume requires a signing key")
	}
	// keccak256("LogResumeProposed(bytes32,address,uint256)")
	eventSig := crypto.Keccak256Hash([]byte("LogResumeProposed(bytes32,address,uint256)"))
	receipt, _, err := evm.SubmitTxReceipt(ctx, c.ec, c.signer, c.contract, c.abi, "proposeResume")
	if err != nil {
		return "", err
	}
	for _, log := range receipt.Logs {
		if len(log.Topics) >= 2 && log.Topics[0] == eventSig {
			proposalID := log.Topics[1]
			return "0x" + hex.EncodeToString(proposalID[:]), nil
		}
	}
	return "", errors.New("amm: LogResumeProposed event not found in receipt")
}

// SignResume adds a signature to an existing resume proposal. When quorum (2-of-N) is met
// the AMM auto-unpauses atomically within this transaction.
func (c *Client) SignResume(ctx context.Context, proposalID [32]byte) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: signResume requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "signResume", proposalID)
}
