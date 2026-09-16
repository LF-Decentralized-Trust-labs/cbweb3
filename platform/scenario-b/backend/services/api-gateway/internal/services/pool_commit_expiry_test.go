// SPDX-License-Identifier: Apache-2.0

package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// --- Stub repository ---
//
// The worker runs in its own goroutine, so this stub is shared state like any other and needs a
// lock. Without one these tests raced: the worker appended to `updated` inside UpdateStatus while
// the test read it, and `go test -race ./internal/services/` failed on develop for that reason
// alone. cancel() does not wait for the worker to stop, so an in-flight append could still be
// running when the assertions read the slice.
//
// It also signals progress, which is what lets the tests stop guessing with time.Sleep. A sleep
// long enough on a quiet laptop is not long enough on a loaded CI runner, and the failure would
// look like a broken worker rather than a slow machine.

type recordedUpdate struct {
	id     string
	status apidomain.CommitStatus
}

type stubExpiryRepo struct {
	mu      sync.Mutex
	pending []apidomain.PoolCommit
	updated []recordedUpdate
	// want is how many updates a waiter is waiting for; reached closes when that many exist.
	want    int
	reached chan struct{}
	// swept receives once per sweep, so a test can wait for the loop to have looked at all.
	swept chan struct{}
}

func newStubExpiryRepo(pending ...apidomain.PoolCommit) *stubExpiryRepo {
	return &stubExpiryRepo{pending: pending, swept: make(chan struct{}, 64)}
}

func (r *stubExpiryRepo) FindExpiredPending(_ context.Context) ([]apidomain.PoolCommit, error) {
	r.mu.Lock()
	out := append([]apidomain.PoolCommit(nil), r.pending...)
	r.mu.Unlock()
	// Non-blocking: a sweep must never stall because nobody is listening.
	select {
	case r.swept <- struct{}{}:
	default:
	}
	return out, nil
}

func (r *stubExpiryRepo) UpdateStatus(_ context.Context, commitID string, status apidomain.CommitStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updated = append(r.updated, recordedUpdate{commitID, status})
	if r.reached != nil && r.want > 0 && len(r.updated) >= r.want {
		close(r.reached)
		r.reached = nil
	}
	return nil
}

// snapshot copies what has been recorded so far. Assertions read this, never the slice itself.
func (r *stubExpiryRepo) snapshot() []recordedUpdate {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedUpdate(nil), r.updated...)
}

// expect returns a channel closed once n updates have been recorded. Waiting on a condition rather
// than on a duration is what removes the timing assumption: the test can only fail if the work never
// happens, not because a machine was busy.
func (r *stubExpiryRepo) expect(n int) <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan struct{})
	r.want, r.reached = n, ch
	if len(r.updated) >= n {
		close(ch)
		r.reached = nil
	}
	return ch
}

// runWorker starts the worker and returns a stop function that cancels it AND waits for Run to
// return. Reading the stub before the worker has stopped is the other half of the race.
func runWorker(t *testing.T, w *services.PoolCommitExpiryWorker) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("worker did not stop within 2s of cancellation")
		}
	}
}

// waitFor blocks until ch fires, failing the test on a generous deadline.
func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out after 2s waiting for %s", what)
	}
}

// --- Tests ---

func TestExpiry_marksExpiredCommits(t *testing.T) {
	now := time.Now()
	repo := newStubExpiryRepo(
		apidomain.PoolCommit{CommitID: "abc", Status: apidomain.CommitStatusPending, ExpiresAt: now.Add(-1 * time.Minute)},
		apidomain.PoolCommit{CommitID: "def", Status: apidomain.CommitStatusPending, ExpiresAt: now.Add(-5 * time.Minute)},
	)
	bothExpired := repo.expect(2)

	stop := runWorker(t, services.NewPoolCommitExpiryWorker(repo, 10*time.Millisecond))
	waitFor(t, bothExpired, "both commits to be expired")
	stop()

	updated := repo.snapshot()
	if len(updated) < 2 {
		t.Fatalf("expected 2 updates, got %d", len(updated))
	}
	for _, u := range updated {
		if u.status != apidomain.CommitStatusExpired {
			t.Errorf("expected EXPIRED, got %s for commit %s", u.status, u.id)
		}
	}
}

func TestExpiry_doesNotExpireMatched(t *testing.T) {
	// FindExpiredPending returns only PENDING commits — stub returns empty for this test.
	repo := newStubExpiryRepo()

	stop := runWorker(t, services.NewPoolCommitExpiryWorker(repo, 10*time.Millisecond))
	// Nothing will be updated, so there is no update to wait for. Waiting for a sweep to have
	// HAPPENED is what makes the assertion mean something: without it, zero updates could simply
	// mean the loop had not ticked yet.
	waitFor(t, repo.swept, "the first sweep")
	stop()

	if updated := repo.snapshot(); len(updated) != 0 {
		t.Errorf("expected 0 updates for empty pending set, got %d", len(updated))
	}
}

func TestExpiry_gracefulShutdown(t *testing.T) {
	repo := newStubExpiryRepo()
	worker := services.NewPoolCommitExpiryWorker(repo, 1*time.Hour) // long interval

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	cancel() // cancel immediately

	select {
	case <-done:
		// OK — worker exited
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not shut down within 1 second")
	}
}
