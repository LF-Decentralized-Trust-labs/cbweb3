// Package services provides the liquidity provision service for Scenario B (FR-027 / data-model.md §9).
// Extended for 005-cooperative-liquidity: commit-reveal, fee distribution, proportional withdrawal.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AMLiquidityAdder executes add/remove-liquidity transactions on the Hub AMM.
type AMLiquidityAdder interface {
	AddLiquidity(ctx context.Context, pair, providerID, tokenAAmount, tokenBAmount string) (lpShares string, err error)
	// Escrow-and-finalize paired deposit (decision D6): each side is escrowed against a shared
	// commit key, then finalized once both are present (mints proportional LP shares on-chain).
	DepositForCommit(ctx context.Context, commitID [32]byte, isTokenA bool, amount *big.Int, shareRecipient string) error
	FinalizeCommit(ctx context.Context, commitID [32]byte) error
	// RemoveLiquidityShares burns the share owner's on-chain LP shares and returns a single (home)
	// currency (decision D1; homeIsTokenA selects the side). The configured signer must own the
	// shares (decision D4).
	RemoveLiquidityShares(ctx context.Context, shares *big.Int, homeIsTokenA bool, minAmountOut *big.Int) (amountOut string, err error)
	// LPBalanceOf reads the on-chain LP-share balance — the source of truth for pool ownership.
	LPBalanceOf(ctx context.Context, holder string) (*big.Int, error)
	// GetPoolReserves returns live on-chain reserves (D13 / FR-007).
	GetPoolReserves(ctx context.Context, pair string) (reserveA, reserveB string, ratio float64, err error)
}

// LPResult is returned from AddLiquidity / RemoveLiquidity operations.
type LPResult struct {
	LPID             string    `json:"lp_id"`
	PoolPair         string    `json:"pool_pair"`
	ProviderBankID   string    `json:"provider_bank_id"`
	TokenAAmount     string    `json:"token_a_amount"`
	TokenBAmount     string    `json:"token_b_amount"`
	LPShares         string    `json:"lp_shares"`
	DepositSide      string    `json:"deposit_side,omitempty"`
	SharesPercentage *float64  `json:"shares_percentage,omitempty"`
	WithdrawalMode   string    `json:"withdrawal_mode,omitempty"`
	FeeClaimPaid     string    `json:"fee_claim_paid,omitempty"`
	AddedAt          time.Time `json:"added_at,omitempty"`
	WithdrawnAt      time.Time `json:"withdrawn_at,omitempty"`
}

// CommitResult is returned from RegisterCommit.
// LPIDs is populated only when status = EXECUTED (commit-reveal auto-matched).
// lp_ids[0] = providerA position, lp_ids[1] = providerB position (FR-002).
// OnChainCommitID is the bytes32 commitId returned by LiquidityCommitRegistry (007-bridge-based-cb-liquidity).
type CommitResult struct {
	CommitID        string                 `json:"commit_id"`
	PoolPair        string                 `json:"pool_pair"`
	Side            apidomain.CommitSide   `json:"side"`
	Amount          string                 `json:"amount"`
	Status          apidomain.CommitStatus `json:"status"`
	ExpiresAt       time.Time              `json:"expires_at"`
	LPIDs           []string               `json:"lp_ids,omitempty"`
	OnChainCommitID string                 `json:"on_chain_commit_id,omitempty"`
}

// LiquidityProvisionRequest carries parameters for add-liquidity calls.
type LiquidityProvisionRequest struct {
	PoolPair       string
	ProviderBankID string
	TokenAAmount   string
	TokenBAmount   string
}

// LiquidityRemoveRequest carries parameters for remove-liquidity calls.
type LiquidityRemoveRequest struct {
	PoolPair       string
	ProviderBankID string
	LPID           string
}

// CommitRequest carries parameters for a commit-reveal deposit intent.
// OnChainCommitID is optional — set by the handler when LiquidityCommitRegistry.registerCommit
// was called before persisting the commit (007-bridge-based-cb-liquidity).
type CommitRequest struct {
	PoolPair        string
	ProviderID      string
	Side            apidomain.CommitSide
	Amount          string
	OnChainCommitID []byte // nil for cooperative (off-chain match); bytes32 for sovereign (on-chain match)
}

// PoolCommitRepo is the repository interface consumed by the service.
type PoolCommitRepo interface {
	Create(ctx context.Context, commit *apidomain.PoolCommit) error
	FindActiveByPairAndSide(ctx context.Context, poolPair string, side apidomain.CommitSide) (*apidomain.PoolCommit, error)
	FindByProviderAndPair(ctx context.Context, providerID, poolPair string) (*apidomain.PoolCommit, error)
	UpdateStatus(ctx context.Context, commitID string, status apidomain.CommitStatus) error
	LinkCounterpart(ctx context.Context, commitID, counterpartID string) error
	FindByID(ctx context.Context, commitID string) (*apidomain.PoolCommit, error)
	ListByPair(ctx context.Context, poolPair, status string) ([]apidomain.PoolCommit, error)
	// UpdateStatusByOnChainCommitID transitions a commit identified by its on-chain bytes32 commitId.
	UpdateStatusByOnChainCommitID(ctx context.Context, onChainCommitIDHex string, status apidomain.CommitStatus) error
	// FindByOnChainCommitID returns the commit associated with the given on-chain bytes32 commitId.
	FindByOnChainCommitID(ctx context.Context, onChainCommitIDHex string) (*apidomain.PoolCommit, error)
}

// LPFeeEventRepo is the repository interface for fee event persistence.
type LPFeeEventRepo interface {
	Create(ctx context.Context, event *apidomain.LPFeeEvent) error
	FindBySwapOrderID(ctx context.Context, orderID string) (*apidomain.LPFeeEvent, error)
}

// LiquidityProvisionService manages on-chain liquidity provision with GORM persistence.
type LiquidityProvisionService struct {
	db         *gorm.DB
	amm        AMLiquidityAdder
	commitRepo PoolCommitRepo
	feeRepo    LPFeeEventRepo
}

// NewLiquidityProvisionService creates a LiquidityProvisionService.
func NewLiquidityProvisionService(db *gorm.DB, amm AMLiquidityAdder) *LiquidityProvisionService {
	return &LiquidityProvisionService{db: db, amm: amm}
}

// NewLiquidityProvisionServiceWithRepos creates a LiquidityProvisionService with explicit repos.
func NewLiquidityProvisionServiceWithRepos(db *gorm.DB, amm AMLiquidityAdder, commitRepo PoolCommitRepo, feeRepo LPFeeEventRepo) *LiquidityProvisionService {
	return &LiquidityProvisionService{db: db, amm: amm, commitRepo: commitRepo, feeRepo: feeRepo}
}

// commitRepoFromDB returns the commit repo, falling back to an in-process implementation.
func (s *LiquidityProvisionService) commitRepoOrDB() PoolCommitRepo {
	if s.commitRepo != nil {
		return s.commitRepo
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
//  LEGACY DUAL-SIDED DEPOSIT (backward-compat, deposit_side = BOTH)
// ─────────────────────────────────────────────────────────────────────────────

// AddLiquidity submits a dual-sided liquidity deposit to the AMM and persists the position (FR-027).
// Creates a LEGACY position (deposit_side = BOTH). shares_percentage is calculated
// proportionally and all active positions for the pool are recalculated (FR-003).
func (s *LiquidityProvisionService) AddLiquidity(ctx context.Context, req LiquidityProvisionRequest) (*LPResult, error) {
	if req.PoolPair == "" || req.ProviderBankID == "" {
		return nil, fmt.Errorf("pool_pair and provider_bank_id are required")
	}
	if _, ok := new(big.Int).SetString(req.TokenAAmount, 10); !ok {
		return nil, fmt.Errorf("invalid token_a_amount")
	}
	if _, ok := new(big.Int).SetString(req.TokenBAmount, 10); !ok {
		return nil, fmt.Errorf("invalid token_b_amount")
	}

	lpShares, err := s.amm.AddLiquidity(ctx, req.PoolPair, req.ProviderBankID, req.TokenAAmount, req.TokenBAmount)
	if err != nil {
		return nil, fmt.Errorf("amm add-liquidity failed: %w", err)
	}

	now := time.Now()
	pos := &apidomain.LiquidityPosition{
		LPID:                uuid.NewString(),
		ProviderBankID:      req.ProviderBankID,
		PoolPair:            req.PoolPair,
		TokenAContributed:   req.TokenAAmount,
		TokenBContributed:   req.TokenBAmount,
		LPShares:            lpShares,
		AddedAt:             now,
		DepositSide:         apidomain.DepositSideBoth,
		FeeClaimAccumulated: "0",
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(pos).Error; err != nil {
			return err
		}
		return s.recalculateSharesInTx(ctx, tx, req.PoolPair)
	}); err != nil {
		return nil, fmt.Errorf("persist liquidity position failed: %w", err)
	}

	sp := pos.SharesPercentage
	return &LPResult{
		LPID:             pos.LPID,
		PoolPair:         req.PoolPair,
		ProviderBankID:   req.ProviderBankID,
		TokenAAmount:     req.TokenAAmount,
		TokenBAmount:     req.TokenBAmount,
		LPShares:         lpShares,
		DepositSide:      string(apidomain.DepositSideBoth),
		SharesPercentage: sp,
		AddedAt:          now,
	}, nil
}

// RemoveLiquidity burns LP shares and records the withdrawal timestamp (FR-027).
// Routes to LEGACY or PROPORTIONAL withdrawal based on deposit_side (D7).
func (s *LiquidityProvisionService) RemoveLiquidity(ctx context.Context, req LiquidityRemoveRequest) (*LPResult, error) {
	if req.PoolPair == "" || req.ProviderBankID == "" || req.LPID == "" {
		return nil, fmt.Errorf("pool_pair, provider_bank_id, and lp_id are required")
	}

	var pos apidomain.LiquidityPosition
	if err := s.db.WithContext(ctx).
		Where("lp_id = ? AND provider_bank_id = ? AND pool_pair = ? AND status = ?",
			req.LPID, req.ProviderBankID, req.PoolPair, apidomain.LPStatusActive).
		First(&pos).Error; err != nil {
		return nil, fmt.Errorf("active liquidity position not found: %w", err)
	}

	return s.removeShares(ctx, &pos)
}

// removeShares withdraws via the on-chain LP-share model (decisions D1/D4): it burns the share
// owner's LP shares and returns a single home currency (the provider's deposit side), zap-swapping
// the other side. The configured signer must own the shares.
//
// NOTE (off happy-path; flagged for follow-up): for the commercial cooperative flow shares are
// currently owned by the operator gateway (the off-chain position maps them to the bank). A faithful
// multi-bank withdrawal requires the owning bank to sign the burn; tracked as a follow-up. The
// sovereign flow already mints to the CB's own address, so CB-signed withdrawal is correct there.
func (s *LiquidityProvisionService) removeShares(ctx context.Context, pos *apidomain.LiquidityPosition) (*LPResult, error) {
	// Resolve the share count to burn. Prefer the position's recorded shares; fall back to the
	// owner's full on-chain balance when the position predates on-chain share accounting.
	shares, ok := new(big.Int).SetString(pos.LPShares, 10)
	if !ok || shares.Sign() <= 0 {
		bal, err := s.amm.LPBalanceOf(ctx, pos.ProviderBankID)
		if err != nil {
			return nil, fmt.Errorf("resolve LP-share balance for %s: %w", pos.LPID, err)
		}
		shares = bal
	}
	if shares.Sign() <= 0 {
		return nil, fmt.Errorf("position %s has no LP shares to withdraw", pos.LPID)
	}

	// Home currency = the provider's deposit side (B only when deposit_side == B).
	homeIsTokenA := pos.DepositSide != apidomain.DepositSideB

	amountOut, err := s.amm.RemoveLiquidityShares(ctx, shares, homeIsTokenA, big.NewInt(0))
	if err != nil {
		return nil, fmt.Errorf("amm remove-liquidity (shares) failed: %w", err)
	}

	tokenAOut, tokenBOut := amountOut, "0"
	if !homeIsTokenA {
		tokenAOut, tokenBOut = "0", amountOut
	}

	now := time.Now()
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(pos).Updates(map[string]interface{}{
			"withdrawn_at": now,
			"status":       apidomain.LPStatusWithdrawn,
		}).Error; err != nil {
			return err
		}
		return s.recalculateSharesInTx(ctx, tx, pos.PoolPair)
	}); err != nil {
		return nil, fmt.Errorf("update withdrawal failed: %w", err)
	}

	return &LPResult{
		LPID:           pos.LPID,
		PoolPair:       pos.PoolPair,
		ProviderBankID: pos.ProviderBankID,
		TokenAAmount:   tokenAOut,
		TokenBAmount:   tokenBOut,
		LPShares:       shares.String(),
		WithdrawalMode: "SHARES_HOME_CURRENCY",
		FeeClaimPaid:   "0",
		AddedAt:        pos.AddedAt,
		WithdrawnAt:    now,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
//  COMMIT-REVEAL (US1 — cooperative single-sided deposits)
// ─────────────────────────────────────────────────────────────────────────────

// RegisterCommit records a deposit intent and executes the commit-reveal if both sides are present.
// Enforces: (1) LP role, (2) unique (pool_pair, side) for PENDING/MATCHED, (3) no same-provider both sides.
// When a counterpart is found, both on-chain transfers are executed and LiquidityPositions are created (FR-002).
func (s *LiquidityProvisionService) RegisterCommit(ctx context.Context, req CommitRequest) (*CommitResult, error) {
	if req.PoolPair == "" || req.ProviderID == "" || req.Amount == "" {
		return nil, fmt.Errorf("pool_pair, provider_id, and amount are required")
	}
	if req.Side != apidomain.CommitSideA && req.Side != apidomain.CommitSideB {
		return nil, fmt.Errorf("side must be 'A' or 'B'")
	}
	if _, ok := new(big.Int).SetString(req.Amount, 10); !ok {
		return nil, fmt.Errorf("invalid amount: %s", req.Amount)
	}

	if s.commitRepo == nil {
		return nil, fmt.Errorf("commit repository not configured; call NewLiquidityProvisionServiceWithRepos")
	}

	// Check SAME_PROVIDER_BOTH_SIDES (FR-002 / clarification Q3).
	if existing, err := s.commitRepo.FindByProviderAndPair(ctx, req.ProviderID, req.PoolPair); err == nil && existing != nil {
		if existing.Side != req.Side {
			return nil, &CommitError{Code: "SAME_PROVIDER_BOTH_SIDES", Message: "provider already has an active commit on the opposite side of this pool pair"}
		}
		// Same side — check for existing commit.
		return nil, &CommitError{Code: "COMMIT_ALREADY_EXISTS", Message: "an active commit already exists for this provider and pool pair"}
	}

	// Check if the side already has an active commit from another provider.
	if _, err := s.commitRepo.FindActiveByPairAndSide(ctx, req.PoolPair, req.Side); err == nil {
		return nil, &CommitError{Code: "COMMIT_ALREADY_EXISTS", Message: "a PENDING or MATCHED commit already exists for this pool pair and side"}
	}

	now := time.Now().UTC()
	commit := &apidomain.PoolCommit{
		CommitID:   uuid.NewString(),
		PoolPair:   req.PoolPair,
		ProviderID: req.ProviderID,
		Side:       req.Side,
		Amount:     req.Amount,
		Status:     apidomain.CommitStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(72 * time.Hour),
	}
	// Persist on_chain_commit_id if the handler already called LiquidityCommitRegistry.registerCommit.
	if len(req.OnChainCommitID) > 0 {
		id := make([]byte, len(req.OnChainCommitID))
		copy(id, req.OnChainCommitID)
		commit.OnChainCommitID = &id
	}

	if err := s.commitRepo.Create(ctx, commit); err != nil {
		return nil, fmt.Errorf("create commit: %w", err)
	}

	// Attempt to find counterpart.
	counterpart, err := s.commitRepo.FindActiveByPairAndSide(ctx, req.PoolPair, commit.OppositeSide())
	if errors.Is(err, gorm.ErrRecordNotFound) || counterpart == nil {
		// No counterpart yet — pool stays PENDING_COUNTERPART.
		return commitToResult(commit), nil
	}
	if err != nil {
		return nil, fmt.Errorf("find counterpart: %w", err)
	}

	// Counterpart found — link and execute both transfers.
	if err := s.commitRepo.LinkCounterpart(ctx, commit.CommitID, counterpart.CommitID); err != nil {
		return nil, fmt.Errorf("link counterpart: %w", err)
	}

	// Execute on-chain transfers for both sides (FR-002).
	lpIDs, execErr := s.executeMatchedCommits(ctx, commit, counterpart)
	if execErr != nil {
		return nil, execErr
	}

	commit.Status = apidomain.CommitStatusExecuted
	result := commitToResult(commit)
	result.LPIDs = lpIDs
	return result, nil
}

// executeMatchedCommits sends both addSingleSidedLiquidity transactions and creates
// LiquidityPosition records for each provider. Implements FR-014 retry logic.
// Returns [lpIDA, lpIDB] so RegisterCommit can include them in CommitResult.lp_ids (FR-002).
func (s *LiquidityProvisionService) executeMatchedCommits(ctx context.Context, c1, c2 *apidomain.PoolCommit) ([]string, error) {
	// Determine which is side A and which is side B.
	commitA, commitB := c1, c2
	if c1.Side == apidomain.CommitSideB {
		commitA, commitB = c2, c1
	}

	amtA, _ := new(big.Int).SetString(commitA.Amount, 10)
	amtB, _ := new(big.Int).SetString(commitB.Amount, 10)

	// Escrow-and-finalize (decision D6): the operator gateway escrows both matched sides against a
	// shared key, then finalizes (moves escrow → reserves, mints proportional shares). Unlike the
	// prior two-tx flow, a failure before finalize leaves only escrowed tokens — refundable via
	// cancelCommitDeposit — never a partial half-funded pool.
	escrowKey := deriveEscrowKey(commitA.CommitID, commitB.CommitID)

	if err := s.amm.DepositForCommit(ctx, escrowKey, true, amtA, ""); err != nil {
		_ = s.commitRepo.UpdateStatus(ctx, c1.CommitID, apidomain.CommitStatusReconciliationRequired)
		_ = s.commitRepo.UpdateStatus(ctx, c2.CommitID, apidomain.CommitStatusReconciliationRequired)
		return nil, fmt.Errorf("depositForCommit TOKEN_A: %w", err)
	}

	if err := retryWithBackoff(ctx, 5, func(attempt int) error {
		return s.amm.DepositForCommit(ctx, escrowKey, false, amtB, "")
	}); err != nil {
		_ = s.commitRepo.UpdateStatus(ctx, c1.CommitID, apidomain.CommitStatusReconciliationRequired)
		_ = s.commitRepo.UpdateStatus(ctx, c2.CommitID, apidomain.CommitStatusReconciliationRequired)
		return nil, fmt.Errorf("depositForCommit TOKEN_B (side A escrowed, refundable): %w; both commits → RECONCILIATION_REQUIRED", err)
	}

	if err := retryWithBackoff(ctx, 5, func(attempt int) error {
		return s.amm.FinalizeCommit(ctx, escrowKey)
	}); err != nil {
		_ = s.commitRepo.UpdateStatus(ctx, c1.CommitID, apidomain.CommitStatusReconciliationRequired)
		_ = s.commitRepo.UpdateStatus(ctx, c2.CommitID, apidomain.CommitStatusReconciliationRequired)
		return nil, fmt.Errorf("finalizeCommit (both sides escrowed, refundable): %w; both commits → RECONCILIATION_REQUIRED", err)
	}

	// Both transfers confirmed — mark commits as EXECUTED and create LP positions.
	_ = s.commitRepo.UpdateStatus(ctx, c1.CommitID, apidomain.CommitStatusExecuted)
	_ = s.commitRepo.UpdateStatus(ctx, c2.CommitID, apidomain.CommitStatusExecuted)

	now := time.Now()
	posA := &apidomain.LiquidityPosition{
		LPID:                uuid.NewString(),
		ProviderBankID:      commitA.ProviderID,
		PoolPair:            commitA.PoolPair,
		TokenAContributed:   commitA.Amount,
		TokenBContributed:   "0",
		LPShares:            "0",
		AddedAt:             now,
		Status:              apidomain.LPStatusActive,
		DepositSide:         apidomain.DepositSideA,
		FeeClaimAccumulated: "0",
		CommitID:            &commitA.CommitID,
	}
	posB := &apidomain.LiquidityPosition{
		LPID:                uuid.NewString(),
		ProviderBankID:      commitB.ProviderID,
		PoolPair:            commitB.PoolPair,
		TokenAContributed:   "0",
		TokenBContributed:   commitB.Amount,
		LPShares:            "0",
		AddedAt:             now,
		Status:              apidomain.LPStatusActive,
		DepositSide:         apidomain.DepositSideB,
		FeeClaimAccumulated: "0",
		CommitID:            &commitB.CommitID,
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(posA).Error; err != nil {
			return err
		}
		if err := tx.Create(posB).Error; err != nil {
			return err
		}
		return s.recalculateSharesInTx(ctx, tx, commitA.PoolPair)
	}); err != nil {
		return nil, fmt.Errorf("executeMatchedCommits tx: %w", err)
	}

	// Seed an initial pool_state_reading so that removeProportional can proceed
	// immediately without waiting for the LiquidityMonitorService to poll.
	// After a fresh commit-reveal the reserves equal the two committed amounts.
	amtAF, _ := new(big.Float).SetString(commitA.Amount)
	amtBF, _ := new(big.Float).SetString(commitB.Amount)
	var ratio float64
	if amtAF != nil && amtBF != nil {
		af64, _ := amtAF.Float64()
		bf64, _ := amtBF.Float64()
		if af64 > 0 {
			ratio = bf64 / af64
		}
	}
	lpCount := 2
	reading := &apidomain.PoolStateReading{
		ReadingID:    uuid.NewString(),
		PoolPair:     commitA.PoolPair,
		ReserveA:     commitA.Amount,
		ReserveB:     commitB.Amount,
		CurrentRatio: ratio,
		RecordedAt:   now,
		FeeRateBps:   30,
		TotalLPCount: &lpCount,
		PoolStatus:   apidomain.PoolStatusActive,
	}
	if err := s.db.WithContext(ctx).Create(reading).Error; err != nil {
		log.Printf("[liquidity_provision] seed pool_state_reading after commit-reveal: %v (non-fatal)", err)
	}

	// lp_ids[0] = providerA (side A), lp_ids[1] = providerB (side B) — order matches commit sides.
	return []string{posA.LPID, posB.LPID}, nil
}

// GetCommit returns a PoolCommit by ID.
func (s *LiquidityProvisionService) GetCommit(ctx context.Context, commitID string) (*apidomain.PoolCommit, error) {
	if s.commitRepo == nil {
		return nil, fmt.Errorf("commit repository not configured")
	}
	return s.commitRepo.FindByID(ctx, commitID)
}

// ListCommits returns all commits for a pool pair, optionally filtered by status.
func (s *LiquidityProvisionService) ListCommits(ctx context.Context, poolPair, status string) ([]apidomain.PoolCommit, error) {
	if s.commitRepo == nil {
		return nil, fmt.Errorf("commit repository not configured")
	}
	return s.commitRepo.ListByPair(ctx, poolPair, status)
}

// CancelCommit marks a PENDING commit as EXPIRED. Returns COMMIT_ALREADY_MATCHED if not PENDING.
func (s *LiquidityProvisionService) CancelCommit(ctx context.Context, commitID, providerID string) error {
	if s.commitRepo == nil {
		return fmt.Errorf("commit repository not configured")
	}
	commit, err := s.commitRepo.FindByID(ctx, commitID)
	if err != nil {
		return &CommitError{Code: "COMMIT_NOT_FOUND", Message: "commit not found"}
	}
	if commit.ProviderID != providerID {
		return &CommitError{Code: "NOT_AUTHORIZED_LP", Message: "commit belongs to a different provider"}
	}
	if commit.Status == apidomain.CommitStatusMatched {
		return &CommitError{Code: "COMMIT_ALREADY_MATCHED", Message: "commit cannot be cancelled after matching"}
	}
	if commit.Status != apidomain.CommitStatusPending {
		return &CommitError{Code: "COMMIT_ALREADY_MATCHED", Message: fmt.Sprintf("commit is in terminal status: %s", commit.Status)}
	}
	return s.commitRepo.UpdateStatus(ctx, commitID, apidomain.CommitStatusExpired)
}

// ─────────────────────────────────────────────────────────────────────────────
//  FEE DISTRIBUTION (US3)
// ─────────────────────────────────────────────────────────────────────────────

// RecordSwapFee creates an LPFeeEvent and updates fee_claim_accumulated for all active LPs (FR-005/FR-006).
func (s *LiquidityProvisionService) RecordSwapFee(ctx context.Context, poolPair, swapOrderID string, feeAmountA, feeAmountB *big.Int) error {
	if s.feeRepo == nil {
		return nil // fee tracking not configured; skip gracefully
	}

	// Fetch all active LP positions for this pool pair.
	var positions []apidomain.LiquidityPosition
	if err := s.db.WithContext(ctx).
		Where("pool_pair = ? AND status = ?", poolPair, apidomain.LPStatusActive).
		Find(&positions).Error; err != nil {
		return fmt.Errorf("fetch active positions: %w", err)
	}
	if len(positions) == 0 {
		return nil
	}

	// Build distribution snapshot.
	dist := make(map[string]string, len(positions))
	for _, p := range positions {
		pct := 0.0
		if p.SharesPercentage != nil {
			pct = *p.SharesPercentage
		}
		dist[p.LPID] = fmt.Sprintf("%.4f", pct)
	}
	distJSON, err := json.Marshal(dist)
	if err != nil {
		return fmt.Errorf("marshal distribution: %w", err)
	}

	event := &apidomain.LPFeeEvent{
		EventID:      uuid.NewString(),
		PoolPair:     poolPair,
		SwapOrderID:  swapOrderID,
		FeeAmountA:   feeAmountA.String(),
		FeeAmountB:   feeAmountB.String(),
		Distribution: string(distJSON),
	}
	if err := s.feeRepo.Create(ctx, event); err != nil {
		return fmt.Errorf("create fee event: %w", err)
	}

	// Batch update fee_claim_accumulated for each LP (T027).
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, p := range positions {
			if p.SharesPercentage == nil {
				continue
			}
			pct := *p.SharesPercentage
			claimA := new(big.Float).Mul(new(big.Float).SetInt(feeAmountA), big.NewFloat(pct/100))
			claimB := new(big.Float).Mul(new(big.Float).SetInt(feeAmountB), big.NewFloat(pct/100))
			claimAInt, _ := claimA.Int(nil)
			claimBInt, _ := claimB.Int(nil)

			existing, _ := new(big.Int).SetString(p.FeeClaimAccumulated, 10)
			if existing == nil {
				existing = big.NewInt(0)
			}
			// Store fee_claim_accumulated as sum of token A credits (simplification; token B tracked separately if needed).
			newClaim := new(big.Int).Add(existing, new(big.Int).Add(claimAInt, claimBInt))

			if err := tx.Model(&apidomain.LiquidityPosition{}).
				Where("lp_id = ?", p.LPID).
				Update("fee_claim_accumulated", newClaim.String()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ─────────────────────────────────────────────────────────────────────────────
//  HELPERS
// ─────────────────────────────────────────────────────────────────────────────

// recalculateSharesInTx recomputes shares_percentage for all ACTIVE positions
// of a pool pair within a transaction. Guarantees SUM = 100% (FR-003 / clarification Q5).
func (s *LiquidityProvisionService) recalculateSharesInTx(ctx context.Context, tx *gorm.DB, poolPair string) error {
	var positions []apidomain.LiquidityPosition
	if err := tx.WithContext(ctx).
		Where("pool_pair = ? AND status = ?", poolPair, apidomain.LPStatusActive).
		Find(&positions).Error; err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	// Calculate total value in pool (token_a + token_b contributions as proxy).
	totalA := new(big.Float)
	totalB := new(big.Float)
	for _, p := range positions {
		if a, ok := new(big.Float).SetString(p.TokenAContributed); ok {
			totalA.Add(totalA, a)
		}
		if b, ok := new(big.Float).SetString(p.TokenBContributed); ok {
			totalB.Add(totalB, b)
		}
	}
	total := new(big.Float).Add(totalA, totalB)
	if total.Sign() == 0 {
		return nil
	}

	for _, p := range positions {
		posA, _ := new(big.Float).SetString(p.TokenAContributed)
		posB, _ := new(big.Float).SetString(p.TokenBContributed)
		posTotal := new(big.Float).Add(posA, posB)
		pct, _ := new(big.Float).Quo(posTotal, total).Mul(new(big.Float).Quo(posTotal, total), big.NewFloat(100)).Float64()

		if err := tx.WithContext(ctx).Model(&apidomain.LiquidityPosition{}).
			Where("lp_id = ?", p.LPID).
			Update("shares_percentage", pct).Error; err != nil {
			return err
		}
	}
	return nil
}

// retryWithBackoff retries fn up to maxAttempts with exponential backoff (2s, 4s, 8s, 16s, 32s, cap 60s).
func retryWithBackoff(ctx context.Context, maxAttempts int, fn func(attempt int) error) error {
	backoff := 2 * time.Second
	maxBackoff := 60 * time.Second
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(i); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
	return lastErr
}

// commitToResult converts a PoolCommit to a CommitResult.
func commitToResult(c *apidomain.PoolCommit) *CommitResult {
	res := &CommitResult{
		CommitID:  c.CommitID,
		PoolPair:  c.PoolPair,
		Side:      c.Side,
		Amount:    c.Amount,
		Status:    c.Status,
		ExpiresAt: c.ExpiresAt,
	}
	if c.OnChainCommitID != nil && len(*c.OnChainCommitID) == 32 {
		res.OnChainCommitID = "0x" + fmt.Sprintf("%x", *c.OnChainCommitID)
	}
	return res
}

// CommitError is a domain error carrying a machine-readable code for API responses.
type CommitError struct {
	Code    string
	Message string
}

func (e *CommitError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
