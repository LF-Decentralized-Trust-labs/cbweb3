// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// erc20BalanceABI is the minimal read used to observe a Hub token balance.
const erc20BalanceABI = `[
{"type":"function","name":"balanceOf","stateMutability":"view",
 "inputs":[{"name":"account","type":"address"}],
 "outputs":[{"name":"","type":"uint256"}]}
]`

// hubBalanceReader reads ERC-20 balances on the Hub chain. Read-only: it holds no signer, so it
// cannot move anything — the reconciliation observes, it does not act.
type hubBalanceReader struct {
	ec      *ethclient.Client
	timeout time.Duration
}

// newHubBalanceReader dials the Hub. Returns nil when no RPC is configured, which is how a
// gateway without Hub access ends up with no reconciliation rather than a broken one.
func newHubBalanceReader(ctx context.Context, rpcURL string, timeout time.Duration) *hubBalanceReader {
	if rpcURL == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, rpcURL, timeout)
	if err != nil {
		log.Printf("warning: Hub balance reader unavailable (%s): %v — reconciliation disabled", rpcURL, err)
		return nil
	}
	return &hubBalanceReader{ec: ec, timeout: timeout}
}

// BalanceOf returns the holder's balance of an ERC-20 token as a base-unit decimal string.
func (r *hubBalanceReader) BalanceOf(ctx context.Context, tokenAddress, holder string) (string, error) {
	token := common.HexToAddress(tokenAddress)
	acct := common.HexToAddress(holder)
	if token == (common.Address{}) || acct == (common.Address{}) {
		return "", fmt.Errorf("balanceOf: token %q / holder %q must both be addresses", tokenAddress, holder)
	}
	parsed, err := evm.ParseABI(erc20BalanceABI)
	if err != nil {
		return "", fmt.Errorf("balanceOf: parse ABI: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	var bal big.Int
	if err := evm.Call(callCtx, r.ec, token, parsed, "balanceOf", []interface{}{acct}, &bal); err != nil {
		return "", fmt.Errorf("balanceOf(%s) on %s: %w", acct.Hex(), token.Hex(), err)
	}
	return bal.String(), nil
}

// Close releases the RPC connection.
func (r *hubBalanceReader) Close() {
	if r != nil && r.ec != nil {
		r.ec.Close()
	}
}
