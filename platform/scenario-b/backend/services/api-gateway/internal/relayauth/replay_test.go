// SPDX-License-Identifier: Apache-2.0

package relayauth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
)

// A signature is valid for as long as its timestamp is inside the skew window, so a captured request
// can be sent again within five minutes and verifies exactly as the original did. For most internal
// routes that only repeats an operation the receiver deduplicates. For the daily transfer limit it
// does not: Deduct accumulates and Restore subtracts, so a replayed restore drives a bank's recorded
// daily volume toward zero and lets it transact past the limit its central bank configured — a
// compliance control undone by resending bytes.
//
// The guard closes it without a protocol change: an ECDSA signature is randomized, so a legitimate
// signer never produces the same one twice. A signature seen a second time is therefore a replay by
// definition, never a genuine repeat of the same intent.

func TestReplayGuard_AdmitsASignatureOnce(t *testing.T) {
	g := relayauth.NewReplayGuard(5 * time.Minute)
	now := time.Unix(1_760_000_000, 0)

	if !g.Admit("bank-a", "sig-1", now) {
		t.Fatal("the first use of a signature must be admitted")
	}
	if g.Admit("bank-a", "sig-1", now.Add(time.Second)) {
		t.Fatal("the same signature must not authenticate a second request — that is a replay")
	}
}

func TestReplayGuard_DistinguishesSignaturesAndSigners(t *testing.T) {
	g := relayauth.NewReplayGuard(5 * time.Minute)
	now := time.Unix(1_760_000_000, 0)

	if !g.Admit("bank-a", "sig-1", now) {
		t.Fatal("first admission failed")
	}
	if !g.Admit("bank-a", "sig-2", now) {
		t.Fatal("a different signature is a different request — two genuine calls in the same second " +
			"must both go through")
	}
	if !g.Admit("bank-b", "sig-1", now) {
		t.Fatal("another signer's signature is not this signer's replay")
	}
}

func TestReplayGuard_ForgetsOnceTheSkewWindowHasPassed(t *testing.T) {
	// Past the window the verifier rejects the timestamp anyway, so remembering longer only grows the
	// map. Forgetting is what bounds it.
	g := relayauth.NewReplayGuard(time.Minute)
	now := time.Unix(1_760_000_000, 0)

	if !g.Admit("bank-a", "sig-1", now) {
		t.Fatal("first admission failed")
	}
	if !g.Admit("bank-a", "sig-1", now.Add(2*time.Minute)) {
		t.Fatal("an entry older than the window must have been dropped")
	}
	if g.Len() > 1 {
		t.Fatalf("guard holds %d entries; want the expired one pruned", g.Len())
	}
}

// An ECDSA signature is malleable: negating s modulo the curve order yields a DIFFERENT encoding
// that verifies over the SAME message, and producing it needs no private key. A guard keyed on the
// signature bytes would have admitted that second form — a captured request replayed past the very
// check meant to stop it. Keying on r closes it, because malleability cannot touch r.
func TestReplayGuard_RejectsAMalleatedCopyOfAnAdmittedSignature(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	ts := int64(1_760_000_000)
	body := []byte(`{"payer_bank_id":"bank-a","amount_human":"1000"}`)

	original, err := relayauth.Sign(key, ts, "POST", "/internal/v2/transfer-limits/restore", body)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Flip s → n-s and re-encode. No private key involved.
	raw, err := base64.StdEncoding.DecodeString(original)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var parsed struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse signature: %v", err)
	}
	flipped := struct{ R, S *big.Int }{R: parsed.R, S: new(big.Int).Sub(elliptic.P256().Params().N, parsed.S)}
	reencoded, err := asn1.Marshal(flipped)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	malleated := base64.StdEncoding.EncodeToString(reencoded)

	if malleated == original {
		t.Fatal("the malleated form must differ from the original, or the test proves nothing")
	}
	// The premise: the receiver's own verifier accepts it, so only the guard stands between a
	// captured request and a second execution.
	if err := relayauth.Verify(&key.PublicKey, ts, "POST", "/internal/v2/transfer-limits/restore", body, malleated); err != nil {
		t.Skipf("this Go version rejects the malleated form at verification (%v), so the guard is not the "+
			"control being tested here", err)
	}

	g := relayauth.NewReplayGuard(5 * time.Minute)
	now := time.Unix(ts, 0)
	if !g.Admit("bank-a", original, now) {
		t.Fatal("the original must be admitted")
	}
	if g.Admit("bank-a", malleated, now) {
		t.Fatal("a malleated copy of an admitted signature is the same request replayed and must be refused")
	}
}

// --- shared store (multi-instance) ---
//
// The in-memory guard protects one process. A central bank that runs its gateway behind a load
// balancer with more than one replica gets no protection at all from it: the capture goes to the
// other replica, which has never seen the signature. That matters most for the route the guard was
// added for — a replayed transfer-limit restore credits a bank's daily allowance back — so on a
// replicated deployment the compliance control would silently be back where it started.

type fakeSeenStore struct {
	seen  map[string]bool
	calls int
	err   error
}

func newFakeSeenStore() *fakeSeenStore { return &fakeSeenStore{seen: map[string]bool{}} }

func (f *fakeSeenStore) Admit(_ context.Context, key string, _ time.Duration) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	if f.seen[key] {
		return false, nil
	}
	f.seen[key] = true
	return true, nil
}

func TestReplayGuard_SharedStoreRefusesAcrossInstances(t *testing.T) {
	shared := newFakeSeenStore()
	// Two guards, one store: the two replicas of one central bank's gateway.
	a := relayauth.NewReplayGuard(5 * time.Minute).WithShared(shared)
	b := relayauth.NewReplayGuard(5 * time.Minute).WithShared(shared)
	now := time.Unix(1_760_000_000, 0)

	if !a.Admit("bank-a", "sig-1", now) {
		t.Fatal("the first use must be admitted")
	}
	if b.Admit("bank-a", "sig-1", now) {
		t.Fatal("the OTHER replica must refuse the same signature — that is the whole point of sharing")
	}
}

func TestReplayGuard_LocalReplayDoesNotConsultTheSharedStore(t *testing.T) {
	// A signature this process already saw is decided locally: no round-trip, and no way for a Redis
	// hiccup to turn a known replay into an admission.
	shared := newFakeSeenStore()
	g := relayauth.NewReplayGuard(5 * time.Minute).WithShared(shared)
	now := time.Unix(1_760_000_000, 0)

	g.Admit("bank-a", "sig-1", now)
	before := shared.calls
	if g.Admit("bank-a", "sig-1", now) {
		t.Fatal("a locally known replay must be refused")
	}
	if shared.calls != before {
		t.Fatalf("shared store consulted %d extra time(s); a local replay needs no round-trip",
			shared.calls-before)
	}
}

func TestReplayGuard_SharedStoreFailureFallsBackToLocal(t *testing.T) {
	// Redis down must not stop settlement: every internal route rides this middleware, so refusing
	// on a store error would take bridge-in, the hub swap and the residue return down over a cache.
	// The local guard still holds within the process, which is what the deployment had before.
	shared := newFakeSeenStore()
	shared.err = errors.New("connection refused")
	g := relayauth.NewReplayGuard(5 * time.Minute).WithShared(shared)
	now := time.Unix(1_760_000_000, 0)

	if !g.Admit("bank-a", "sig-1", now) {
		t.Fatal("an unreachable shared store must not refuse verified traffic")
	}
	if g.Admit("bank-a", "sig-1", now) {
		t.Fatal("the local guard must still refuse the replay while the shared store is down")
	}
}

func TestRedisSeenStore_AdmitsOnce(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set — skipping the real Redis round-trip (set it to a reachable Redis to run)")
	}
	store := relayauth.NewRedisSeenStore(addr, os.Getenv("REDIS_PASSWORD"), 0)
	defer func() { _ = store.Close() }()

	key := "test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	first, err := store.Admit(context.Background(), key, 30*time.Second)
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}
	if !first {
		t.Fatal("a key never seen must be admitted")
	}
	second, err := store.Admit(context.Background(), key, 30*time.Second)
	if err != nil {
		t.Fatalf("second admit: %v", err)
	}
	if second {
		t.Fatal("the same key must not be admitted twice")
	}
}

func TestReplayGuard_NilIsDisabledRatherThanRefusing(t *testing.T) {
	// A gateway wired without a guard must keep serving: an unconfigured replay cache is not a reason
	// to reject verified traffic.
	var g *relayauth.ReplayGuard
	if !g.Admit("bank-a", "sig-1", time.Unix(1_760_000_000, 0)) {
		t.Fatal("a nil guard must admit")
	}
}
