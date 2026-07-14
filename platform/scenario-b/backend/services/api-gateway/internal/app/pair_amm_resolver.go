// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	ammclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/amm"
	tcebmclient "github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/tcebm"
)

// pairAMMResolver resolves the AMM + W-token clients for a given pool_pair from
// the on-chain PairRegistry (the source of truth populated by RegisterPairOnChain).
// This makes swap/liquidity DYNAMIC per pair: a corridor opened at runtime (e.g. a
// new Colombia spoke) is usable immediately, with no gateway env or restart.
//
// Clients are cached by contract address (an ethclient dial each), so repeated
// operations on the same pair reuse connections. The pair→addresses map is cached
// and refreshed from GetAllActivePairs on a miss (a newly-created pair).
type pairAMMResolver struct {
	pr        *PairRegistryClient
	rpcURL    string
	chainID   int64
	signerKey string
	timeout   time.Duration

	mu       sync.Mutex
	pairs    map[string]pairAddrs           // pairId → resolved addresses
	ammCache map[string]*ammclient.Client   // ammAddr (lower) → client
	tokCache map[string]*tcebmclient.Client // tokenAddr (lower) → client
}

type pairAddrs struct {
	amm    string
	tokenA string
	tokenB string
}

func newPairAMMResolver(pr *PairRegistryClient, rpcURL string, chainID int64, signerKey string, timeout time.Duration) *pairAMMResolver {
	return &pairAMMResolver{
		pr:        pr,
		rpcURL:    rpcURL,
		chainID:   chainID,
		signerKey: signerKey,
		timeout:   timeout,
		pairs:     map[string]pairAddrs{},
		ammCache:  map[string]*ammclient.Client{},
		tokCache:  map[string]*tcebmclient.Client{},
	}
}

// lookupPair returns the resolved addresses for poolPair, refreshing the cache
// from the on-chain PairRegistry on a miss.
func (r *pairAMMResolver) lookupPair(ctx context.Context, poolPair string) (pairAddrs, error) {
	r.mu.Lock()
	if pa, ok := r.pairs[poolPair]; ok {
		r.mu.Unlock()
		return pa, nil
	}
	r.mu.Unlock()

	entries, err := r.pr.GetAllActivePairs(ctx)
	if err != nil {
		return pairAddrs{}, fmt.Errorf("resolve pair %q: %w", poolPair, err)
	}
	r.mu.Lock()
	for _, e := range entries {
		r.pairs[e.PairID] = pairAddrs{amm: e.AMMAddress, tokenA: e.TokenA, tokenB: e.TokenB}
	}
	pa, ok := r.pairs[poolPair]
	r.mu.Unlock()
	if !ok {
		return pairAddrs{}, fmt.Errorf("resolve pair %q: not found among active pairs", poolPair)
	}
	return pa, nil
}

// primaryClient returns an AMM client bound to the FIRST active pair. It backs the
// genuinely pairless legacy operations (LP-share balance read, withdrawal, and the
// sovereign commit calls whose target AMM address is passed explicitly) now that no
// default/bootstrap AMM exists. With a single corridor this is exactly that pool;
// with multiple corridors it is a documented limitation (see TD-001) — those callers
// predate multi-pair and do not carry a pool_pair. Errors when no active pair exists.
func (r *pairAMMResolver) primaryClient(ctx context.Context) (*ammclient.Client, error) {
	entries, err := r.pr.GetAllActivePairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve primary AMM: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("resolve primary AMM: no active pairs")
	}
	return r.ammFor(ctx, entries[0].PairID)
}

// ammFor returns an AMM client bound to poolPair's on-chain AMM + W-tokens.
func (r *pairAMMResolver) ammFor(ctx context.Context, poolPair string) (*ammclient.Client, error) {
	pa, err := r.lookupPair(ctx, poolPair)
	if err != nil {
		return nil, err
	}
	key := strings.ToLower(pa.amm)
	r.mu.Lock()
	c := r.ammCache[key]
	r.mu.Unlock()
	if c != nil {
		return c, nil
	}
	c, err = ammclient.NewClient(ctx, ammclient.Config{
		RPCURL:          r.rpcURL,
		ContractAddress: pa.amm,
		ChainID:         r.chainID,
		PrivateKeyHex:   r.signerKey,
		Timeout:         r.timeout,
		TokenAAddress:   pa.tokenA,
		TokenBAddress:   pa.tokenB,
	})
	if err != nil {
		return nil, fmt.Errorf("amm client for pair %q: %w", poolPair, err)
	}
	r.mu.Lock()
	r.ammCache[key] = c
	r.mu.Unlock()
	return c, nil
}

// tokensFor returns the W-token clients (A, B) for poolPair.
func (r *pairAMMResolver) tokensFor(ctx context.Context, poolPair string) (*tcebmclient.Client, *tcebmclient.Client, error) {
	pa, err := r.lookupPair(ctx, poolPair)
	if err != nil {
		return nil, nil, err
	}
	tokA, err := r.tokenClient(ctx, pa.tokenA)
	if err != nil {
		return nil, nil, err
	}
	tokB, err := r.tokenClient(ctx, pa.tokenB)
	if err != nil {
		return nil, nil, err
	}
	return tokA, tokB, nil
}

// ammAddressFor returns poolPair's on-chain AMM address (for approvals).
func (r *pairAMMResolver) ammAddressFor(ctx context.Context, poolPair string) (string, error) {
	pa, err := r.lookupPair(ctx, poolPair)
	if err != nil {
		return "", err
	}
	return pa.amm, nil
}

func (r *pairAMMResolver) tokenClient(ctx context.Context, addr string) (*tcebmclient.Client, error) {
	key := strings.ToLower(addr)
	r.mu.Lock()
	c := r.tokCache[key]
	r.mu.Unlock()
	if c != nil {
		return c, nil
	}
	c, err := tcebmclient.NewClient(ctx, tcebmclient.Config{
		RPCURL:          r.rpcURL,
		ContractAddress: addr,
		ChainID:         r.chainID,
		PrivateKeyHex:   r.signerKey,
		Timeout:         r.timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("token client %s: %w", addr, err)
	}
	r.mu.Lock()
	r.tokCache[key] = c
	r.mu.Unlock()
	return c, nil
}
