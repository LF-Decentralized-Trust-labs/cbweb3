// SPDX-License-Identifier: Apache-2.0

package relayauth

import (
	"encoding/asn1"
	"encoding/base64"
	"math/big"
	"strings"
	"sync"
	"time"
)

// ReplayGuard refuses a signature that has already authenticated a request.
//
// WHY IT IS NEEDED. The canonical string binds method, path, body and timestamp, which makes a
// captured signature non-transferable to a different request — but perfectly valid for the SAME
// request, for as long as its timestamp stays inside the skew window. Whether that matters depends
// on the route: most internal endpoints deduplicate downstream (a bridge-in is keyed by its
// correlation id, a hub swap by its position claim), so a repeat is absorbed. The transfer-limit pair
// does not. CheckAndDeduct accumulates and Restore subtracts, so replaying a captured restore drives
// a bank's recorded daily volume toward zero and lets it transact past the limit its central bank
// configured. That is a compliance control undone by resending bytes, so the deferral that fits the
// other routes does not fit this one.
//
// WHY A SEEN-SIGNATURE CACHE RATHER THAN A NONCE. A nonce would have to enter the canonical string,
// which means changing the protocol on both ends — the Go signer and the relay's TypeScript one —
// and every peer would have to move at once. The cache needs no protocol change and costs no false
// rejections: ECDSA signing is randomized, so a legitimate signer never emits the same signature
// twice. Two genuine calls with an identical body in the same second still carry different
// signatures; a signature seen twice is a replay by construction.
//
// WHY THE CACHE KEY IS r, NOT THE SIGNATURE BYTES. An ECDSA signature is malleable: (r, s) and
// (r, n-s) are both valid over the same message, and turning one into the other needs no private
// key. Keying on the encoded signature would therefore have let a captured request through on its
// second form — the exact bypass the cache exists to prevent. r is the x-coordinate of kG, so it is
// fixed by the nonce the signer chose: it survives malleability and re-encoding, and two genuine
// signatures differ in it because they differ in k. (A repeated r would mean a reused k, which
// leaks the private key outright — a far larger problem than a replay.)
//
// BOUNDED BY CONSTRUCTION. Only a signature that already verified is recorded, so an attacker with no
// pinned private key cannot grow the map at all. Its size is therefore the number of signed requests
// a legitimate peer makes within one skew window, and entries past that window are pruned — past it
// the verifier rejects the timestamp anyway, so remembering longer would protect nothing.
type ReplayGuard struct {
	mu        sync.Mutex
	seen      map[string]time.Time
	ttl       time.Duration
	lastPrune time.Time
}

// NewReplayGuard returns a guard that remembers each accepted signature for ttl — which should be
// the verifier's skew window, since a signature is worthless to a replayer once it is outside it.
func NewReplayGuard(ttl time.Duration) *ReplayGuard {
	if ttl <= 0 {
		ttl = DefaultMaxSkew
	}
	return &ReplayGuard{seen: map[string]time.Time{}, ttl: ttl}
}

// Admit records this signature against its signer and reports whether this is its FIRST use. A false
// return means the request is a replay and must be rejected.
//
// Call it only AFTER the signature has verified: an unverified one must never be able to put an entry
// in the map, or an attacker could grow it at will and — worse — poison the identity of a signature
// the legitimate peer has not sent yet.
//
// A nil guard admits: a gateway wired without one is in the pre-existing state, and turning an
// unconfigured cache into a wall of 401s would be a worse failure than the replay it prevents.
func (g *ReplayGuard) Admit(keyID, signatureB64 string, now time.Time) bool {
	if g == nil {
		return true
	}
	key := keyID + "\x00" + signatureIdentity(signatureB64)

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.seen == nil {
		g.seen = map[string]time.Time{}
	}
	g.pruneLocked(now)

	if seenAt, ok := g.seen[key]; ok && now.Sub(seenAt) < g.ttl {
		return false
	}
	g.seen[key] = now
	return true
}

// Len reports how many signatures are currently remembered. For tests and for an operator counting
// the cost of the cache.
func (g *ReplayGuard) Len() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.seen)
}

// signatureIdentity reduces a signature to the part a replayer cannot change: r.
//
// Falls back to the trimmed encoding when the bytes do not parse as an ECDSA signature. That is
// unreachable in the request path — Admit is called only after verification, which parsed them — but
// the fallback keeps the guard usable (and testable) with opaque tokens rather than silently keying
// everything to one bucket.
func signatureIdentity(signatureB64 string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureB64))
	if err != nil {
		return strings.TrimSpace(signatureB64)
	}
	var parsed struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(raw, &parsed); err != nil || parsed.R == nil {
		return strings.TrimSpace(signatureB64)
	}
	return parsed.R.Text(16)
}

// pruneLocked drops entries older than the window. Rate-limited to once per half-window so a busy
// gateway does not walk the whole map on every request; the map only ever holds what one window's
// legitimate traffic put there, so a late sweep costs memory, never correctness.
func (g *ReplayGuard) pruneLocked(now time.Time) {
	if now.Sub(g.lastPrune) < g.ttl/2 {
		return
	}
	g.lastPrune = now
	for k, at := range g.seen {
		if now.Sub(at) >= g.ttl {
			delete(g.seen, k)
		}
	}
}
