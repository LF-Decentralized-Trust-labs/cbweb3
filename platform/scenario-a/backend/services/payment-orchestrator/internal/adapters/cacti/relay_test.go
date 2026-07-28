// SPDX-License-Identifier: Apache-2.0

package cacti

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// memWatermarkStore is an in-memory ports.RelayWatermarkStore used to prove the
// poller resumes from a persisted cursor and writes its progress back.
type memWatermarkStore struct {
	mu     sync.Mutex
	values map[string]int64
	sets   map[string]int64 // last value written per kind (write count is len-agnostic)
	writes int
}

func newMemWatermarkStore() *memWatermarkStore {
	return &memWatermarkStore{values: map[string]int64{}, sets: map[string]int64{}}
}

func (m *memWatermarkStore) GetWatermark(_ context.Context, kind string) (int64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[kind]
	return v, ok, nil
}

func (m *memWatermarkStore) SetWatermark(_ context.Context, kind string, value int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[kind] = value
	m.sets[kind] = value
	m.writes++
	return nil
}

func (m *memWatermarkStore) lastSet(kind string) (int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.sets[kind]
	return v, ok
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestPollEvents_ResumesFromPersistedWatermark proves the finding R2-H-11 fix:
// on (re)start the poller must query the relay using the persisted cursor+1,
// not time.Now(), so events observed during downtime are not skipped. It must
// also persist the newest observed timestamp so a later restart resumes again.
func TestPollEvents_ResumesFromPersistedWatermark(t *testing.T) {
	const kind = "settle"
	// Watermark persisted before the (simulated) restart. An event at
	// eventTs > persisted arrived while the poller was down.
	const persisted int64 = 1_000
	const eventTs int64 = 5_000

	var (
		mu        sync.Mutex
		sinceSeen []int64 // every `since` query param the server received
	)
	firstDelivered := make(chan struct{})
	var once sync.Once

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		mu.Lock()
		sinceSeen = append(sinceSeen, since)
		mu.Unlock()

		// Deliver the event only when the poller asks for a window that includes it.
		if since <= eventTs {
			once.Do(func() { close(firstDelivered) })
			_, _ = io.WriteString(w, fmt.Sprintf(
				`[{"spoke":"spoke-a","contractId":"c1","secret":"s1","blockNumber":42,"txHash":"0xabc","timestamp":%d}]`,
				eventTs,
			))
			return
		}
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	store := newMemWatermarkStore()
	store.values[kind] = persisted // simulate state left by the previous process

	relay := NewCactiRelay(srv.URL, "test-secret", store, testLogger())
	relay.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received []ports.InteroperabilityProof
	var rmu sync.Mutex
	handled := make(chan struct{}, 1)
	go relay.pollEvents(ctx, kind, func(p ports.InteroperabilityProof) error {
		rmu.Lock()
		received = append(received, p)
		rmu.Unlock()
		select {
		case handled <- struct{}{}:
		default:
		}
		return nil
	})

	// Wait for the event to be delivered and handled.
	select {
	case <-firstDelivered:
	case <-time.After(2 * time.Second):
		t.Fatal("relay never queried a window covering the downtime event")
	}
	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was never invoked for the downtime event")
	}
	// Give the loop a moment to persist the watermark after handling.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// 1) The very first request must resume from the persisted cursor (+1),
	//    NOT from a fresh time.Now() (which would be far larger than persisted+1
	//    and would have skipped the event).
	mu.Lock()
	if len(sinceSeen) == 0 {
		mu.Unlock()
		t.Fatal("relay made no requests")
	}
	first := sinceSeen[0]
	mu.Unlock()
	if first != persisted+1 {
		t.Fatalf("first poll used since=%d, want %d (persisted watermark + 1)", first, persisted+1)
	}

	// 2) The downtime event must have been delivered.
	rmu.Lock()
	got := len(received)
	rmu.Unlock()
	if got == 0 {
		t.Fatal("downtime event was lost — handler never received it")
	}

	// 3) The newest observed timestamp must be persisted for the next restart.
	last, ok := store.lastSet(kind)
	if !ok {
		t.Fatal("watermark was never persisted")
	}
	if last != eventTs {
		t.Fatalf("persisted watermark = %d, want %d (newest event timestamp)", last, eventTs)
	}
}

// TestPollEvents_HaltsWatermarkOnHandlerFailure proves the advance-after-delivery fix
// (finding R2-H-11): when the handler fails on an event, the poller must NOT advance the
// watermark past it. The earlier successfully-handled event is persisted; the failing event
// is re-fetched every tick until it succeeds, never skipped.
func TestPollEvents_HaltsWatermarkOnHandlerFailure(t *testing.T) {
	const kind = "settle"
	const persisted int64 = 1_000
	const okTs int64 = 2_000  // c1 — handler succeeds
	const badTs int64 = 3_000 // c2 — handler fails

	// Server returns only events with timestamp >= since (mirrors the real relay filter).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		var out []string
		if okTs >= since {
			out = append(out, fmt.Sprintf(`{"spoke":"spoke-a","contractId":"c1","secret":"s1","blockNumber":1,"txHash":"0x1","timestamp":%d}`, okTs))
		}
		if badTs >= since {
			out = append(out, fmt.Sprintf(`{"spoke":"spoke-a","contractId":"c2","secret":"s2","blockNumber":2,"txHash":"0x2","timestamp":%d}`, badTs))
		}
		_, _ = io.WriteString(w, "["+joinCSV(out)+"]")
	}))
	defer srv.Close()

	store := newMemWatermarkStore()
	store.values[kind] = persisted

	relay := NewCactiRelay(srv.URL, "test-secret", store, testLogger())
	relay.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	badCalls := 0
	go relay.pollEvents(ctx, kind, func(p ports.InteroperabilityProof) error {
		if p.ContractID == "c2" {
			mu.Lock()
			badCalls++
			mu.Unlock()
			return fmt.Errorf("simulated settle failure")
		}
		return nil
	})

	// Let several poll cycles run so the failing event is retried.
	time.Sleep(150 * time.Millisecond)
	cancel()

	// The watermark must sit at the last GOOD event (okTs), never past the failed one.
	last, ok := store.lastSet(kind)
	if !ok {
		t.Fatal("watermark was never persisted for the successfully-handled event")
	}
	if last != okTs {
		t.Fatalf("persisted watermark = %d, want %d (must not advance past the failed event)", last, okTs)
	}

	// The failing event must have been retried, not skipped after the first failure.
	mu.Lock()
	got := badCalls
	mu.Unlock()
	if got < 2 {
		t.Fatalf("failing event was retried %d time(s), want >= 2 (must keep retrying)", got)
	}
}

func joinCSV(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

// TestPollEvents_NoStoreFallsBackToNow ensures the poller degrades gracefully
// when no watermark store is wired (nil): it must not panic and must start from
// roughly time.Now().
func TestPollEvents_NoStoreFallsBackToNow(t *testing.T) {
	var gotSince int64 = -1
	done := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		once.Do(func() {
			gotSince = s
			close(done)
		})
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	relay := NewCactiRelay(srv.URL, "secret", nil, testLogger())
	relay.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go relay.pollEvents(ctx, "lock", func(ports.InteroperabilityProof) error { return nil })

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("relay never polled")
	}
	cancel()

	nowMs := time.Now().UnixMilli()
	// since = lastSeen+1 ≈ now; allow a generous window.
	if gotSince < nowMs-60_000 || gotSince > nowMs+5_000 {
		t.Fatalf("nil-store poll since=%d not near now=%d", gotSince, nowMs)
	}
}
