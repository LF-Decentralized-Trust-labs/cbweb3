// Package services provides the pool status service for Scenario B (FR-028 / REQ-FX-008).
// Extended for 005-cooperative-liquidity (T018): pool_status derivation, fee_rate_bps,
// total_lp_count, pending_commits[].
package services

import (
	"context"
	"fmt"
	"time"
)

// ImbalanceThreshold is the 70/30 ratio threshold that triggers a pool imbalance flag.
const ImbalanceThreshold = 0.70

// AMMPoolReader reads reserve data and fee rate from the Hub AMM contract.
type AMMPoolReader interface {
	GetPoolReserves(ctx context.Context, pair string) (reserveA, reserveB string, ratio float64, err error)
	// GetFeeBps returns the current swap fee in basis points (e.g. 30 = 0.30%). T018 / FR-005.
	GetFeeBps(ctx context.Context) (uint64, error)
}

// PoolStatusEnricher provides DB-backed counts for LP positions and pending commits (T018).
// Optional: if nil, total_lp_count defaults to 0 and pending_commits to an empty slice.
type PoolStatusEnricher interface {
	CountActiveLPs(ctx context.Context, poolPair string) (int, error)
	ListPendingCommits(ctx context.Context, poolPair string) ([]PendingCommitSummary, error)
}

// PendingCommitSummary is a lightweight view of a PENDING PoolCommit for pool status display.
type PendingCommitSummary struct {
	CommitID   string    `json:"commit_id"`
	ProviderID string    `json:"provider_id"`
	Side       string    `json:"side"`
	Amount     string    `json:"amount"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// PoolStatusResponse holds the current state of an AMM liquidity pool (T018 / FR-028).
type PoolStatusResponse struct {
	PoolPair       string                 `json:"pool_pair"`
	// PoolStatus is derived from reserves (D8): EMPTY | PENDING_COUNTERPART | ACTIVE.
	PoolStatus     string                 `json:"pool_status"`
	ReserveA       string                 `json:"reserve_a"`
	ReserveB       string                 `json:"reserve_b"`
	CurrentRatio   float64                `json:"current_ratio"`
	ImbalanceFlag  bool                   `json:"imbalance_flag"`
	// FeeRateBps is the current swap fee in basis points (FR-005 / T024).
	FeeRateBps     uint64                 `json:"fee_rate_bps"`
	// TotalLPCount is the number of ACTIVE LP positions in this pool.
	TotalLPCount   int                    `json:"total_lp_count"`
	// PendingCommits lists commits awaiting a counterpart (PENDING status only).
	PendingCommits []PendingCommitSummary `json:"pending_commits"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// PoolStatusService reads live pool reserves from the AMM and computes the imbalance flag.
type PoolStatusService struct {
	reader   AMMPoolReader
	enricher PoolStatusEnricher
}

// NewPoolStatusService creates a PoolStatusService without DB enrichment.
func NewPoolStatusService(reader AMMPoolReader) *PoolStatusService {
	return &PoolStatusService{reader: reader}
}

// WithEnricher attaches a PoolStatusEnricher for LP count and pending commit queries.
// Returns the service to allow method chaining.
func (s *PoolStatusService) WithEnricher(e PoolStatusEnricher) *PoolStatusService {
	s.enricher = e
	return s
}

// GetPoolStatus queries the AMM and DB for the full cooperative pool status (T018).
func (s *PoolStatusService) GetPoolStatus(ctx context.Context, pair string) (*PoolStatusResponse, error) {
	if pair == "" {
		return nil, fmt.Errorf("pair is required")
	}

	reserveA, reserveB, ratio, err := s.reader.GetPoolReserves(ctx, pair)
	if err != nil {
		return nil, fmt.Errorf("amm pool status query failed: %w", err)
	}

	// fee_rate_bps — non-fatal: default to 0 if the call fails (contract may not have setFeeBps yet).
	var feeBps uint64
	if bps, err := s.reader.GetFeeBps(ctx); err == nil {
		feeBps = bps
	}

	// LP count and pending commits — only available when DB enricher is wired.
	lpCount := 0
	pendingCommits := []PendingCommitSummary{}
	if s.enricher != nil {
		if n, err := s.enricher.CountActiveLPs(ctx, pair); err == nil {
			lpCount = n
		}
		if commits, err := s.enricher.ListPendingCommits(ctx, pair); err == nil {
			pendingCommits = commits
		}
	}

	poolStatus := derivePoolStatus(reserveA, reserveB)
	// D15 / FR-001: DB-first override — when reserves are still zero (commit pending on-chain)
	// but a PENDING PoolCommit exists in DB, the pool is awaiting a counterpart, not empty.
	if poolStatus == "EMPTY" && len(pendingCommits) > 0 {
		poolStatus = "PENDING_COUNTERPART"
	}

	return &PoolStatusResponse{
		PoolPair:       pair,
		PoolStatus:     poolStatus,
		ReserveA:       reserveA,
		ReserveB:       reserveB,
		CurrentRatio:   ratio,
		ImbalanceFlag:  ratio > ImbalanceThreshold,
		FeeRateBps:     feeBps,
		TotalLPCount:   lpCount,
		PendingCommits: pendingCommits,
		UpdatedAt:      time.Now(),
	}, nil
}

// derivePoolStatus maps (reserveA, reserveB) to a PoolStatus string (D8 / FR-011).
// EMPTY: both zero; PENDING_COUNTERPART: exactly one side > 0; ACTIVE: both > 0.
func derivePoolStatus(reserveA, reserveB string) string {
	zeroA := reserveA == "" || reserveA == "0"
	zeroB := reserveB == "" || reserveB == "0"
	switch {
	case zeroA && zeroB:
		return "EMPTY"
	case !zeroA && zeroB, zeroA && !zeroB:
		return "PENDING_COUNTERPART"
	default:
		return "ACTIVE"
	}
}
