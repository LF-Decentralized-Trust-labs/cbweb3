// Package app — CurrencyRegistry EVM adapter.
// Wraps the on-chain CurrencyRegistry contract (contracts/src/CurrencyRegistry.sol) for use by
// currency_service.go (006-hub-currency-registry).
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
	"github.com/ethereum/go-ethereum/ethclient"
)

// currencyRegistryABI is the minimal ABI for CurrencyRegistry.sol.
// Keep in sync with contracts/src/CurrencyRegistry.sol (006-hub-currency-registry).
const currencyRegistryABI = `[
{"type":"function","name":"registerCurrency","stateMutability":"nonpayable","inputs":[
  {"name":"symbol","type":"string"},
  {"name":"countryName","type":"string"},
  {"name":"tokenAddress","type":"address"},
  {"name":"proposerCB","type":"string"}
],"outputs":[]},
{"type":"function","name":"removeCurrency","stateMutability":"nonpayable","inputs":[
  {"name":"symbol","type":"string"}
],"outputs":[]},
{"type":"function","name":"getCurrency","stateMutability":"view","inputs":[
  {"name":"symbol","type":"string"}
],"outputs":[
  {"name":"","type":"tuple","components":[
    {"name":"symbol","type":"string"},
    {"name":"countryName","type":"string"},
    {"name":"tokenAddress","type":"address"},
    {"name":"proposerCB","type":"string"}
  ]}
]},
{"type":"function","name":"getAllCurrencies","stateMutability":"view","inputs":[],"outputs":[
  {"name":"","type":"tuple[]","components":[
    {"name":"symbol","type":"string"},
    {"name":"countryName","type":"string"},
    {"name":"tokenAddress","type":"address"},
    {"name":"proposerCB","type":"string"}
  ]}
]}
]`

// CurrencyRegistryClient is an EVM client for CurrencyRegistry.sol.
type CurrencyRegistryClient struct {
	contract common.Address
	ec       *ethclient.Client
	parsed   abi.ABI
	signer   *evm.Signer
	timeout  time.Duration
}

// CurrencyRegistryConfig holds connection parameters for the CurrencyRegistry EVM client.
type CurrencyRegistryConfig struct {
	RPCURL          string
	ContractAddress string
	ChainID         int64
	PrivateKeyHex   string
	Timeout         time.Duration
}

// NewCurrencyRegistryClient constructs a CurrencyRegistryClient from configuration.
func NewCurrencyRegistryClient(ctx context.Context, cfg CurrencyRegistryConfig) (*CurrencyRegistryClient, error) {
	if cfg.ContractAddress == "" {
		return nil, fmt.Errorf("CurrencyRegistryConfig: ContractAddress is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, cfg.RPCURL, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("currency registry: dial: %w", err)
	}
	parsed, err := evm.ParseABI(currencyRegistryABI)
	if err != nil {
		ec.Close()
		return nil, fmt.Errorf("currency registry: parse ABI: %w", err)
	}
	c := &CurrencyRegistryClient{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		parsed:   parsed,
		timeout:  cfg.Timeout,
	}
	if cfg.PrivateKeyHex != "" {
		signer, sigErr := evm.NewSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
		if sigErr != nil {
			ec.Close()
			return nil, fmt.Errorf("currency registry: signer: %w", sigErr)
		}
		c.signer = signer
	}
	return c, nil
}

// Close releases the underlying RPC connection.
func (c *CurrencyRegistryClient) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// RegisterCurrency submits a registerCurrency transaction.
func (c *CurrencyRegistryClient) RegisterCurrency(
	ctx context.Context,
	symbol, countryName, tokenAddress, proposerCB string,
) (string, error) {
	if c.signer == nil {
		return "", fmt.Errorf("currency registry: registerCurrency requires a signing key")
	}
	txHash, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.parsed, "registerCurrency",
		symbol,
		countryName,
		common.HexToAddress(tokenAddress),
		proposerCB,
	)
	if err != nil {
		return "", fmt.Errorf("currency registry registerCurrency: %w", err)
	}
	return txHash, nil
}

// RemoveCurrency submits a removeCurrency transaction.
func (c *CurrencyRegistryClient) RemoveCurrency(ctx context.Context, symbol string) (string, error) {
	if c.signer == nil {
		return "", fmt.Errorf("currency registry: removeCurrency requires a signing key")
	}
	txHash, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.parsed, "removeCurrency", symbol)
	if err != nil {
		return "", fmt.Errorf("currency registry removeCurrency: %w", err)
	}
	return txHash, nil
}

// GetAllCurrencies reads all registered currencies from on-chain.
func (c *CurrencyRegistryClient) GetAllCurrencies(ctx context.Context) ([]domain.CurrencyEntry, error) {
	input, err := c.parsed.Pack("getAllCurrencies")
	if err != nil {
		return nil, fmt.Errorf("currency registry getAllCurrencies pack: %w", err)
	}
	msg := ethereum.CallMsg{To: &c.contract, Data: input}
	raw, err := c.ec.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("currency registry getAllCurrencies call: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	method := c.parsed.Methods["getAllCurrencies"]
	entries, err := method.Outputs.Unpack(raw)
	if err != nil {
		return nil, fmt.Errorf("currency registry getAllCurrencies unpack: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// go-ethereum unpacks tuple[] as a slice of anonymous structs via reflection.
	rv := reflect.ValueOf(entries[0])
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("currency registry getAllCurrencies: unexpected output type %T", entries[0])
	}

	result := make([]domain.CurrencyEntry, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}
		sym := item.FieldByName("Symbol")
		country := item.FieldByName("CountryName")
		tokenAddr := item.FieldByName("TokenAddress")
		proposer := item.FieldByName("ProposerCB")
		if !sym.IsValid() || !country.IsValid() || !tokenAddr.IsValid() || !proposer.IsValid() {
			continue
		}
		result = append(result, domain.CurrencyEntry{
			Symbol:       sym.String(),
			CountryName:  country.String(),
			TokenAddress: strings.ToLower(tokenAddr.Interface().(common.Address).Hex()),
			ProposerCB:   proposer.String(),
		})
	}
	return result, nil
}
