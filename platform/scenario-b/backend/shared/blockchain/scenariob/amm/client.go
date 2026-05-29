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
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// erc20ABI is the minimal ERC-20 ABI for approve() and balanceOf().
// approve() is required before safeTransferFrom; balanceOf() is used for pre-flight
// balance checks in SovereignAddLiquidity (FR-010 / T021).
const erc20ABI = `[{"type":"function","name":"approve","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},{"type":"function","name":"balanceOf","stateMutability":"view","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]}]`

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
{"type":"function","name":"getAmountIn","stateMutability":"pure","inputs":[
  {"name":"reserveIn","type":"uint256"},
  {"name":"reserveOut","type":"uint256"},
  {"name":"amountOut","type":"uint256"}
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
],"outputs":[]},
{"type":"function","name":"removeLiquidity","stateMutability":"nonpayable","inputs":[
  {"name":"amountA","type":"uint256"},
  {"name":"amountB","type":"uint256"}
],"outputs":[]},
{"type":"function","name":"addSingleSidedLiquidity","stateMutability":"nonpayable","inputs":[
  {"name":"isTokenA","type":"bool"},
  {"name":"amount","type":"uint256"}
],"outputs":[]},
{"type":"function","name":"removeSingleSidedLiquidity","stateMutability":"nonpayable","inputs":[
  {"name":"isTokenA","type":"bool"},
  {"name":"amount","type":"uint256"}
],"outputs":[]},
{"type":"function","name":"feeBps","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"setFeeBps","stateMutability":"nonpayable","inputs":[
  {"name":"newFeeBps","type":"uint256"}
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
	contract common.Address
	ec       *ethclient.Client
	abi      abi.ABI
	erc20ABI abi.ABI
	signer   *evm.Signer
	timeout  time.Duration
	tokenA   common.Address
	tokenB   common.Address
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
	if err := c.approveToken(ctx, tokenIn, maxAmountIn); err != nil {
		return nil, fmt.Errorf("approve tokenIn for swap: %w", err)
	}
	txHash, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi,
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
	return &SwapResult{TxHash: txHash, AmountIn: req.MaxAmountIn, OrderID: req.PayerID}, nil
}

// approveToken grants the AMM contract max-uint256 allowance on a given ERC-20 token.
// It is called before addLiquidity and swap to satisfy safeTransferFrom requirements.
func (c *Client) approveToken(ctx context.Context, token common.Address, amount *big.Int) error {
	if token == (common.Address{}) {
		return fmt.Errorf("amm: token address not resolved; check AMM contract address and RPC connectivity")
	}
	_, err := evm.SubmitTx(ctx, c.ec, c.signer, token, c.erc20ABI, "approve", c.contract, amount)
	return err
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

// RemoveLiquidity submits a removeLiquidity transaction from the configured signer.
func (c *Client) RemoveLiquidity(ctx context.Context, amountA, amountB *big.Int) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: removeLiquidity requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "removeLiquidity", amountA, amountB)
}

// AddSingleSidedLiquidity submits an addSingleSidedLiquidity transaction.
// The caller must hold the Liquidity Provider role on-chain (IdentityRegistry).
// isTokenA=true deposits TOKEN_A; false deposits TOKEN_B.
func (c *Client) AddSingleSidedLiquidity(ctx context.Context, isTokenA bool, amount *big.Int) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: addSingleSidedLiquidity requires a signing key")
	}
	token := c.tokenB
	if isTokenA {
		token = c.tokenA
	}
	if err := c.approveToken(ctx, token, amount); err != nil {
		return "", fmt.Errorf("approve token for single-sided: %w", err)
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "addSingleSidedLiquidity", isTokenA, amount)
}

// AddSingleSidedLiquidityAt calls addSingleSidedLiquidity on an arbitrary AMM contract address
// (different from the client's configured pool). Used by sovereign CB liquidity flows where the
// matched commit may target any registered sovereign pair pool.
func (c *Client) AddSingleSidedLiquidityAt(ctx context.Context, ammAddress string, isTokenA bool, amount *big.Int) error {
	if c.signer == nil {
		return errors.New("amm: addSingleSidedLiquidityAt requires a signing key")
	}
	contract := common.HexToAddress(ammAddress)
	// Resolve the token address by calling TOKEN_A/TOKEN_B on the target AMM.
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
		return fmt.Errorf("approve token for sovereign single-sided: %w", err)
	}
	if _, err := evm.SubmitTx(ctx, c.ec, c.signer, contract, c.abi, "addSingleSidedLiquidity", isTokenA, amount); err != nil {
		return fmt.Errorf("addSingleSidedLiquidityAt %s: %w", ammAddress, err)
	}
	return nil
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

// RemoveSingleSidedLiquidity submits a removeSingleSidedLiquidity transaction.
// isTokenA=true withdraws TOKEN_A; false withdraws TOKEN_B.
func (c *Client) RemoveSingleSidedLiquidity(ctx context.Context, isTokenA bool, amount *big.Int) (string, error) {
	if c.signer == nil {
		return "", errors.New("amm: removeSingleSidedLiquidity requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "removeSingleSidedLiquidity", isTokenA, amount)
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
