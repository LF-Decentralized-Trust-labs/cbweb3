// SPDX-License-Identifier: Apache-2.0

// Package services provides PairService — the business logic layer for pair proposal and confirmation.
// Bridges PairHandler requests to the on-chain PairRegistryClient and the DB PairRepository (D9/D11).
package services

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	PairID     string `json:"pair_id"`
	Status     string `json:"status"`
	TxHash     string `json:"tx_hash"`
	AMMAddress string `json:"amm_address"`
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
	// GetAllPairs returns every on-chain pair regardless of status (PROPOSED and ACTIVE).
	GetAllPairs(ctx context.Context) ([]domain.PairEntry, error)
	// DeployDedicatedAMM deploys a fresh per-pair AMM over (tokenA, tokenB) and
	// returns its address, signed by the proposing CB's hub key. Used by
	// ProposePair when the caller supplies no amm_address.
	DeployDedicatedAMM(ctx context.Context, tokenA, tokenB string) (string, error)
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

// zeroEVMAddress is what IdentityRegistry.getCentralBankOf returns for a token it has no
// mapping for — an unresolved authority, not a rightful confirmer.
const zeroEVMAddress = "0x0000000000000000000000000000000000000000"

// TokenAuthorityReader resolves, on the hub, which address the IdentityRegistry recognises as a
// token's issuing central bank, and which address this gateway signs with.
//
// It exists so an unauthorized confirm is refused BEFORE a transaction is spent on a certain
// revert. PairRegistry gates confirmPair on getCentralBankOf(tokenB); the revert carries no
// decoded reason through the EVM client, so without this pre-check the operator sees a generic
// "transaction reverted — check contract permissions and token allowances" and a wasted tx.
// Since each currency's issuance authority moved to its own central bank, a CB attempting to
// confirm a corridor it does not own is an ordinary operator mistake, not an exotic failure.
type TokenAuthorityReader interface {
	CentralBankOfToken(ctx context.Context, tokenAddress string) (string, error)
	HubSignerAddress() string
}

// PairService implements PairServiceIface.
type PairService struct {
	client    PairRegistryClientIface
	repo      PairRepositoryIface
	authority TokenAuthorityReader
}

// NewPairService creates a PairService backed by an on-chain client and a DB repo.
// client may be nil for read-only mode (ListActivePairs only).
func NewPairService(client PairRegistryClientIface, repo PairRepositoryIface) *PairService {
	return &PairService{client: client, repo: repo}
}

// WithTokenAuthorityReader attaches the hub authority reader used to pre-check confirm rights.
// Optional: without it an unauthorized confirm still fails, just later and less clearly.
func (s *PairService) WithTokenAuthorityReader(r TokenAuthorityReader) *PairService {
	s.authority = r
	return s
}

// confirmAuthorityCheck refuses a confirm this gateway is not entitled to make.
//
// Read-only and best-effort: any inability to resolve the authority (unknown pair, RPC error,
// reader not wired) falls through to the on-chain attempt rather than blocking a legitimate
// confirm on a failed read. Only a definite mismatch is refused.
func (s *PairService) confirmAuthorityCheck(ctx context.Context, pairID string) error {
	if s.authority == nil {
		return nil
	}
	self := strings.TrimSpace(s.authority.HubSignerAddress())
	if self == "" {
		return nil
	}
	pairs, err := s.client.GetAllPairs(ctx)
	if err != nil {
		log.Printf("[pair] confirm pre-check: could not list pairs for %q: %v (falling through to on-chain)", pairID, err)
		return nil
	}
	tokenB := ""
	for _, p := range pairs {
		if p.PairID == pairID {
			tokenB = p.TokenB
			break
		}
	}
	if tokenB == "" {
		log.Printf("[pair] confirm pre-check: pair %q not found on-chain (falling through to on-chain)", pairID)
		return nil
	}
	expected, err := s.authority.CentralBankOfToken(ctx, tokenB)
	if err != nil {
		log.Printf("[pair] confirm pre-check: could not resolve the central bank of %s: %v (falling through to on-chain)", tokenB, err)
		return nil
	}
	expected = strings.TrimSpace(expected)
	// An absent mapping (empty or the zero address) is not a mismatch — it means the registry
	// has no issuing central bank for this token yet. Reporting it as "you are not the CB"
	// would name address zero as the rightful confirmer; leave it to the chain.
	if expected == "" || expected == zeroEVMAddress {
		log.Printf("[pair] confirm pre-check: token %s has no central bank on the hub registry (falling through to on-chain)", tokenB)
		return nil
	}
	if strings.EqualFold(expected, self) {
		return nil
	}
	return fmt.Errorf("%w: confirming %s requires the central bank of its token B (%s); this gateway signs as %s",
		ErrNotCentralBankOfTokenB, pairID, expected, self)
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

	// A corridor MUST own a dedicated AMM whose immutable TOKEN_A/TOKEN_B are the
	// pair's own tokens. When the caller supplies no amm_address, deploy one now
	// (signed by the proposing CB's hub key) instead of binding a shared AMM.
	if strings.TrimSpace(req.AMMAddress) == "" {
		ammAddr, derr := s.client.DeployDedicatedAMM(ctx, req.TokenAAddress, req.TokenBAddress)
		if derr != nil {
			return nil, fmt.Errorf("pair service: deploy dedicated AMM: %w", derr)
		}
		req.AMMAddress = ammAddr
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

	return &PairProposeResult{PairID: req.PairID, Status: domain.PairStatusProposed, TxHash: txHash, AMMAddress: req.AMMAddress}, nil
}

// ConfirmPair submits an on-chain confirmPair transaction and updates the DB record to ACTIVE.
// If the pair was proposed via another gateway (not in local DB), syncs from on-chain after confirming.
func (s *PairService) ConfirmPair(ctx context.Context, req PairConfirmRequest) (*PairConfirmResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("pair service: on-chain client not configured (PAIR_REGISTRY_CONTRACT_ADDRESS missing)")
	}
	if err := s.confirmAuthorityCheck(ctx, req.PairID); err != nil {
		return nil, err
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

// ListActivePairs returns the full set of pairs, using the on-chain PairRegistry as the
// source of truth for pair existence and status (PROPOSED and ACTIVE). This enables cross-CB
// discovery: a pair proposed via one Central Bank gateway is visible to the counterparty CB
// even before it is confirmed. Local DB rows are merged in by PairID to supply human labels
// (proposer/confirmer CB names, timestamps). When the on-chain call fails, the method falls
// back to the DB-only view. When no client is configured, DB-only behaviour is preserved.
func (s *PairService) ListActivePairs(ctx context.Context) ([]domain.PairProposal, error) {
	dbPairs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// No on-chain client: preserve legacy DB-only behaviour.
	if s.client == nil {
		return dbPairs, nil
	}

	onChain, cerr := s.client.GetAllPairs(ctx)
	if cerr != nil {
		return dbPairs, nil // on-chain unavailable — return DB view
	}

	// Index DB rows by PairID to enrich on-chain entries with human labels.
	dbByID := make(map[string]domain.PairProposal, len(dbPairs))
	for _, p := range dbPairs {
		dbByID[p.PairID] = p
	}

	result := make([]domain.PairProposal, 0, len(onChain))
	for _, e := range onChain {
		status := e.Status
		if status == "" {
			status = domain.PairStatusActive // defensive: treat unlabelled on-chain entries as ACTIVE
		}
		proposal := domain.PairProposal{
			PairID:        e.PairID,
			TokenAAddress: e.TokenA,
			TokenBAddress: e.TokenB,
			AMMAddress:    e.AMMAddress,
			Status:        status,
		}
		// Merge DB-supplied human labels / timestamps when available.
		if db, ok := dbByID[e.PairID]; ok {
			proposal.ProposerCB = db.ProposerCB
			proposal.ConfirmerCB = db.ConfirmerCB
			proposal.ProposedAt = db.ProposedAt
			proposal.ConfirmedAt = db.ConfirmedAt
		}
		result = append(result, proposal)
	}

	return result, nil
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
