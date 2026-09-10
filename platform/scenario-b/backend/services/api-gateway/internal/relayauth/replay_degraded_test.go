// SPDX-License-Identifier: Apache-2.0

package relayauth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
)

// Admit used to answer a single bool, which collapsed two very different situations into "yes":
//
//   - the shared store agreed this signature is new;
//   - the shared store could not be reached, so only this process's memory was consulted.
//
// The second is a best-effort answer, and on POST /internal/v2/transfer-limits/restore — a
// compliance control that credits a bank's daily allowance back — best-effort is not enough. An
// actor who induces a few minutes of store unavailability can replay a captured restore inside the
// ±5-minute signature skew and let a bank transact past the limit its central bank configured.
//
// Failing closed everywhere was rejected: it would stop bridge-in, the delegated hub swap and the
// residue return, a far larger outage than the window it closes. So the caller has to be able to
// tell the two apart and decide per route. That is all this type does; the routing decision lives
// in the middleware.

type flakyStore struct {
	err      error
	admitted bool
	calls    int
}

func (s *flakyStore) Admit(context.Context, string, time.Duration) (bool, error) {
	s.calls++
	return s.admitted, s.err
}

func TestAdmit_SharedStoreAgrees(t *testing.T) {
	g := relayauth.NewReplayGuard(time.Minute).WithShared(&flakyStore{admitted: true})
	if got := g.Admit("k", "sig", time.Now()); got != relayauth.AdmissionAdmitted {
		t.Errorf("got %v, want relayauth.AdmissionAdmitted", got)
	}
}

func TestAdmit_SharedStoreSaysSeen(t *testing.T) {
	g := relayauth.NewReplayGuard(time.Minute).WithShared(&flakyStore{admitted: false})
	if got := g.Admit("k", "sig", time.Now()); got != relayauth.AdmissionRefused {
		t.Errorf("got %v, want relayauth.AdmissionRefused", got)
	}
}

// The case the whole card is about.
func TestAdmit_SharedStoreUnreachableIsDegradedNotAdmitted(t *testing.T) {
	g := relayauth.NewReplayGuard(time.Minute).WithShared(&flakyStore{err: errors.New("dial tcp: refused")})

	got := g.Admit("k", "sig", time.Now())
	if got == relayauth.AdmissionAdmitted {
		t.Fatal("an unreachable shared store reported a full admission; the caller cannot then tell " +
			"a checked request from an unchecked one, which is what made the fail-open invisible")
	}
	if got != relayauth.AdmissionDegraded {
		t.Fatalf("got %v, want relayauth.AdmissionDegraded", got)
	}
}

// The local memory still decides first, and it is authoritative when it says NO: a replay this
// process has already seen is a replay whatever the store thinks.
func TestAdmit_LocalRefusalNeedsNoStore(t *testing.T) {
	store := &flakyStore{admitted: true}
	g := relayauth.NewReplayGuard(time.Minute).WithShared(store)
	now := time.Now()

	if got := g.Admit("k", "sig", now); got != relayauth.AdmissionAdmitted {
		t.Fatalf("first use: got %v", got)
	}
	if got := g.Admit("k", "sig", now); got != relayauth.AdmissionRefused {
		t.Errorf("second use: got %v, want relayauth.AdmissionRefused", got)
	}
	if store.calls != 1 {
		t.Errorf("the shared store was consulted %d times; a local refusal is decided without a "+
			"round-trip, so no store hiccup can turn a known replay into an admission", store.calls)
	}
}

// A deployment with no shared store configured is NOT degraded. It is the single-process
// deployment this guard was born in, and its answer is as authoritative as that deployment gets.
// Reporting it as degraded would make the fail-closed route below permanently unusable wherever
// Redis was never wired — turning a hardening measure into an outage.
func TestAdmit_NoSharedStoreIsAdmittedNotDegraded(t *testing.T) {
	g := relayauth.NewReplayGuard(time.Minute)
	if got := g.Admit("k", "sig", time.Now()); got != relayauth.AdmissionAdmitted {
		t.Errorf("got %v, want relayauth.AdmissionAdmitted — no shared store is a deployment choice, not a fault", got)
	}
}

// A nil guard admits, as before: a gateway wired without one is in the pre-existing state.
func TestAdmit_NilGuardAdmits(t *testing.T) {
	var g *relayauth.ReplayGuard
	if got := g.Admit("k", "sig", time.Now()); got != relayauth.AdmissionAdmitted {
		t.Errorf("got %v, want relayauth.AdmissionAdmitted", got)
	}
}

// relayauth.Admission values are compared in the middleware; a zero value that meant "admitted" would make
// a forgotten assignment fail open, which is the exact defect being fixed.
func TestAdmission_ZeroValueIsNotAnAdmission(t *testing.T) {
	var zero relayauth.Admission
	if zero == relayauth.AdmissionAdmitted {
		t.Error("the zero relayauth.Admission is relayauth.AdmissionAdmitted; an uninitialised value must never mean yes")
	}
}
