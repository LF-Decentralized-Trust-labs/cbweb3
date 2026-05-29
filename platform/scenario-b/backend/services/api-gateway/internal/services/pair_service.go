// Package services provides PairService — the business logic layer for pair proposal and confirmation.
// Bridges PairHandler requests to the on-chain PairRegistryClient and the DB PairRepository (D9/D11).
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ── Shared request/response types (consumed by pair_handler.go via import) ──

// PairProposeRequest carries the input for POST /api/v2/amm/pairs/propose.
type PairProposeRequest struct {
	PairID        string `json:"pair_id"`
	TokenAAddress string `json:"token_a_address"`
	TokenBAddress string `json:"token_b_address"`
	AMMAddress    string `json:"amm_address"`
	ProposerCB    string `json:"proposer_cb"`
}

// PairProposeResult is the response body for a successful propose.
type PairProposeResult struct {
	PairID string `json:"pair_id"`
	Status string `json:"status"`
	TxHash string `json:"tx_hash"`
}

// PairConfirmRequest carries the input for POST /api/v2/amm/pairs/confirm.
type PairConfirmRequest struct {
	PairID      string `json:"pair_id"`
	ConfirmerCB string `json:"confirmer_cb"`
}

// PairConfirmResult is the response body for a successful confirm.
type PairConfirmResult struct {
	PairID string `json:"pair_id"`
	Status string `json:"status"`
	TxHash string `json:"tx_hash"`
}

// PairServiceIface is the interface consumed by pair_handler.go.
type PairServiceIface interface {
	ProposePair(ctx context.Context, req PairProposeRequest) (*PairProposeResult, error)
	ConfirmPair(ctx context.Context, req PairConfirmRequest) (*PairConfirmResult, error)
	ListActivePairs(ctx context.Context) ([]domain.PairProposal, error)
}

// ── Error sentinels ──────────────────────────────────────────────────────────

var ErrPairAlreadyExists = errors.New("PAIR_ALREADY_EXISTS")
var ErrPairNotFound = errors.New("PAIR_NOT_FOUND")
var ErrPairAlreadyActive = errors.New("PAIR_ALREADY_ACTIVE")
var ErrNotCentralBankOfTokenA = errors.New("NOT_CENTRAL_BANK_OF_TOKEN_A")
var ErrNotCentralBankOfTokenB = errors.New("NOT_CENTRAL_BANK_OF_TOKEN_B")

// ── PairService implementation ───────────────────────────────────────────────

// PairRegistryClientIface abstracts on-chain PairRegistry calls for PairService.
type PairRegistryClientIface interface {
	ProposePair(ctx context.Context, pairID, tokenA, tokenB, ammAddress string) (string, error)
	ConfirmPair(ctx context.Context, pairID string) (string, error)
	GetAllActivePairs(ctx context.Context) ([]domain.PairEntry, error)
}

// PairRepositoryIface abstracts DB persistence for PairService.
type PairRepositoryIface interface {
	Create(ctx context.Context, p *domain.PairProposal) error
	FindByPairID(ctx context.Context, pairID string) (*domain.PairProposal, error)
	// FindByTokenPair returns any PROPOSED or ACTIVE pair for the given token combination (order-agnostic).
	// Used by ProposePair to enforce FR-011 duplicate token-pair prevention.
	FindByTokenPair(ctx context.Context, tokenA, tokenB string) (*domain.PairProposal, error)
	Activate(ctx context.Context, pairID, confirmerCB string, confirmedAt time.Time) error
	ListActive(ctx context.Context) ([]domain.PairProposal, error)
	ListAll(ctx context.Context) ([]domain.PairProposal, error)
}

// PairService implements PairServiceIface.
type PairService struct {
	client PairRegistryClientIface
	repo   PairRepositoryIface
}

// NewPairService creates a PairService backed by an on-chain client and a DB repo.
// client may be nil for read-only mode (ListActivePairs only).
func NewPairService(client PairRegistryClientIface, repo PairRepositoryIface) *PairService {
	return &PairService{client: client, repo: repo}
}

// ProposePair submits an on-chain proposePair transaction and records the proposal in DB.
// proposer_cb field records SideA (the CB initiating the pair) per FR-008/FR-010.
func (s *PairService) ProposePair(ctx context.Context, req PairProposeRequest) (*PairProposeResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("pair service: on-chain client not configured (PAIR_REGISTRY_CONTRACT_ADDRESS missing)")
	}
	if existing, _ := s.repo.FindByPairID(ctx, req.PairID); existing != nil {
		return nil, fmt.Errorf("%w: pair %q already exists with status %s",
			ErrPairAlreadyExists, req.PairID, existing.Status)
	}
	// FR-011: prevent duplicate pair for the same token combination (order-agnostic).
	if existing, _ := s.repo.FindByTokenPair(ctx, req.TokenAAddress, req.TokenBAddress); existing != nil {
		return nil, fmt.Errorf("%w: a pair for this token combination already exists with status %s",
			ErrPairAlreadyExists, existing.Status)
	}

	txHash, err := s.client.ProposePair(ctx, req.PairID, req.TokenAAddress, req.TokenBAddress, req.AMMAddress)
	if err != nil {
		// Check if pair is already on-chain ACTIVE (idempotent: DB was cleared but chain is source of truth)
		if activePairs, cerr := s.client.GetAllActivePairs(ctx); cerr == nil {
			for _, p := range activePairs {
				if p.PairID == req.PairID {
					return nil, ErrPairAlreadyExists
				}
			}
		}
		return nil, mapOnChainProposePairError(err)
	}

	proposal := &domain.PairProposal{
		PairID:        req.PairID,
		ProposerCB:    req.ProposerCB,
		TokenAAddress: req.TokenAAddress,
		TokenBAddress: req.TokenBAddress,
		AMMAddress:    req.AMMAddress,
		Status:        domain.PairStatusProposed,
		ProposedAt:    time.Now().UTC(),
	}
	_ = s.repo.Create(ctx, proposal) // best-effort; on-chain is source of truth

	return &PairProposeResult{PairID: req.PairID, Status: domain.PairStatusProposed, TxHash: txHash}, nil
}

// ConfirmPair submits an on-chain confirmPair transaction and updates the DB record to ACTIVE.
// If the pair was proposed via another gateway (not in local DB), syncs from on-chain after confirming.
func (s *PairService) ConfirmPair(ctx context.Context, req PairConfirmRequest) (*PairConfirmResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("pair service: on-chain client not configured (PAIR_REGISTRY_CONTRACT_ADDRESS missing)")
	}

	txHash, err := s.client.ConfirmPair(ctx, req.PairID)
	if err != nil {
		// Check if pair is already ACTIVE on-chain (idempotent confirm)
		if activePairs, cerr := s.client.GetAllActivePairs(ctx); cerr == nil {
			for _, p := range activePairs {
				if p.PairID == req.PairID {
					// Pair is already active; sync to local DB and return success
					confirmedAt := time.Now().UTC()
					if aerr := s.repo.Activate(ctx, req.PairID, req.ConfirmerCB, confirmedAt); aerr != nil {
						_ = s.repo.Create(ctx, &domain.PairProposal{
							PairID:        p.PairID,
							ProposerCB:    "",
							ConfirmerCB:   req.ConfirmerCB,
							TokenAAddress: p.TokenA,
							TokenBAddress: p.TokenB,
							AMMAddress:    p.AMMAddress,
							Status:        domain.PairStatusProposed,
							ProposedAt:    confirmedAt,
						})
						_ = s.repo.Activate(ctx, req.PairID, req.ConfirmerCB, confirmedAt)
					}
					return &PairConfirmResult{PairID: req.PairID, Status: domain.PairStatusActive, TxHash: ""}, nil
				}
			}
		}
		return nil, mapOnChainConfirmPairError(err)
	}

	confirmedAt := time.Now().UTC()
	// Try to activate in local DB; if not found (pair proposed via another gateway), sync from on-chain
	if aerr := s.repo.Activate(ctx, req.PairID, req.ConfirmerCB, confirmedAt); aerr != nil {
		if activePairs, cerr := s.client.GetAllActivePairs(ctx); cerr == nil {
			for _, p := range activePairs {
				if p.PairID == req.PairID {
					_ = s.repo.Create(ctx, &domain.PairProposal{
						PairID:        p.PairID,
						ProposerCB:    "",
						ConfirmerCB:   req.ConfirmerCB,
						TokenAAddress: p.TokenA,
						TokenBAddress: p.TokenB,
						AMMAddress:    p.AMMAddress,
						Status:        domain.PairStatusProposed,
						ProposedAt:    confirmedAt,
					})
					_ = s.repo.Activate(ctx, req.PairID, req.ConfirmerCB, confirmedAt)
					break
				}
			}
		}
	}

	return &PairConfirmResult{PairID: req.PairID, Status: domain.PairStatusActive, TxHash: txHash}, nil
}

// ListActivePairs returns all pairs from the DB, syncing any PROPOSED pairs that are already
// ACTIVE on-chain (cross-gateway lazy sync: pair proposed via gateway A, confirmed via gateway B).
func (s *PairService) ListActivePairs(ctx context.Context) ([]domain.PairProposal, error) {
	pairs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// Lazy on-chain sync: only when the client is configured and there are PROPOSED rows.
	if s.client == nil {
		return pairs, nil
	}
	hasPending := false
	for _, p := range pairs {
		if p.Status == domain.PairStatusProposed {
			hasPending = true
			break
		}
	}
	if !hasPending {
		return pairs, nil
	}

	activePairs, cerr := s.client.GetAllActivePairs(ctx)
	if cerr != nil {
		return pairs, nil // on-chain unavailable — return stale DB view
	}
	onChainActive := make(map[string]domain.PairEntry, len(activePairs))
	for _, ap := range activePairs {
		onChainActive[ap.PairID] = ap
	}

	now := time.Now().UTC()
	syncedIDs := make(map[string]bool)
	for i, p := range pairs {
		if p.Status == domain.PairStatusProposed {
			if _, ok := onChainActive[p.PairID]; ok {
				_ = s.repo.Activate(ctx, p.PairID, "", now)
				pairs[i].Status = domain.PairStatusActive
				syncedIDs[p.PairID] = true
			}
		}
	}

	// Add any on-chain ACTIVE pairs not present in local DB at all.
	knownIDs := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		knownIDs[p.PairID] = true
	}
	for _, ap := range activePairs {
		if !knownIDs[ap.PairID] {
			np := &domain.PairProposal{
				PairID:        ap.PairID,
				ProposerCB:    "",
				TokenAAddress: ap.TokenA,
				TokenBAddress: ap.TokenB,
				AMMAddress:    ap.AMMAddress,
				Status:        domain.PairStatusProposed,
				ProposedAt:    now,
			}
			_ = s.repo.Create(ctx, np)
			_ = s.repo.Activate(ctx, ap.PairID, "", now)
			pairs = append(pairs, domain.PairProposal{
				PairID:        ap.PairID,
				TokenAAddress: ap.TokenA,
				TokenBAddress: ap.TokenB,
				AMMAddress:    ap.AMMAddress,
				Status:        domain.PairStatusActive,
				ProposedAt:    now,
			})
		}
	}

	return pairs, nil
}

func mapOnChainProposePairError(err error) error {
	msg := err.Error()
	if containsAny(msg, "AlreadyExists", "PAIR_ALREADY_EXISTS") {
		return ErrPairAlreadyExists
	}
	if containsAny(msg, "Unauthorized", "UNAUTHORIZED") {
		return ErrNotCentralBankOfTokenA
	}
	return err
}

func mapOnChainConfirmPairError(err error) error {
	msg := err.Error()
	if containsAny(msg, "AlreadyActive", "PAIR_ALREADY_ACTIVE") {
		return ErrPairAlreadyActive
	}
	if containsAny(msg, "Unauthorized", "UNAUTHORIZED") {
		return ErrNotCentralBankOfTokenB
	}
	if containsAny(msg, "NotFound", "PAIR_NOT_FOUND") {
		return ErrPairNotFound
	}
	return err
}

func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
