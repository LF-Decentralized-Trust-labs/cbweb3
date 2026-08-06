// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	tcebmclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/tcebm"
)

// defaultTokenDecimals is the ERC-20 decimals assumed when the on-chain decimals()
// call fails. Every tCeBM deployment in Scenario B is an 18-decimal OpenZeppelin
// ERC-20, so this only ever applies to a transient RPC hiccup — and it is logged.
const defaultTokenDecimals uint8 = 18

// sovereignSupplyAdapter reads the total supply of THIS Central Bank's own wrapped
// token on the hub (W-tCeBM_<CUR>). The token address is resolved from the on-chain
// CurrencyRegistry rather than from env, so a currency registered at runtime is
// readable with no gateway restart.
//
// The client is read-only (no private key): totalSupply is a view call, and giving
// this path a signer would let a read endpoint hold a spending key for no reason.
type sovereignSupplyAdapter struct {
	currencies services.CurrencyServiceIface
	rpcURL     string
	symbol     string // wrapped symbol, e.g. "W-tCeBM_BRL"
	timeout    time.Duration

	mu     sync.Mutex
	addr   string // resolved token address (cached after the first successful lookup)
	client *tcebmclient.Client
}

// newSovereignSupplyAdapter builds the adapter for nativeAssetSymbol (e.g. "tCeBM_BRL").
// Returns nil when it cannot possibly work, so the route stays unregistered instead of
// serving errors.
func newSovereignSupplyAdapter(currencies services.CurrencyServiceIface, rpcURL, nativeAssetSymbol string, timeout time.Duration) *sovereignSupplyAdapter {
	native := strings.TrimSpace(nativeAssetSymbol)
	if currencies == nil || strings.TrimSpace(rpcURL) == "" || native == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &sovereignSupplyAdapter{
		currencies: currencies,
		rpcURL:     rpcURL,
		symbol:     "W-" + native,
		timeout:    timeout,
	}
}

// SovereignSupply implements handlers.SovereignSupplyReader.
func (a *sovereignSupplyAdapter) SovereignSupply(ctx context.Context) (*handlers.SovereignSupply, error) {
	client, addr, err := a.resolve(ctx)
	if err != nil {
		return nil, err
	}

	total, err := client.TotalSupply(ctx)
	if err != nil {
		return nil, fmt.Errorf("totalSupply %s (%s): %w", a.symbol, addr, err)
	}

	decimals := defaultTokenDecimals
	if d, derr := client.Decimals(ctx); derr == nil {
		decimals = d
	} else {
		log.Printf("warning: decimals() failed for %s (%s), assuming %d: %v", a.symbol, addr, defaultTokenDecimals, derr)
	}

	return &handlers.SovereignSupply{
		Symbol:       a.symbol,
		TokenAddress: addr,
		TotalSupply:  total,
		Decimals:     decimals,
	}, nil
}

// resolve returns a cached read-only client for this CB's wrapped token, looking the
// address up in the CurrencyRegistry on the first call (or after a failed lookup).
func (a *sovereignSupplyAdapter) resolve(ctx context.Context) (*tcebmclient.Client, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil {
		return a.client, a.addr, nil
	}

	entries, err := a.currencies.ListCurrencies(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("list hub currencies: %w", err)
	}
	addr := ""
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.Symbol), a.symbol) {
			addr = strings.TrimSpace(e.TokenAddress)
			break
		}
	}
	if addr == "" {
		return nil, "", fmt.Errorf("%s is not registered in the hub CurrencyRegistry", a.symbol)
	}

	client, err := tcebmclient.NewClient(ctx, tcebmclient.Config{
		RPCURL:          a.rpcURL,
		ContractAddress: addr,
		Timeout:         a.timeout,
	})
	if err != nil {
		return nil, "", fmt.Errorf("dial hub token %s (%s): %w", a.symbol, addr, err)
	}
	a.client, a.addr = client, addr
	return client, addr, nil
}
