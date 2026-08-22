// SPDX-License-Identifier: Apache-2.0

// Package app — PairRegistry EVM adapter.
// Wraps the on-chain PairRegistry contract (contracts/src/PairRegistry.sol) for use by
// pair_router.go and pair_handler.go (D9/D10 — 005-cooperative-liquidity).
package app

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry/bindings"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// pairRegistryABI is the minimal ABI for PairRegistry.sol.
// Keep in sync with contracts/src/PairRegistry.sol (D9).
const pairRegistryABI = `[
{"type":"function","name":"proposePair","stateMutability":"nonpayable","inputs":[
  {"name":"pairId","type":"string"},
  {"name":"tokenA","type":"address"},
  {"name":"tokenB","type":"address"},
  {"name":"ammAddress","type":"address"}
],"outputs":[]},
{"type":"function","name":"confirmPair","stateMutability":"nonpayable","inputs":[
  {"name":"pairId","type":"string"}
],"outputs":[]},
{"type":"function","name":"getActivePairsPaged","stateMutability":"view","inputs":[{"name":"offset","type":"uint256"},{"name":"limit","type":"uint256"}],"outputs":[
  {"name":"page","type":"tuple[]","components":[
    {"name":"pairId","type":"string"},
    {"name":"ammAddress","type":"address"},
    {"name":"tokenA","type":"address"},
    {"name":"tokenB","type":"address"},
    {"name":"status","type":"uint8"},
    {"name":"proposer","type":"address"},
    {"name":"confirmer","type":"address"}
  ]},
  {"name":"total","type":"uint256"}
]},
{"type":"function","name":"getAllActivePairs","stateMutability":"view","inputs":[],"outputs":[
  {"name":"","type":"tuple[]","components":[
    {"name":"pairId","type":"string"},
    {"name":"ammAddress","type":"address"},
    {"name":"tokenA","type":"address"},
    {"name":"tokenB","type":"address"},
    {"name":"status","type":"uint8"},
    {"name":"proposer","type":"address"},
    {"name":"confirmer","type":"address"}
  ]}
]},
{"type":"function","name":"getAllPairs","stateMutability":"view","inputs":[],"outputs":[
  {"name":"","type":"tuple[]","components":[
    {"name":"pairId","type":"string"},
    {"name":"ammAddress","type":"address"},
    {"name":"tokenA","type":"address"},
    {"name":"tokenB","type":"address"},
    {"name":"status","type":"uint8"},
    {"name":"proposer","type":"address"},
    {"name":"confirmer","type":"address"}
  ]}
]},
{"type":"event","name":"PairRegistered","anonymous":false,"inputs":[
  {"name":"pairId","type":"string","indexed":true},
  {"name":"ammAddress","type":"address","indexed":true},
  {"name":"tokenA","type":"address","indexed":false},
  {"name":"tokenB","type":"address","indexed":false}
]},
{"type":"event","name":"PairProposed","anonymous":false,"inputs":[
  {"name":"pairId","type":"string","indexed":true},
  {"name":"proposer","type":"address","indexed":true},
  {"name":"tokenA","type":"address","indexed":false},
  {"name":"tokenB","type":"address","indexed":false}
]}
]`

// PairRegisteredEvent is the decoded payload of an on-chain PairRegistered event.
type PairRegisteredEvent struct {
	PairID     string
	AMMAddress string
	TokenA     string
	TokenB     string
}

// PairRegistryClient is an EVM client for PairRegistry.sol.
type PairRegistryClient struct {
	contract         common.Address
	ec               *ethclient.Client
	parsed           abi.ABI
	signer           *evm.Signer
	timeout          time.Duration
	identityRegistry common.Address
}

// PairRegistryConfig holds connection parameters for the PairRegistry EVM client.
type PairRegistryConfig struct {
	RPCURL          string
	ContractAddress string
	ChainID         int64
	PrivateKeyHex   string
	Timeout         time.Duration
	// IdentityRegistryAddress is the Hub IdentityRegistry passed to the
	// AutomatedMarketMaker constructor when ProposePair deploys a dedicated,
	// per-pair AMM (empty amm_address path). Optional for read-only clients.
	IdentityRegistryAddress string
}

// NewPairRegistryClient constructs a PairRegistryClient from configuration.
func NewPairRegistryClient(ctx context.Context, cfg PairRegistryConfig) (*PairRegistryClient, error) {
	if cfg.ContractAddress == "" {
		return nil, fmt.Errorf("PairRegistryConfig: ContractAddress is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, cfg.RPCURL, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("pair registry: dial: %w", err)
	}
	parsed, err := evm.ParseABI(pairRegistryABI)
	if err != nil {
		ec.Close()
		return nil, fmt.Errorf("pair registry: parse ABI: %w", err)
	}
	c := &PairRegistryClient{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		parsed:   parsed,
		timeout:  cfg.Timeout,
	}
	if cfg.IdentityRegistryAddress != "" {
		c.identityRegistry = common.HexToAddress(cfg.IdentityRegistryAddress)
	}
	if cfg.PrivateKeyHex != "" {
		signer, sigErr := evm.SharedSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
		if sigErr != nil {
			ec.Close()
			return nil, fmt.Errorf("pair registry: signer: %w", sigErr)
		}
		c.signer = signer
	}
	return c, nil
}

// identityCBOfABI is the minimal IdentityRegistry read used to resolve which address may act
// as the central bank of a token on the hub. PairRegistry gates proposePair on
// getCentralBankOf(tokenA) and confirmPair on getCentralBankOf(tokenB).
const identityCBOfABI = `[
{"type":"function","name":"getCentralBankOf","stateMutability":"view","inputs":[{"name":"token","type":"address"}],"outputs":[{"name":"","type":"address"}]}
]`

// CentralBankOfToken reads IdentityRegistry.getCentralBankOf(token) on the hub, i.e. which
// address the registry recognises as that token's issuing central bank.
func (c *PairRegistryClient) CentralBankOfToken(ctx context.Context, tokenAddress string) (string, error) {
	if c.identityRegistry == (common.Address{}) {
		return "", fmt.Errorf("pair registry: HUB_IDENTITY_REGISTRY_ADDRESS not configured")
	}
	token := common.HexToAddress(tokenAddress)
	if token == (common.Address{}) {
		return "", fmt.Errorf("pair registry: invalid token address %q", tokenAddress)
	}
	parsed, err := evm.ParseABI(identityCBOfABI)
	if err != nil {
		return "", fmt.Errorf("pair registry: parse identity ABI: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var cb common.Address
	if err := evm.Call(callCtx, c.ec, c.identityRegistry, parsed, "getCentralBankOf",
		[]interface{}{token}, &cb); err != nil {
		return "", fmt.Errorf("pair registry: getCentralBankOf(%s): %w", token.Hex(), err)
	}
	return cb.Hex(), nil
}

// HubSignerAddress is this gateway's hub signing address, or "" on a read-only client.
func (c *PairRegistryClient) HubSignerAddress() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
}

// Close releases the underlying RPC connection.
func (c *PairRegistryClient) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// ProposePair submits a proposePair transaction.
func (c *PairRegistryClient) ProposePair(
	ctx context.Context,
	pairID, tokenA, tokenB, ammAddress string,
) (string, error) {
	if c.signer == nil {
		return "", fmt.Errorf("pair registry: proposePair requires a signing key")
	}
	txHash, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.parsed, "proposePair",
		pairID,
		common.HexToAddress(tokenA),
		common.HexToAddress(tokenB),
		common.HexToAddress(ammAddress),
	)
	if err != nil {
		return "", fmt.Errorf("pair registry proposePair: %w", err)
	}
	return txHash, nil
}

// DeployDedicatedAMM deploys a fresh AutomatedMarketMaker bound to (tokenA, tokenB)
// and the Hub IdentityRegistry, returning its address. This is the per-pair pool a
// corridor must own: the AMM's TOKEN_A/TOKEN_B are immutable, so a pair MUST be
// backed by an AMM constructed over its own tokens — never the shared bootstrap AMM
// (whose tokens are unrelated), which is why add-liquidity against a mis-bound pair
// reverts. Mirrors the compliance-service RegisterPair deploy path (besu.go) so both
// the internal relay and the CB-authenticated v2 propose flow produce identical pools.
func (c *PairRegistryClient) DeployDedicatedAMM(ctx context.Context, tokenA, tokenB string) (string, error) {
	if c.signer == nil {
		return "", fmt.Errorf("pair registry: deploy AMM requires a signing key")
	}
	if (c.identityRegistry == common.Address{}) {
		return "", fmt.Errorf("pair registry: deploy AMM requires HUB_IDENTITY_REGISTRY_ADDRESS")
	}
	opts, err := c.signer.TransactOpts(ctx)
	if err != nil {
		return "", fmt.Errorf("pair registry: deploy AMM opts: %w", err)
	}
	if gasPrice, gerr := c.ec.SuggestGasPrice(ctx); gerr == nil {
		opts.GasPrice = gasPrice
	}
	// The deploy goes through the signer's counter like every other submission from this account:
	// bind would otherwise read PendingNonceAt itself and could claim a nonce another call in this
	// process has already taken. The deployed address is derived from (sender, nonce), so it stays
	// correct precisely because the nonce is the one actually broadcast.
	var ammAddr common.Address
	deployTx, err := c.signer.WithNonce(ctx, c.ec, func(nonce uint64) (*types.Transaction, error) {
		opts.Nonce = new(big.Int).SetUint64(nonce)
		addr, tx, _, derr := bindings.DeployAutomatedMarketMaker(
			opts, c.ec,
			common.HexToAddress(tokenA),
			common.HexToAddress(tokenB),
			c.identityRegistry,
		)
		if derr != nil {
			return nil, derr
		}
		ammAddr = addr
		return tx, nil
	})
	if err != nil {
		return "", fmt.Errorf("pair registry: deploy AMM: %w", err)
	}
	if _, err := evm.WaitForReceipt(ctx, c.ec, deployTx, "deploy AMM"); err != nil {
		return "", fmt.Errorf("pair registry: %w", err)
	}

	c.applyDefaultFee(ctx, ammAddr)
	return ammAddr.Hex(), nil
}

// defaultAMMFeeBps is the swap fee a freshly deployed corridor AMM is configured with.
//
// The contract's constructor leaves feeBps at 0, and the design documents a 30 bps fee
// distributed to liquidity providers on every swap (docs/runbooks/contract-configuration.md,
// docs/design/cooperative-liquidity.md). Under the old single static AMM that gap did
// not show: the pool was configured once, out of band. Now every corridor deploys its
// own AMM at propose time, and nothing was setting the fee — so a runtime-opened
// corridor charged nothing and its providers earned nothing.
const defaultAMMFeeBps = 30

// applyDefaultFee sets the documented swap fee on a just-deployed AMM.
//
// Best-effort, and loudly so. The AMM already exists on-chain by the time this runs, so
// returning an error here would fail the propose and strand a deployed pool. A corridor
// with no fee still settles payments — it just pays its liquidity providers nothing —
// which is a condition to shout about, not one to abort on.
//
// AMM_DEFAULT_FEE_BPS overrides the default; "0" is honoured as a deliberate choice and
// skips the call entirely, so an operator wanting a fee-less corridor gets one without a
// misleading error in the log.
func (c *PairRegistryClient) applyDefaultFee(ctx context.Context, amm common.Address) {
	feeBps := defaultAMMFeeBps
	if raw := strings.TrimSpace(os.Getenv("AMM_DEFAULT_FEE_BPS")); raw != "" {
		parsed, perr := strconv.Atoi(raw)
		if perr != nil || parsed < 0 {
			log.Printf("warning: AMM_DEFAULT_FEE_BPS=%q is not a non-negative integer; using %d", raw, defaultAMMFeeBps)
		} else {
			feeBps = parsed
		}
	}
	if feeBps == 0 {
		log.Printf("AMM %s deployed with feeBps=0 (AMM_DEFAULT_FEE_BPS=0): liquidity providers earn nothing on this corridor", amm.Hex())
		return
	}

	transactor, err := bindings.NewAutomatedMarketMakerTransactor(amm, c.ec)
	if err != nil {
		log.Printf("warning: AMM %s deployed but its fee could NOT be set (bind: %v) — feeBps stays 0 and liquidity providers earn nothing", amm.Hex(), err)
		return
	}
	opts, err := c.signer.TransactOpts(ctx)
	if err != nil {
		log.Printf("warning: AMM %s deployed but its fee could NOT be set (opts: %v) — feeBps stays 0 and liquidity providers earn nothing", amm.Hex(), err)
		return
	}
	if gasPrice, gerr := c.ec.SuggestGasPrice(ctx); gerr == nil {
		opts.GasPrice = gasPrice
	}
	// Same nonce counter as every other submission from this account — the deploy above
	// took one, and bind would otherwise read PendingNonceAt and race it.
	tx, err := c.signer.WithNonce(ctx, c.ec, func(nonce uint64) (*types.Transaction, error) {
		opts.Nonce = new(big.Int).SetUint64(nonce)
		return transactor.SetFeeBps(opts, big.NewInt(int64(feeBps)))
	})
	if err != nil {
		log.Printf("warning: AMM %s deployed but setFeeBps(%d) failed (%v) — feeBps stays 0 and liquidity providers earn nothing", amm.Hex(), feeBps, err)
		return
	}
	if _, err := evm.WaitForReceipt(ctx, c.ec, tx, "setFeeBps"); err != nil {
		log.Printf("warning: AMM %s setFeeBps(%d) was submitted but not confirmed (%v) — verify feeBps on-chain", amm.Hex(), feeBps, err)
		return
	}
	log.Printf("AMM %s deployed with feeBps=%d", amm.Hex(), feeBps)
}

// ConfirmPair submits a confirmPair transaction.
func (c *PairRegistryClient) ConfirmPair(ctx context.Context, pairID string) (string, error) {
	if c.signer == nil {
		return "", fmt.Errorf("pair registry: confirmPair requires a signing key")
	}
	txHash, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.parsed, "confirmPair", pairID)
	if err != nil {
		return "", fmt.Errorf("pair registry confirmPair: %w", err)
	}
	return txHash, nil
}

// pairPageSize is how many entries one getActivePairsPaged call asks for; it matches the
// contract's MAX_PAGE_SIZE, which clamps anything larger anyway.
const pairPageSize = 100

// GetAllActivePairs reads every ACTIVE pair from on-chain, one bounded page at a time.
//
// It used to call getAllActivePairs(), which returns the whole set in one response. That is
// `external view` so no gas is at stake, but the response grows with the number of corridors
// and one oversized eth_call fails worse than several small ones (finding R2-M-14). The
// contract keeps that function for compatibility; the platform no longer uses it.
//
// The loop stops on a short page, which is also how an offset past the end reads, so a set
// that shrinks mid-walk terminates rather than spinning.
func (c *PairRegistryClient) GetAllActivePairs(ctx context.Context) ([]domain.PairEntry, error) {
	var out []domain.PairEntry
	for offset := uint64(0); ; offset += pairPageSize {
		page, total, err := c.activePairsPage(ctx, offset, pairPageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(page) < pairPageSize || uint64(len(out)) >= total {
			return out, nil
		}
	}
}

// activePairsPage reads one window and reports the total so the caller can stop.
func (c *PairRegistryClient) activePairsPage(ctx context.Context, offset, limit uint64) ([]domain.PairEntry, uint64, error) {
	input, err := c.parsed.Pack("getActivePairsPaged", new(big.Int).SetUint64(offset), new(big.Int).SetUint64(limit))
	if err != nil {
		return nil, 0, fmt.Errorf("pair registry getActivePairsPaged pack: %w", err)
	}
	msg := ethereum.CallMsg{To: &c.contract, Data: input}
	raw, err := c.ec.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("pair registry getActivePairsPaged call: %w", err)
	}
	if len(raw) == 0 {
		return nil, 0, nil
	}

	method := c.parsed.Methods["getActivePairsPaged"]
	entries, err := method.Outputs.Unpack(raw)
	if err != nil {
		return nil, 0, fmt.Errorf("pair registry getActivePairsPaged unpack: %w", err)
	}
	if len(entries) < 2 {
		return nil, 0, fmt.Errorf("pair registry getActivePairsPaged: expected (page, total), got %d outputs", len(entries))
	}
	total, ok := entries[1].(*big.Int)
	if !ok {
		return nil, 0, fmt.Errorf("pair registry getActivePairsPaged: total has unexpected type %T", entries[1])
	}

	// go-ethereum unpacks tuple[] as a slice of anonymous structs via reflection.
	// Type-asserting to a named struct always fails; use reflect to extract fields.
	rv := reflect.ValueOf(entries[0])
	if rv.Kind() != reflect.Slice {
		return nil, 0, fmt.Errorf("pair registry getActivePairsPaged: unexpected output type %T", entries[0])
	}

	result := make([]domain.PairEntry, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		pairId := item.FieldByName("PairId")
		ammAddr := item.FieldByName("AmmAddress")
		tokenA := item.FieldByName("TokenA")
		tokenB := item.FieldByName("TokenB")
		if !pairId.IsValid() || !ammAddr.IsValid() || !tokenA.IsValid() || !tokenB.IsValid() {
			continue
		}
		result = append(result, domain.PairEntry{
			PairID:     pairId.String(),
			AMMAddress: strings.ToLower(ammAddr.Interface().(common.Address).Hex()),
			TokenA:     strings.ToLower(tokenA.Interface().(common.Address).Hex()),
			TokenB:     strings.ToLower(tokenB.Interface().(common.Address).Hex()),
		})
	}
	return result, total.Uint64(), nil
}

// GetAllPairs reads every registered pair from on-chain, regardless of status
// (PROPOSED and ACTIVE). It backs cross-CB discovery: a pair proposed via one
// Central Bank gateway is visible to the counterparty CB before confirmation.
// The on-chain status enum (0=PROPOSED, 1=ACTIVE) is mapped into PairEntry.Status.
func (c *PairRegistryClient) GetAllPairs(ctx context.Context) ([]domain.PairEntry, error) {
	input, err := c.parsed.Pack("getAllPairs")
	if err != nil {
		return nil, fmt.Errorf("pair registry getAllPairs pack: %w", err)
	}
	msg := ethereum.CallMsg{To: &c.contract, Data: input}
	raw, err := c.ec.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("pair registry getAllPairs call: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	method := c.parsed.Methods["getAllPairs"]
	entries, err := method.Outputs.Unpack(raw)
	if err != nil {
		return nil, fmt.Errorf("pair registry getAllPairs unpack: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// go-ethereum unpacks tuple[] as a slice of anonymous structs via reflection.
	// Type-asserting to a named struct always fails; use reflect to extract fields.
	rv := reflect.ValueOf(entries[0])
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("pair registry getAllPairs: unexpected output type %T", entries[0])
	}

	result := make([]domain.PairEntry, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		pairId := item.FieldByName("PairId")
		ammAddr := item.FieldByName("AmmAddress")
		tokenA := item.FieldByName("TokenA")
		tokenB := item.FieldByName("TokenB")
		if !pairId.IsValid() || !ammAddr.IsValid() || !tokenA.IsValid() || !tokenB.IsValid() {
			continue
		}
		result = append(result, domain.PairEntry{
			PairID:     pairId.String(),
			AMMAddress: strings.ToLower(ammAddr.Interface().(common.Address).Hex()),
			TokenA:     strings.ToLower(tokenA.Interface().(common.Address).Hex()),
			TokenB:     strings.ToLower(tokenB.Interface().(common.Address).Hex()),
			Status:     pairStatusString(item.FieldByName("Status")),
		})
	}
	return result, nil
}

// pairStatusString maps the on-chain PairStatus enum (uint8) to its string label.
// Mirrors PairRegistry.sol: 0=PROPOSED, 1=ACTIVE.
func pairStatusString(v reflect.Value) string {
	if !v.IsValid() || !v.CanUint() {
		return ""
	}
	switch v.Uint() {
	case 0:
		return domain.PairStatusProposed
	case 1:
		return domain.PairStatusActive
	default:
		return ""
	}
}

// SubscribePairRegistered opens an event filter for PairRegistered logs.
// Cancel ctx to shut down. Used by PairRouter (D10).
func (c *PairRegistryClient) SubscribePairRegistered(ctx context.Context) (<-chan PairRegisteredEvent, error) {
	pairRegisteredEvent := c.parsed.Events["PairRegistered"]
	query := ethereum.FilterQuery{
		Addresses: []common.Address{c.contract},
		Topics:    [][]common.Hash{{pairRegisteredEvent.ID}},
	}
	logCh := make(chan types.Log, 32)
	sub, err := c.ec.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil {
		return nil, fmt.Errorf("pair registry subscribe: %w", err)
	}

	outCh := make(chan PairRegisteredEvent, 32)
	go func() {
		defer close(outCh)
		for {
			select {
			case <-ctx.Done():
				sub.Unsubscribe()
				return
			case subErr := <-sub.Err():
				if subErr != nil {
					_ = subErr // PairRouter restarts on close
				}
				return
			case evLog := <-logCh:
				ev, decErr := decodePairRegisteredLog(pairRegisteredEvent, evLog)
				if decErr != nil {
					continue
				}
				select {
				case outCh <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return outCh, nil
}

// decodePairRegisteredLog decodes a raw EVM log into a PairRegisteredEvent.
// The indexed pairId string is stored as its keccak256 hash and cannot be recovered.
// PairID is left empty; PairRouter reconciles via DB lookup.
func decodePairRegisteredLog(event abi.Event, evLog types.Log) (PairRegisteredEvent, error) {
	// NonIndexed inputs (tokenA, tokenB) live in the log Data field.
	// abi.Arguments.Unpack returns []interface{}.
	nonIndexedArgs := event.Inputs.NonIndexed()
	values, err := nonIndexedArgs.Unpack(evLog.Data)
	if err != nil {
		return PairRegisteredEvent{}, fmt.Errorf("decode PairRegistered data: %w", err)
	}
	tokenA, _ := values[0].(common.Address)
	tokenB, _ := values[1].(common.Address)

	// Topics[2] holds the indexed ammAddress.
	var ammAddr common.Address
	if len(evLog.Topics) >= 3 {
		ammAddr = common.HexToAddress(evLog.Topics[2].Hex())
	}

	return PairRegisteredEvent{
		PairID:     "",
		AMMAddress: strings.ToLower(ammAddr.Hex()),
		TokenA:     strings.ToLower(tokenA.Hex()),
		TokenB:     strings.ToLower(tokenB.Hex()),
	}, nil
}
