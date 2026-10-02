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
	"strings"
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

// TestPollEvents_ResumesFromPersistedSeq proves the finding R2-H-11 fix:
// on (re)start the poller must query the relay using the persisted journal seq (not time.Now()),
// so events observed during downtime are not skipped, and must persist the newest seq it handled
// so a later restart resumes again. The cursor is a durable seq — the relay filters seq > since,
// so the poller passes the cursor as-is (no +1).
func TestPollEvents_ResumesFromPersistedSeq(t *testing.T) {
	const kind = "settle"
	// Seq persisted before the (simulated) restart. An event with a higher seq was journaled
	// while the poller was down.
	const persisted int64 = 1_000
	const eventSeq int64 = 5_000

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

		// The relay serves events with seq strictly greater than `since`.
		if since < eventSeq {
			once.Do(func() { close(firstDelivered) })
			_, _ = io.WriteString(w, fmt.Sprintf(
				`[{"spoke":"spoke-a","contractId":"c1","secret":"s1","blockNumber":42,"txHash":"0xabc","seq":%d}]`,
				eventSeq,
			))
			return
		}
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	store := newMemWatermarkStore()
	store.values[watermarkStoreKey(kind)] = persisted // simulate state left by the previous process

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
	// Give the loop a moment to persist the cursor after handling.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// 1) The very first request must resume from the persisted seq exactly (no +1, no time.Now()).
	mu.Lock()
	if len(sinceSeen) == 0 {
		mu.Unlock()
		t.Fatal("relay made no requests")
	}
	first := sinceSeen[0]
	mu.Unlock()
	if first != persisted {
		t.Fatalf("first poll used since=%d, want %d (persisted seq)", first, persisted)
	}

	// 2) The downtime event must have been delivered.
	rmu.Lock()
	got := len(received)
	rmu.Unlock()
	if got == 0 {
		t.Fatal("downtime event was lost — handler never received it")
	}

	// 3) The newest handled seq must be persisted (under the versioned key) for the next restart.
	last, ok := store.lastSet(watermarkStoreKey(kind))
	if !ok {
		t.Fatal("cursor was never persisted")
	}
	if last != eventSeq {
		t.Fatalf("persisted seq = %d, want %d (newest event seq)", last, eventSeq)
	}
}

// TestPollEvents_HaltsWatermarkOnHandlerFailure proves the advance-after-delivery fix
// (finding R2-H-11): when the handler fails on an event, the poller must NOT advance the
// watermark past it. The earlier successfully-handled event is persisted; the failing event
// is re-fetched every tick until it succeeds, never skipped.
func TestPollEvents_HaltsWatermarkOnHandlerFailure(t *testing.T) {
	const kind = "settle"
	const persisted int64 = 1_000
	const okSeq int64 = 2_000  // c1 — handler succeeds
	const badSeq int64 = 3_000 // c2 — handler fails

	// Server returns events with seq strictly greater than since (mirrors the real relay filter).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		var out []string
		if okSeq > since {
			out = append(out, fmt.Sprintf(`{"spoke":"spoke-a","contractId":"c1","secret":"s1","blockNumber":1,"txHash":"0x1","seq":%d}`, okSeq))
		}
		if badSeq > since {
			out = append(out, fmt.Sprintf(`{"spoke":"spoke-a","contractId":"c2","secret":"s2","blockNumber":2,"txHash":"0x2","seq":%d}`, badSeq))
		}
		_, _ = io.WriteString(w, "["+joinCSV(out)+"]")
	}))
	defer srv.Close()

	store := newMemWatermarkStore()
	store.values[watermarkStoreKey(kind)] = persisted

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

	// The cursor must sit at the last GOOD event (okSeq), never past the failed one.
	last, ok := store.lastSet(watermarkStoreKey(kind))
	if !ok {
		t.Fatal("cursor was never persisted for the successfully-handled event")
	}
	if last != okSeq {
		t.Fatalf("persisted seq = %d, want %d (must not advance past the failed event)", last, okSeq)
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

// TestPollEvents_NoStoreStartsAtJournalHead ensures the poller degrades gracefully when no
// watermark store is wired (nil): it must not panic and must start from seq 0 (the journal head).
// Unlike the old ms cursor there is no time.Now() fallback — the durable, bounded journal makes
// seq 0 a safe "give me what you still hold" resume.
func TestPollEvents_NoStoreStartsAtJournalHead(t *testing.T) {
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

	if gotSince != 0 {
		t.Fatalf("nil-store poll since=%d, want 0 (journal head)", gotSince)
	}
}

// ── journal retention: finding out that the cursor fell off the end ──────────────
//
// The relay's journal is capped. A consumer that lags past the cap receives the entries that
// survived and advances over the ones that did not, silently. For the settle journal that is a
// lost settlement, and it cannot be recovered from anywhere else: an HTLC leg is settled by
// transferLocked on its owner's own Paladin node, so the owner is the only party that can do
// it, and the journal is how it finds out that it must. Until now only the relay knew, in a
// warning nobody reads — the orchestrator that actually loses learned nothing.
//
// The relay serves the highest seq it has dropped. A persisted cursor at or below that means
// events are gone.

// logCapture collects records so a test can assert what an operator would see.
type logCapture struct {
	mu      sync.Mutex
	records []string
}

func (c *logCapture) Enabled(context.Context, slog.Level) bool { return true }
func (c *logCapture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := r.Level.String() + " " + r.Message
	r.Attrs(func(a slog.Attr) bool {
		msg += " " + a.Key + "=" + a.Value.String()
		return true
	})
	c.records = append(c.records, msg)
	return nil
}
func (c *logCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *logCapture) WithGroup(string) slog.Handler      { return c }
func (c *logCapture) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.records...)
}

func containsAll(records []string, level string, needles ...string) bool {
	for _, r := range records {
		if !strings.HasPrefix(r, level) {
			continue
		}
		ok := true
		for _, n := range needles {
			if !strings.Contains(r, n) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// A cursor behind the relay's trim mark means settlements were missed. It must be reported at
// ERROR, naming the range, because nothing downstream will notice on its own.
func TestPollEvents_ReportsEventsLostToJournalRetention(t *testing.T) {
	polls := make(chan int64, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		select {
		case polls <- s:
		default:
		}
		w.Header().Set(journalTrimmedHeader, "500") // the relay dropped everything through seq 500
		_, _ = io.WriteString(w, `[{"spoke":"spoke-a","contractId":"c1","secret":"ab","seq":501}]`)
	}))
	defer srv.Close()

	store := newMemWatermarkStore()
	store.values[watermarkStoreKey("settle")] = 100 // we are 400 seqs behind the trim

	cap := &logCapture{}
	relay := NewCactiRelay(srv.URL, "secret", store, slog.New(cap))
	relay.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go relay.pollEvents(ctx, "settle", func(ports.InteroperabilityProof) error { return nil })

	deadline := time.After(3 * time.Second)
	for {
		if containsAll(cap.all(), "ERROR", "101", "500") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("no ERROR naming the lost seq range; got: %v", cap.all())
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
}

// Losing events must not also stop the ones that survived: they are the settlements this
// entity can still make, and stalling would turn a data loss into an outage — the mistake
// this whole line of work exists to undo. Report, then carry on past the gap.
func TestPollEvents_ContinuesPastTheGapInsteadOfStalling(t *testing.T) {
	var handled int64
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(journalTrimmedHeader, "500")
		s, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		if s >= 501 {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		_, _ = io.WriteString(w, `[{"spoke":"spoke-a","contractId":"c1","secret":"ab","seq":501}]`)
	}))
	defer srv.Close()

	store := newMemWatermarkStore()
	store.values[watermarkStoreKey("settle")] = 100

	relay := NewCactiRelay(srv.URL, "secret", store, testLogger())
	relay.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go relay.pollEvents(ctx, "settle", func(ports.InteroperabilityProof) error {
		mu.Lock()
		handled++
		mu.Unlock()
		return nil
	})

	deadline := time.After(3 * time.Second)
	for {
		if v, ok := store.lastSet(watermarkStoreKey("settle")); ok && v >= 501 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the poller stalled at the gap instead of processing what survived")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if handled == 0 {
		t.Error("the surviving event was never handled")
	}
}

// A consumer that has never persisted a cursor — a bank joining a network that has been
// running — legitimately starts at the head and has no claim to events that predate it.
// Reporting those as lost would cry wolf on every new participant.
func TestPollEvents_DoesNotReportLossForAFirstEverStart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(journalTrimmedHeader, "500")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	cap := &logCapture{}
	relay := NewCactiRelay(srv.URL, "secret", newMemWatermarkStore(), slog.New(cap))
	relay.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go relay.pollEvents(ctx, "settle", func(ports.InteroperabilityProof) error { return nil })

	time.Sleep(200 * time.Millisecond)
	cancel()

	for _, r := range cap.all() {
		if strings.HasPrefix(r, "ERROR") && strings.Contains(r, "retention") {
			t.Errorf("a first-ever start must not report lost events; got: %s", r)
		}
	}
}

// A relay that predates the header sends nothing, and an orchestrator must keep working
// against it — they are deployed separately.
func TestPollEvents_ToleratesARelayWithoutTheHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"spoke":"spoke-a","contractId":"c1","secret":"ab","seq":7}]`)
	}))
	defer srv.Close()

	store := newMemWatermarkStore()
	store.values[watermarkStoreKey("settle")] = 1

	relay := NewCactiRelay(srv.URL, "secret", store, testLogger())
	relay.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go relay.pollEvents(ctx, "settle", func(ports.InteroperabilityProof) error { return nil })

	deadline := time.After(2 * time.Second)
	for {
		if v, ok := store.lastSet(watermarkStoreKey("settle")); ok && v == 7 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the poller must keep working against a relay that does not send the header")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
}
