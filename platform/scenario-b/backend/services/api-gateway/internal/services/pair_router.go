// SPDX-License-Identifier: Apache-2.0

// Package services provides PairRouter — the in-memory routing cache for multi-pair AMM support.
// PairRouter maps pool_pair strings (e.g. "BRL-USD") to their dedicated AMM client instances.
// It is initialized at startup from the on-chain PairRegistry.getAllActivePairs() and
// updated in real time via a PairRegistered event subscription goroutine (D10 — 005-cooperative-liquidity).
package services

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// PairActivatedEvent carries the routing information for a newly active pair.
// It is consumed by PairRouter.handleActivation to update the cache.
type PairActivatedEvent struct {
	PairID     string
	AMMAddress string
	TokenA     string
	TokenB     string
}

// PairSource is the interface PairRouter uses to read initial state and subscribe to events.
// Implemented by app.PairRegistryClient (D10).
type PairSource interface {
	GetAllActivePairs(ctx context.Context) ([]domain.PairEntry, error)
}

// AMMClientFactory creates a minimal AMM client for a given pair.
// The factory receives the AMM contract address for the pair and returns
// an AMMPoolReader usable by existing swap/quote/pool-status services.
// Callers (app.go) provide the factory; PairRouter uses it to populate the cache.
type AMMClientFactory func(ctx context.Context, ammAddress string) (AMMPoolReader, error)

// PairRouter maintains a concurrency-safe map from pair_id → AMMPoolReader.
// All reads are protected by a shared RWMutex for zero-contention quote paths.
type PairRouter struct {
	mu      sync.RWMutex
	cache   map[string]AMMPoolReader
	factory AMMClientFactory
}

// NewPairRouter constructs and initializes the PairRouter.
// It fetches all active pairs from the on-chain PairRegistry at startup
// and stores them in the cache using the provided factory.
//
// The event goroutine (SubscribePairRegistered) should be started separately
// by the caller after construction using WatchActivations.
func NewPairRouter(ctx context.Context, source PairSource, factory AMMClientFactory) (*PairRouter, error) {
	r := &PairRouter{
		cache:   make(map[string]AMMPoolReader),
		factory: factory,
	}
	pairs, err := source.GetAllActivePairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("pair router: load initial pairs: %w", err)
	}
	for _, p := range pairs {
		client, err := factory(ctx, p.AMMAddress)
		if err != nil {
			log.Printf("pair router: warning: failed to create client for pair %s (amm=%s): %v",
				p.PairID, p.AMMAddress, err)
			continue
		}
		r.cache[p.PairID] = client
		log.Printf("pair router: loaded pair %s → %s", p.PairID, p.AMMAddress)
	}
	return r, nil
}

// ClientFor returns the AMMPoolReader for the given pair (e.g. "BRL-USD").
// Returns nil, error if the pair is not registered or not yet active.
func (r *PairRouter) ClientFor(pairID string) (AMMPoolReader, error) {
	r.mu.RLock()
	client, ok := r.cache[pairID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("pair router: pair %q not found or not yet active", pairID)
	}
	return client, nil
}

// AddPair registers a new active pair in the cache.
// Called by WatchActivations when a PairRegistered event arrives.
func (r *PairRouter) AddPair(ctx context.Context, pairID, ammAddress string) {
	client, err := r.factory(ctx, ammAddress)
	if err != nil {
		log.Printf("pair router: failed to create client for new pair %s (amm=%s): %v",
			pairID, ammAddress, err)
		return
	}
	r.mu.Lock()
	r.cache[pairID] = client
	r.mu.Unlock()
	log.Printf("pair router: registered new pair %s → %s", pairID, ammAddress)
}

// ListPairIDs returns all pair_ids currently registered (active).
// Used by pair_handler.go to resolve pair entries from DB without an RPC call (D11).
func (r *PairRouter) ListPairIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.cache))
	for id := range r.cache {
		ids = append(ids, id)
	}
	return ids
}

// WatchActivations starts a goroutine that listens on eventCh for new ACTIVE pairs
// and calls AddPair to update the routing cache without restart (D10 / FR-016).
// The goroutine exits when eventCh is closed or ctx is cancelled.
func (r *PairRouter) WatchActivations(ctx context.Context, eventCh <-chan PairActivatedEvent) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-eventCh:
				if !ok {
					return
				}
				r.AddPair(ctx, ev.PairID, ev.AMMAddress)
			}
		}
	}()
}
