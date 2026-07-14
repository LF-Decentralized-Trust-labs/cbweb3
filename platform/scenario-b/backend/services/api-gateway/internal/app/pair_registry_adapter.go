// SPDX-License-Identifier: Apache-2.0

// Package app — PairRegistry EVM adapter.
// Wraps the on-chain PairRegistry contract (contracts/src/PairRegistry.sol) for use by
// pair_router.go and pair_handler.go (D9/D10 — 005-cooperative-liquidity).
package app

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
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
	contract common.Address
	ec       *ethclient.Client
	parsed   abi.ABI
	signer   *evm.Signer
	timeout  time.Duration
}

// PairRegistryConfig holds connection parameters for the PairRegistry EVM client.
type PairRegistryConfig struct {
	RPCURL          string
	ContractAddress string
	ChainID         int64
	PrivateKeyHex   string
	Timeout         time.Duration
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
	if cfg.PrivateKeyHex != "" {
		signer, sigErr := evm.NewSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
		if sigErr != nil {
			ec.Close()
			return nil, fmt.Errorf("pair registry: signer: %w", sigErr)
		}
		c.signer = signer
	}
	return c, nil
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

// GetAllActivePairs reads all ACTIVE pairs from on-chain.
func (c *PairRegistryClient) GetAllActivePairs(ctx context.Context) ([]domain.PairEntry, error) {
	input, err := c.parsed.Pack("getAllActivePairs")
	if err != nil {
		return nil, fmt.Errorf("pair registry getAllActivePairs pack: %w", err)
	}
	msg := ethereum.CallMsg{To: &c.contract, Data: input}
	raw, err := c.ec.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("pair registry getAllActivePairs call: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	method := c.parsed.Methods["getAllActivePairs"]
	entries, err := method.Outputs.Unpack(raw)
	if err != nil {
		return nil, fmt.Errorf("pair registry getAllActivePairs unpack: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// go-ethereum unpacks tuple[] as a slice of anonymous structs via reflection.
	// Type-asserting to a named struct always fails; use reflect to extract fields.
	rv := reflect.ValueOf(entries[0])
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("pair registry getAllActivePairs: unexpected output type %T", entries[0])
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
	return result, nil
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
