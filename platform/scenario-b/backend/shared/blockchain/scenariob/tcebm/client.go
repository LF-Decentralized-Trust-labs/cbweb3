// SPDX-License-Identifier: Apache-2.0

// Package tcebm provides an EVM client for the TokenizedCentralBankMoney (tCeBM) contract.
// The client uses an embedded ABI and go-ethereum/ethclient for ERC-20 + mint/burn calls,
// following the same pattern as the AMM client (T057).
package tcebm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ABIJSON is the minimal ABI for TokenizedCentralBankMoney (ERC-20 + RBAC mint/burn).
const ABIJSON = `[
{"type":"function","name":"decimals","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"uint8"}]},
{"type":"function","name":"symbol","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"string"}]},
{"type":"function","name":"balanceOf","stateMutability":"view","inputs":[
  {"name":"account","type":"address"}
],"outputs":[{"name":"","type":"uint256"}]},
{"type":"function","name":"totalSupply","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"uint256"}]},
{"type":"function","name":"allowance","stateMutability":"view","inputs":[
  {"name":"owner","type":"address"},{"name":"spender","type":"address"}
],"outputs":[{"name":"","type":"uint256"}]},
{"type":"function","name":"hasRole","stateMutability":"view","inputs":[
  {"name":"role","type":"bytes32"},{"name":"account","type":"address"}
],"outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"approve","stateMutability":"nonpayable","inputs":[
  {"name":"spender","type":"address"},{"name":"amount","type":"uint256"}
],"outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"transfer","stateMutability":"nonpayable","inputs":[
  {"name":"to","type":"address"},{"name":"amount","type":"uint256"}
],"outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"mint","stateMutability":"nonpayable","inputs":[
  {"name":"to","type":"address"},{"name":"amount","type":"uint256"}
],"outputs":[]},
{"type":"function","name":"burn","stateMutability":"nonpayable","inputs":[
  {"name":"from","type":"address"},{"name":"amount","type":"uint256"}
],"outputs":[]}
]`

// Config holds the connection parameters for the tCeBM client.
type Config struct {
	RPCURL          string
	ContractAddress string
	ChainID         int64
	PrivateKeyHex   string // optional: required only for state-changing calls
	Timeout         time.Duration
}

// Client is the EVM client for the tCeBM contract on Hub/Spoke networks.
type Client struct {
	contract common.Address
	ec       *ethclient.Client
	abi      abi.ABI
	signer   *evm.Signer
	timeout  time.Duration
}

// NewClient opens an RPC connection and parses the ABI. If PrivateKeyHex is empty the client
// remains read-only (balanceOf only).
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
		return nil, fmt.Errorf("parse tCeBM ABI: %w", err)
	}
	client := &Client{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		abi:      parsed,
		timeout:  cfg.Timeout,
	}
	if cfg.PrivateKeyHex != "" {
		signer, err := evm.SharedSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
		if err != nil {
			return nil, err
		}
		client.signer = signer
	}
	return client, nil
}

// Close releases the underlying RPC connection.
func (c *Client) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// SignerAddress returns the hex address of the configured signing key, or an empty
// string if the client was created without a private key.
func (c *Client) SignerAddress() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
}

// Decimals returns the number of decimal places used by this token (e.g. 18 for standard ERC-20).
func (c *Client) Decimals(ctx context.Context) (uint8, error) {
	var result uint8
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "decimals", nil, &result); err != nil {
		return 0, fmt.Errorf("tcebm: decimals: %w", err)
	}
	return result, nil
}

// Symbol returns the ERC-20 symbol (e.g. "W-tCeBM_BRL"), used to derive the
// currency code of a pair's token for sovereign side resolution.
func (c *Client) Symbol(ctx context.Context) (string, error) {
	var result string
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "symbol", nil, &result); err != nil {
		return "", fmt.Errorf("tcebm: symbol: %w", err)
	}
	return result, nil
}

// BalanceOf returns the token balance of an address as a decimal string.
func (c *Client) BalanceOf(ctx context.Context, address string) (string, error) {
	result := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "balanceOf",
		[]interface{}{common.HexToAddress(address)}, result); err != nil {
		return "", err
	}
	return result.String(), nil
}

// TotalSupply returns the total token supply as a decimal string.
func (c *Client) TotalSupply(ctx context.Context) (string, error) {
	result := new(big.Int)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "totalSupply", nil, result); err != nil {
		return "", err
	}
	return result.String(), nil
}

// Mint mints tokens to the given address. Requires CENTRAL_BANK_ROLE on-chain.
func (c *Client) Mint(ctx context.Context, to string, amount *big.Int) (string, error) {
	if c.signer == nil {
		return "", errors.New("tcebm: mint requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "mint",
		common.HexToAddress(to), amount)
}

// Burn burns tokens from the given address. Requires CENTRAL_BANK_ROLE on-chain.
func (c *Client) Burn(ctx context.Context, from string, amount *big.Int) (string, error) {
	if c.signer == nil {
		return "", errors.New("tcebm: burn requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "burn",
		common.HexToAddress(from), amount)
}

// Approve sets the allowance for a spender. Requires a signing key.
func (c *Client) Approve(ctx context.Context, spender string, amount *big.Int) (string, error) {
	if c.signer == nil {
		return "", errors.New("tcebm: approve requires a signing key")
	}
	return evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "approve",
		common.HexToAddress(spender), amount)
}

// HasCentralBankRole reports whether the signer of this client holds CENTRAL_BANK_ROLE
// on the underlying tCeBM contract. Uses a view eth_call (no gas cost).
// Returns false (not an error) when the signer has no key configured.
func (c *Client) HasCentralBankRole(ctx context.Context) (bool, error) {
	if c.signer == nil {
		return false, nil
	}
	role := crypto.Keccak256Hash([]byte("CENTRAL_BANK_ROLE"))
	var result bool
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "hasRole",
		[]interface{}{role, c.signer.Address()}, &result); err != nil {
		return false, fmt.Errorf("tcebm: hasRole CENTRAL_BANK_ROLE: %w", err)
	}
	return result, nil
}
