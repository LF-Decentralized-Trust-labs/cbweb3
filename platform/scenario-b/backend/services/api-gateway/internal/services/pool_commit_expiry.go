// SPDX-License-Identifier: Apache-2.0

// Package services provides the PoolCommitExpiryWorker goroutine (FR-013 / D6).
package services

import (
	"context"
	"log"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// PoolCommitExpiryRepo is the minimum repository interface needed by the expiry worker.
type PoolCommitExpiryRepo interface {
	FindExpiredPending(ctx context.Context) ([]apidomain.PoolCommit, error)
	UpdateStatus(ctx context.Context, commitID string, status apidomain.CommitStatus) error
}

// PoolCommitExpiryWorker scans for PENDING commits that have passed their expires_at
// and transitions them to EXPIRED. Runs at a configurable interval (default 5 minutes).
// Shutdown is signaled by cancelling the provided context.
type PoolCommitExpiryWorker struct {
	repo     PoolCommitExpiryRepo
	interval time.Duration
}

// NewPoolCommitExpiryWorker creates a worker with the given repository and poll interval.
// Pass interval=0 to use the default of 5 minutes.
func NewPoolCommitExpiryWorker(repo PoolCommitExpiryRepo, interval time.Duration) *PoolCommitExpiryWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &PoolCommitExpiryWorker{repo: repo, interval: interval}
}

// Run starts the expiry loop. Blocks until ctx is cancelled.
// Intended to be launched as a goroutine: go worker.Run(ctx).
func (w *PoolCommitExpiryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("pool_commit_expiry: worker started (interval=%s)", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("pool_commit_expiry: worker stopping (context cancelled)")
			return
		case <-ticker.C:
			w.expire(ctx)
		}
	}
}

// expire finds and marks all expired PENDING commits in a single sweep.
func (w *PoolCommitExpiryWorker) expire(ctx context.Context) {
	commits, err := w.repo.FindExpiredPending(ctx)
	if err != nil {
		log.Printf("pool_commit_expiry: find expired: %v", err)
		return
	}
	for _, c := range commits {
		if err := w.repo.UpdateStatus(ctx, c.CommitID, apidomain.CommitStatusExpired); err != nil {
			log.Printf("pool_commit_expiry: expire commit %s: %v", c.CommitID, err)
		} else {
			log.Printf("pool_commit_expiry: expired commit %s (pool_pair=%s side=%s)", c.CommitID, c.PoolPair, c.Side)
		}
	}
}
