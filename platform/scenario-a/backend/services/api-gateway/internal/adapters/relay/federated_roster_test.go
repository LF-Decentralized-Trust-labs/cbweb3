// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type fakeLocal struct {
	ids []string
	err error
}

func (f *fakeLocal) ListParticipantIdentities(context.Context) ([]string, error) {
	return f.ids, f.err
}

type fakeSpokes struct {
	spokes []Spoke
	err    error
	calls  int
}

func (f *fakeSpokes) ListSpokes(context.Context) ([]Spoke, error) {
	f.calls++
	return f.spokes, f.err
}

type fakePeers struct {
	// byURL maps a peer base URL to the identities it returns.
	byURL map[string][]string
	errs  map[string]error
	mu    sync.Mutex
	calls []string
}

func (f *fakePeers) FetchPeerIdentities(_ context.Context, baseURL string) ([]string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, baseURL)
	f.mu.Unlock()
	if err := f.errs[baseURL]; err != nil {
		return nil, err
	}
	return f.byURL[baseURL], nil
}

func TestFederatedRoster_UnionsLocalAndAllCBGateways(t *testing.T) {
	// A commercial bank node: its local Pente membership is only the bank↔CB group,
	// so it CANNOT see its sibling bank (bank-macro) locally. The federation must
	// query its own spoke's CB gateway too — not skip it — to discover siblings.
	local := &fakeLocal{ids: []string{"funded_operator@spoke-ars-bank-galicia", "funded_operator@spoke-ars-cb"}}
	spokes := &fakeSpokes{spokes: []Spoke{
		{ID: "spoke-ars", InternalApiURL: "http://ars-cb:18845"}, // own spoke's CB → still queried (sibling discovery)
		{ID: "spoke-cop", InternalApiURL: "http://cop-cb:18745"},
	}}
	peers := &fakePeers{byURL: map[string][]string{
		"http://ars-cb:18845": {"funded_operator@spoke-ars-cb", "funded_operator@spoke-ars-bank-galicia", "funded_operator@spoke-ars-bank-macro"},
		"http://cop-cb:18745": {"funded_operator@spoke-cop-cb", "funded_operator@spoke-cop-bank-bancolombia"},
	}}
	r := NewFederatedRoster(local, spokes, peers, time.Minute)

	got, err := r.ListParticipantIdentities(context.Background())
	if err != nil {
		t.Fatalf("ListParticipantIdentities: %v", err)
	}
	want := []string{
		"funded_operator@spoke-ars-bank-galicia",
		"funded_operator@spoke-ars-bank-macro", // discovered via own CB gateway, not local
		"funded_operator@spoke-ars-cb",
		"funded_operator@spoke-cop-bank-bancolombia",
		"funded_operator@spoke-cop-cb",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roster = %v\nwant %v", got, want)
	}
}

func TestFederatedRoster_OnePeerDownDoesNotBlankRoster(t *testing.T) {
	local := &fakeLocal{ids: []string{"funded_operator@spoke-brl-cb"}}
	spokes := &fakeSpokes{spokes: []Spoke{
		{ID: "spoke-cop", InternalApiURL: "http://cop:18745"},
		{ID: "spoke-ars", InternalApiURL: "http://ars:18845"},
	}}
	peers := &fakePeers{
		byURL: map[string][]string{"http://ars:18845": {"funded_operator@spoke-ars-cb"}},
		errs:  map[string]error{"http://cop:18745": errors.New("connection refused")},
	}
	r := NewFederatedRoster(local, spokes, peers, time.Minute)

	got, err := r.ListParticipantIdentities(context.Background())
	if err != nil {
		t.Fatalf("ListParticipantIdentities: %v", err)
	}
	// The reachable peer + local survive; the unreachable one is simply absent.
	want := []string{"funded_operator@spoke-ars-cb", "funded_operator@spoke-brl-cb"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roster = %v, want %v", got, want)
	}
}

func TestFederatedRoster_RelayDownFallsBackToLocal(t *testing.T) {
	local := &fakeLocal{ids: []string{"funded_operator@spoke-brl-cb"}}
	spokes := &fakeSpokes{err: errors.New("relay unreachable")}
	peers := &fakePeers{}
	r := NewFederatedRoster(local, spokes, peers, time.Minute)

	got, err := r.ListParticipantIdentities(context.Background())
	if err != nil {
		t.Fatalf("expected best-effort local fallback, got error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"funded_operator@spoke-brl-cb"}) {
		t.Errorf("roster = %v, want local-only fallback", got)
	}
}

func TestFederatedRoster_LocalAndRelayDownReturnsError(t *testing.T) {
	local := &fakeLocal{err: errors.New("orchestrator down")}
	spokes := &fakeSpokes{err: errors.New("relay unreachable")}
	r := NewFederatedRoster(local, spokes, &fakePeers{}, time.Minute)

	if _, err := r.ListParticipantIdentities(context.Background()); err == nil {
		t.Error("expected an error when both local and relay are down (fail closed)")
	}
}

func TestFederatedRoster_CachesWithinTTL(t *testing.T) {
	local := &fakeLocal{ids: []string{"funded_operator@spoke-brl-cb"}}
	spokes := &fakeSpokes{spokes: []Spoke{{ID: "spoke-cop", InternalApiURL: "http://cop:18745"}}}
	peers := &fakePeers{byURL: map[string][]string{"http://cop:18745": {"funded_operator@spoke-cop-cb"}}}
	r := NewFederatedRoster(local, spokes, peers, time.Minute)

	if _, err := r.ListParticipantIdentities(context.Background()); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := r.ListParticipantIdentities(context.Background()); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if spokes.calls != 1 {
		t.Errorf("relay hit %d times within TTL, want 1 (cached)", spokes.calls)
	}
}
