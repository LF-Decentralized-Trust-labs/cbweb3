// SPDX-License-Identifier: Apache-2.0

// Package app provides the PoolCommitRepository for cooperative liquidity commit-reveal (FR-001 / data-model.md §1.1).
package app

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// PoolCommitRepository handles persistence for PoolCommit records.
// All writes are append-only: EXECUTED and EXPIRED commits are never updated.
type PoolCommitRepository struct {
	db *gorm.DB
}

// NewPoolCommitRepository creates a PoolCommitRepository backed by the given DB.
func NewPoolCommitRepository(db *gorm.DB) *PoolCommitRepository {
	return &PoolCommitRepository{db: db}
}

// Create inserts a new PoolCommit. Returns an error if the unique partial index
// (pool_pair, side) WHERE status IN ('PENDING','MATCHED') would be violated.
func (r *PoolCommitRepository) Create(ctx context.Context, commit *apidomain.PoolCommit) error {
	return r.db.WithContext(ctx).Create(commit).Error
}

// FindActiveByPairAndSide returns the single PENDING or MATCHED commit for a given
// (pool_pair, side) combination, or gorm.ErrRecordNotFound if none exists.
func (r *PoolCommitRepository) FindActiveByPairAndSide(ctx context.Context, poolPair string, side apidomain.CommitSide) (*apidomain.PoolCommit, error) {
	var commit apidomain.PoolCommit
	err := r.db.WithContext(ctx).
		Where("pool_pair = ? AND side = ? AND status IN ('PENDING','MATCHED')", poolPair, side).
		First(&commit).Error
	if err != nil {
		return nil, err
	}
	return &commit, nil
}

// FindByProviderAndPair returns the PENDING or MATCHED commit for a given provider
// and pool_pair (any side), to enforce the SAME_PROVIDER_BOTH_SIDES check.
func (r *PoolCommitRepository) FindByProviderAndPair(ctx context.Context, providerID, poolPair string) (*apidomain.PoolCommit, error) {
	var commit apidomain.PoolCommit
	err := r.db.WithContext(ctx).
		Where("provider_id = ? AND pool_pair = ? AND status IN ('PENDING','MATCHED')", providerID, poolPair).
		First(&commit).Error
	if err != nil {
		return nil, err
	}
	return &commit, nil
}

// UpdateStatus transitions a commit to a new status. Terminal statuses (EXECUTED, EXPIRED)
// should only be written once; callers are responsible for ensuring valid transitions.
func (r *PoolCommitRepository) UpdateStatus(ctx context.Context, commitID string, status apidomain.CommitStatus) error {
	return r.db.WithContext(ctx).
		Model(&apidomain.PoolCommit{}).
		Where("commit_id = ?", commitID).
		Update("status", status).Error
}

// FindExpiredPending returns all PENDING commits whose expires_at is in the past.
// Used by PoolCommitExpiryWorker (D6 / FR-013).
func (r *PoolCommitRepository) FindExpiredPending(ctx context.Context) ([]apidomain.PoolCommit, error) {
	var commits []apidomain.PoolCommit
	err := r.db.WithContext(ctx).
		Where("status = 'PENDING' AND expires_at < ?", time.Now().UTC()).
		Find(&commits).Error
	return commits, err
}

// FindByID returns a PoolCommit by its primary key.
func (r *PoolCommitRepository) FindByID(ctx context.Context, commitID string) (*apidomain.PoolCommit, error) {
	var commit apidomain.PoolCommit
	err := r.db.WithContext(ctx).Where("commit_id = ?", commitID).First(&commit).Error
	if err != nil {
		return nil, err
	}
	return &commit, nil
}

// ListByPair returns all commits for a given pool_pair, optionally filtered by status.
// Pass empty string for status to return all statuses.
func (r *PoolCommitRepository) ListByPair(ctx context.Context, poolPair, status string) ([]apidomain.PoolCommit, error) {
	q := r.db.WithContext(ctx).Where("pool_pair = ?", poolPair)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var commits []apidomain.PoolCommit
	err := q.Order("created_at DESC").Find(&commits).Error
	return commits, err
}

// decodeOnChainCommitID converts a 0x-prefixed hex string (with or without leading zeros)
// to a 32-byte slice suitable for querying the on_chain_commit_id bytea column.
func decodeOnChainCommitID(onChainHex string) ([]byte, error) {
	s := strings.TrimPrefix(onChainHex, "0x")
	if len(s)%2 != 0 {
		s = "0" + s // pad leading zero if odd length (ethers may strip it)
	}
	return hex.DecodeString(s)
}

// UpdateStatusByOnChainCommitID transitions a commit identified by its on-chain bytes32 commitId hex.
func (r *PoolCommitRepository) UpdateStatusByOnChainCommitID(ctx context.Context, onChainCommitIDHex string, status apidomain.CommitStatus) error {
	b, err := decodeOnChainCommitID(onChainCommitIDHex)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&apidomain.PoolCommit{}).
		Where("on_chain_commit_id = ?", b).
		Update("status", status).Error
}

// FindByOnChainCommitID returns the commit associated with the given on-chain bytes32 commitId hex.
func (r *PoolCommitRepository) FindByOnChainCommitID(ctx context.Context, onChainCommitIDHex string) (*apidomain.PoolCommit, error) {
	b, err := decodeOnChainCommitID(onChainCommitIDHex)
	if err != nil {
		return nil, err
	}
	var commit apidomain.PoolCommit
	err = r.db.WithContext(ctx).Where("on_chain_commit_id = ?", b).First(&commit).Error
	if err != nil {
		return nil, err
	}
	return &commit, nil
}
