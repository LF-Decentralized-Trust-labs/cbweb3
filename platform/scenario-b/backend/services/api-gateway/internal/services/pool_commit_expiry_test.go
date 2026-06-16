// SPDX-License-Identifier: Apache-2.0

package services_test

import (
	"context"
	"testing"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// --- Stub repository ---

type stubExpiryRepo struct {
	pending []apidomain.PoolCommit
	updated []struct {
		id     string
		status apidomain.CommitStatus
	}
}

func (r *stubExpiryRepo) FindExpiredPending(_ context.Context) ([]apidomain.PoolCommit, error) {
	return r.pending, nil
}

func (r *stubExpiryRepo) UpdateStatus(_ context.Context, commitID string, status apidomain.CommitStatus) error {
	r.updated = append(r.updated, struct {
		id     string
		status apidomain.CommitStatus
	}{commitID, status})
	return nil
}

// --- Tests ---

func TestExpiry_marksExpiredCommits(t *testing.T) {
	now := time.Now()
	repo := &stubExpiryRepo{
		pending: []apidomain.PoolCommit{
			{CommitID: "abc", Status: apidomain.CommitStatusPending, ExpiresAt: now.Add(-1 * time.Minute)},
			{CommitID: "def", Status: apidomain.CommitStatusPending, ExpiresAt: now.Add(-5 * time.Minute)},
		},
	}

	worker := services.NewPoolCommitExpiryWorker(repo, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	go worker.Run(ctx)

	// Wait for at least one tick
	time.Sleep(30 * time.Millisecond)
	cancel()

	if len(repo.updated) < 2 {
		t.Fatalf("expected 2 updates, got %d", len(repo.updated))
	}
	for _, u := range repo.updated {
		if u.status != apidomain.CommitStatusExpired {
			t.Errorf("expected EXPIRED, got %s for commit %s", u.status, u.id)
		}
	}
}

func TestExpiry_doesNotExpireMatched(t *testing.T) {
	// FindExpiredPending returns only PENDING commits — stub returns empty for this test
	repo := &stubExpiryRepo{pending: nil}

	worker := services.NewPoolCommitExpiryWorker(repo, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	go worker.Run(ctx)

	time.Sleep(30 * time.Millisecond)
	cancel()

	if len(repo.updated) != 0 {
		t.Errorf("expected 0 updates for empty pending set, got %d", len(repo.updated))
	}
}

func TestExpiry_gracefulShutdown(t *testing.T) {
	repo := &stubExpiryRepo{pending: nil}
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
