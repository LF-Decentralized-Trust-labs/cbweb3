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
	currency   string // ISO code of this CB's currency, e.g. "BRL"
	preferred  string // expected wrapped symbol, e.g. "W-tCeBM_BRL"
	timeout    time.Duration

	mu     sync.Mutex
	symbol string // wrapped symbol as registered on-chain (cached with the client)
	addr   string // resolved token address (cached after the first successful lookup)
	client *tcebmclient.Client
}

// newSovereignSupplyAdapter builds the adapter for nativeAssetSymbol (e.g. "tCeBM_BRL").
// Returns nil when it cannot possibly work, so the route stays unregistered instead of
// serving errors.
//
// Only the ISO code is taken from the symbol. The hub names a wrapped token after the
// currency alone ("W-tCeBM_"+currency, hub compliance RegisterCurrencyOnChain) — the
// spoke's own token symbol never reaches it — so a spoke that deploys its tCeBM under
// another prefix (manifests document the shape as "<prefix>_<ISO>" and only pin the
// ISO segment) must still resolve to the same hub token.
func newSovereignSupplyAdapter(currencies services.CurrencyServiceIface, rpcURL, nativeAssetSymbol string, timeout time.Duration) *sovereignSupplyAdapter {
	native := strings.TrimSpace(nativeAssetSymbol)
	if currencies == nil || strings.TrimSpace(rpcURL) == "" || native == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	currency := currencyCodeFromSymbol(native)
	return &sovereignSupplyAdapter{
		currencies: currencies,
		rpcURL:     rpcURL,
		currency:   currency,
		preferred:  "W-tCeBM_" + currency,
		timeout:    timeout,
	}
}

// SovereignSupply implements handlers.SovereignSupplyReader.
func (a *sovereignSupplyAdapter) SovereignSupply(ctx context.Context) (*handlers.SovereignSupply, error) {
	client, symbol, addr, err := a.resolve(ctx)
	if err != nil {
		return nil, err
	}

	total, err := client.TotalSupply(ctx)
	if err != nil {
		return nil, fmt.Errorf("totalSupply %s (%s): %w", symbol, addr, err)
	}

	decimals := defaultTokenDecimals
	if d, derr := client.Decimals(ctx); derr == nil {
		decimals = d
	} else {
		log.Printf("warning: decimals() failed for %s (%s), assuming %d: %v", symbol, addr, defaultTokenDecimals, derr)
	}

	return &handlers.SovereignSupply{
		Symbol:       symbol,
		TokenAddress: addr,
		TotalSupply:  total,
		Decimals:     decimals,
	}, nil
}

// resolve returns a cached read-only client for this CB's wrapped token, looking the
// address up in the CurrencyRegistry on the first call (or after a failed lookup).
//
// The registry read and the RPC dial run OUTSIDE the mutex: both are network calls to
// the hub (cross-VM in a multi-host deployment), and holding the lock across them would
// queue every caller of this public endpoint behind one dial timeout while the hub is
// unreachable. Two concurrent misses may therefore both dial; the loser closes its
// client so the connection is not leaked.
func (a *sovereignSupplyAdapter) resolve(ctx context.Context) (*tcebmclient.Client, string, string, error) {
	a.mu.Lock()
	client, symbol, addr := a.client, a.symbol, a.addr
	a.mu.Unlock()
	if client != nil {
		return client, symbol, addr, nil
	}

	symbol, addr, err := a.lookup(ctx)
	if err != nil {
		return nil, "", "", err
	}

	client, err = tcebmclient.NewClient(ctx, tcebmclient.Config{
		RPCURL:          a.rpcURL,
		ContractAddress: addr,
		Timeout:         a.timeout,
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("dial hub token %s (%s): %w", symbol, addr, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil { // another caller resolved first — keep one connection
		client.Close()
		return a.client, a.symbol, a.addr, nil
	}
	a.client, a.symbol, a.addr = client, symbol, addr
	return client, symbol, addr, nil
}

// lookup finds this CB's wrapped token in the hub CurrencyRegistry, returning the symbol
// as actually registered on-chain and its address.
//
// The expected name is matched first; the fallback accepts any "W-…_<CUR>" entry so a
// change to the hub's naming does not silently blank the card.
func (a *sovereignSupplyAdapter) lookup(ctx context.Context) (string, string, error) {
	entries, err := a.currencies.ListCurrencies(ctx)
	if err != nil {
		return "", "", fmt.Errorf("list hub currencies: %w", err)
	}
	suffix := "_" + a.currency
	fallbackSym, fallbackAddr := "", ""
	for _, e := range entries {
		sym := strings.TrimSpace(e.Symbol)
		if strings.EqualFold(sym, a.preferred) {
			return sym, strings.TrimSpace(e.TokenAddress), nil
		}
		upper := strings.ToUpper(sym)
		if fallbackAddr == "" && strings.HasPrefix(upper, "W-") && strings.HasSuffix(upper, suffix) {
			fallbackSym, fallbackAddr = sym, strings.TrimSpace(e.TokenAddress)
		}
	}
	if fallbackAddr != "" {
		log.Printf("[app] hub wrapped token for %s registered as %q, not %q — using the registered symbol",
			a.currency, fallbackSym, a.preferred)
		return fallbackSym, fallbackAddr, nil
	}
	return "", "", fmt.Errorf("no wrapped %s token is registered in the hub CurrencyRegistry (expected %s)", a.currency, a.preferred)
}
