// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// LocalLister yields this node's own live FX-party roster (its Pente group
// membership). Implemented by the payment-orchestrator gRPC adapter.
type LocalLister interface {
	ListParticipantIdentities(ctx context.Context) ([]string, error)
}

// SpokeLister discovers the consortium's spokes. Implemented by *Client.
type SpokeLister interface {
	ListSpokes(ctx context.Context) ([]Spoke, error)
}

// PeerFetcher fetches a peer gateway's LOCAL roster. Implemented by *Client.
type PeerFetcher interface {
	FetchPeerIdentities(ctx context.Context, baseURL string) ([]string, error)
}

// FederatedRoster resolves the network-wide FX-party roster without any
// hand-maintained cross-spoke list. It unions:
//
//   - this node's own live Pente membership (LocalLister), and
//   - every peer spoke's local roster, discovered from the relay's spoke
//     registry (SpokeLister) and fetched from each peer gateway (PeerFetcher).
//
// The registry's internalApiUrl for every spoke points at that spoke's CENTRAL
// BANK gateway, and a CB authoritatively knows its whole spoke's identities (it
// shares a bilateral Pente group with every one of its banks). So fanning out to
// each spoke's CB gateway yields the full consortium roster — including sibling
// banks a commercial node cannot see locally (its own Pente membership is only
// the bank↔CB group). Every spoke's CB gateway is queried, this node's own
// included: the ?scope=local peer request returns local membership only and does
// NOT re-federate, so a self-call terminates and is deduped. A spoke that joins
// later registers with the relay and appears everywhere on the next refresh — no
// manifest edit anywhere.
//
// It implements the handlers.IdentityRosterProvider interface
// (ListParticipantIdentities), so it drops into the existing IdentityRoster in
// place of the plain live provider.
type FederatedRoster struct {
	local    LocalLister
	spokes   SpokeLister
	peers    PeerFetcher
	cacheTTL time.Duration
	nowFn    func() time.Time

	mu         sync.Mutex
	cached     []string
	cachedAt   time.Time
	cacheValid bool
}

// NewFederatedRoster builds a federated roster. cacheTTL bounds how often the
// relay and peers are hit; <=0 defaults to 10s.
func NewFederatedRoster(local LocalLister, spokes SpokeLister, peers PeerFetcher, cacheTTL time.Duration) *FederatedRoster {
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Second
	}
	return &FederatedRoster{
		local:    local,
		spokes:   spokes,
		peers:    peers,
		cacheTTL: cacheTTL,
		nowFn:    time.Now,
	}
}

// ListParticipantIdentities returns the deduplicated, sorted union of local
// membership and every reachable peer spoke's local roster.
//
// Best-effort by design: a relay-registry error or a single unreachable peer
// degrades the result to what could be gathered (with a warning) rather than
// failing the whole roster — the local spoke's own identities always survive a
// relay outage. The only hard error is when the local source itself fails AND
// nothing else could be collected, so the caller can fail closed.
func (f *FederatedRoster) ListParticipantIdentities(ctx context.Context) ([]string, error) {
	if cached, ok := f.fromCache(); ok {
		return cached, nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0)
	add := func(ids []string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}

	local, localErr := f.local.ListParticipantIdentities(ctx)
	if localErr != nil {
		log.Printf("warning: local Pente membership unavailable for federated roster: %v", localErr)
	} else {
		add(local)
	}

	spokes, err := f.spokes.ListSpokes(ctx)
	if err != nil {
		log.Printf("warning: relay spoke registry unavailable, federated roster limited to local membership: %v", err)
		if len(out) == 0 && localErr != nil {
			return nil, localErr
		}
		return f.store(out), nil
	}

	for _, sp := range spokes {
		if sp.InternalApiURL == "" {
			continue
		}
		ids, perr := f.peers.FetchPeerIdentities(ctx, sp.InternalApiURL)
		if perr != nil {
			// One unreachable peer must not blank the whole network roster.
			log.Printf("warning: spoke %q roster unavailable, skipping it: %v", sp.ID, perr)
			continue
		}
		add(ids)
	}

	if len(out) == 0 && localErr != nil {
		return nil, localErr
	}
	return f.store(out), nil
}

// fromCache returns the cached roster if it is still within the TTL.
func (f *FederatedRoster) fromCache() ([]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.cacheValid || f.nowFn().Sub(f.cachedAt) > f.cacheTTL {
		return nil, false
	}
	cp := make([]string, len(f.cached))
	copy(cp, f.cached)
	return cp, true
}

// store sorts, caches, and returns the roster.
func (f *FederatedRoster) store(out []string) []string {
	sort.Strings(out)
	f.mu.Lock()
	f.cached = out
	f.cachedAt = f.nowFn()
	f.cacheValid = true
	f.mu.Unlock()
	return out
}
