// Package services — SovereignLiquidityService.
// Handles the matched-commit execution path for sovereign CB liquidity pairs
// (007-bridge-based-cb-liquidity / FR-003, FR-005, NFR-001).
//
// Flow (triggered by LiquidityCommitWatcher after CommitMatched on Hub):
//  1. Decode CommitMatched payload to identify which signer belongs to this gateway.
//  2. If neither signer matches LOCAL_CB_HUB_SIGNER → return status:"ignored".
//  3. Look up AMM address from SOVEREIGN_PAIR_AMM_MAP (env JSON, no hardcoding).
//  4. Call AMM.addSingleSidedLiquidity on behalf of the sovereign CB signer.
//  5. Persist PoolCommit.status = EXECUTED and create LiquidityPosition.
//  6. On failure: transition PoolCommit to RECONCILIATION_REQUIRED after ReconcileTimeoutSec.
package services

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"
)

// SovereignExecuteRequest carries the payload from the Cacti watcher for execute-matched-commit.
// Defined here (in services) to avoid a circular import with the handlers package.
type SovereignExecuteRequest struct {
	PoolPair  string `json:"pool_pair"`
	CommitIDA string `json:"commit_id_a"`
	SignerA   string `json:"signer_a"`
	AmountA   string `json:"amount_a"`
	CommitIDB string `json:"commit_id_b"`
	SignerB   string `json:"signer_b"`
	AmountB   string `json:"amount_b"`
}

// sovereignAMMAdder is the subset of AMM adapter needed by the sovereign service.
type sovereignAMMAdder interface {
	// DepositForCommitAt escrows this gateway's single side against the shared escrow key on the
	// AMM at ammAddress (escrow-and-finalize, D6). An empty shareRecipient credits this gateway's
	// own signer (the sovereign CB — decision D4).
	DepositForCommitAt(ctx context.Context, ammAddress string, commitID [32]byte, isTokenA bool, amount *big.Int, shareRecipient string) error
	// FinalizeCommitAt finalizes a fully-deposited commit; an "incomplete" error means the other
	// side has not yet been escrowed (expected for whichever gateway deposits first). On success
	// it returns the minted share split (sharesA, sharesB) from LogCommitFinalized.
	FinalizeCommitAt(ctx context.Context, ammAddress string, commitID [32]byte) (sharesA, sharesB *big.Int, err error)
	// TokenBalanceAt returns the ERC-20 balance of holderAddr for TOKEN_A or TOKEN_B
	// of the AMM at ammAddress. Used for pre-flight balance checks (FR-010 / T021).
	TokenBalanceAt(ctx context.Context, ammAddress string, isTokenA bool, holderAddr string) (*big.Int, error)
}

// SovereignLiquidityService executes matched sovereign liquidity commits for this gateway.
// It implements handlers.SovereignLiquidityServiceIface.
type SovereignLiquidityService struct {
	db         *gorm.DB
	amm        sovereignAMMAdder
	commitRepo PoolCommitRepo
	lpRepo     LPPositionRepo
	// localCBHubSigner is this gateway's sovereign signer address (hex, lowercase).
	localCBHubSigner string
	// sovereignPairAMMMap maps pool_pair → AMM address (loaded from SOVEREIGN_PAIR_AMM_MAP env).
	sovereignPairAMMMap map[string]string
	// ReconcileTimeoutSec is the timeout before a failed commit moves to RECONCILIATION_REQUIRED.
	ReconcileTimeoutSec int
}

// Compile-time assertion: SovereignLiquidityService satisfies SovereignLiquidityServiceIface.
var _ SovereignLiquidityServiceIface = (*SovereignLiquidityService)(nil)

// SovereignLiquidityServiceIface is implemented by SovereignLiquidityService.
// Defined here so handlers can import it without creating a circular dependency.
type SovereignLiquidityServiceIface interface {
	ExecuteMatchedCommit(ctx context.Context, req SovereignExecuteRequest) error
	// IsSovereignPair returns true if poolPair is configured as a sovereign pair.
	IsSovereignPair(poolPair string) bool
	// CheckSufficientWTokenBalance returns nil if holderAddr holds at least `amount`
	// of the W-tCeBM for the given poolPair side. Returns error with code INSUFFICIENT_BALANCE
	// if the balance is too low — used by SovereignAddLiquidity (FR-010 / T021).
	CheckSufficientWTokenBalance(ctx context.Context, poolPair string, isTokenA bool, holderAddr string, amount *big.Int) error
}

// LPPositionRepo is the repository for LiquidityPosition persistence.
// Matches the interface used by LiquidityProvisionService.
type LPPositionRepo interface {
	Create(ctx context.Context, pos *domain.LiquidityPosition) error
	FindByPoolPair(ctx context.Context, poolPair string) ([]domain.LiquidityPosition, error)
	FindByProviderAndPoolPair(ctx context.Context, providerID, poolPair string) ([]domain.LiquidityPosition, error)
}

// SovereignLiquidityServiceConfig holds runtime configuration.
type SovereignLiquidityServiceConfig struct {
	LocalCBHubSigner    string // from LOCAL_CB_HUB_SIGNER env var
	SovereignPairAMMMap string // JSON string from SOVEREIGN_PAIR_AMM_MAP env var
	ReconcileTimeoutSec int    // default: 300
}

// NewSovereignLiquidityServiceFromEnv constructs a SovereignLiquidityService
// reading configuration from standard environment variables.
func NewSovereignLiquidityServiceFromEnv(db *gorm.DB, amm sovereignAMMAdder, commitRepo PoolCommitRepo, lpRepo LPPositionRepo) (*SovereignLiquidityService, error) {
	cfg := SovereignLiquidityServiceConfig{
		LocalCBHubSigner:    strings.ToLower(os.Getenv("LOCAL_CB_HUB_SIGNER")),
		SovereignPairAMMMap: os.Getenv("SOVEREIGN_PAIR_AMM_MAP"),
		ReconcileTimeoutSec: 300,
	}
	return NewSovereignLiquidityService(db, amm, commitRepo, lpRepo, cfg)
}

// NewSovereignLiquidityService constructs a SovereignLiquidityService from config.
func NewSovereignLiquidityService(db *gorm.DB, amm sovereignAMMAdder, commitRepo PoolCommitRepo, lpRepo LPPositionRepo, cfg SovereignLiquidityServiceConfig) (*SovereignLiquidityService, error) {
	pairAMMMap := make(map[string]string)
	if cfg.SovereignPairAMMMap != "" {
		if err := json.Unmarshal([]byte(cfg.SovereignPairAMMMap), &pairAMMMap); err != nil {
			return nil, fmt.Errorf("sovereign liquidity service: invalid SOVEREIGN_PAIR_AMM_MAP JSON: %w", err)
		}
		// Normalize values to lowercase hex.
		for k, v := range pairAMMMap {
			pairAMMMap[k] = strings.ToLower(v)
		}
	}
	timeout := cfg.ReconcileTimeoutSec
	if timeout <= 0 {
		timeout = 300
	}
	return &SovereignLiquidityService{
		db:                  db,
		amm:                 amm,
		commitRepo:          commitRepo,
		lpRepo:              lpRepo,
		localCBHubSigner:    cfg.LocalCBHubSigner,
		sovereignPairAMMMap: pairAMMMap,
		ReconcileTimeoutSec: timeout,
	}, nil
}

// IsSovereignPair returns true if the poolPair is in the SOVEREIGN_PAIR_AMM_MAP.
func (s *SovereignLiquidityService) IsSovereignPair(poolPair string) bool {
	_, ok := s.sovereignPairAMMMap[poolPair]
	return ok
}

// CheckSufficientWTokenBalance checks that holderAddr holds at least `amount` of
// the W-tCeBM associated with the given poolPair side (TOKEN_A or TOKEN_B).
// Returns nil if the balance is sufficient; returns a descriptive error otherwise.
// Called by SovereignAddLiquidity before executing a direct top-up deposit (FR-010 / T021).
func (s *SovereignLiquidityService) CheckSufficientWTokenBalance(ctx context.Context, poolPair string, isTokenA bool, holderAddr string, amount *big.Int) error {
	ammAddress, ok := s.sovereignPairAMMMap[poolPair]
	if !ok {
		return fmt.Errorf("sovereign: unknown pool_pair %q in SOVEREIGN_PAIR_AMM_MAP", poolPair)
	}
	balance, err := s.amm.TokenBalanceAt(ctx, ammAddress, isTokenA, holderAddr)
	if err != nil {
		return fmt.Errorf("sovereign: balance check for %s: %w", holderAddr, err)
	}
	if balance.Cmp(amount) < 0 {
		return &insufficientBalanceError{have: balance, need: amount}
	}
	return nil
}

// insufficientBalanceError is returned by CheckSufficientWTokenBalance when the holder
// does not have enough W-tCeBM. The handler translates this to HTTP 422 INSUFFICIENT_BALANCE.
type insufficientBalanceError struct {
	have *big.Int
	need *big.Int
}

func (e *insufficientBalanceError) Error() string {
	return fmt.Sprintf("insufficient W-tCeBM balance: have %s, need %s", e.have.String(), e.need.String())
}

// IsInsufficientBalance reports whether the error is an insufficientBalanceError.
// Used by the handler to distinguish balance failures from other errors.
func IsInsufficientBalance(err error) bool {
	var e *insufficientBalanceError
	return errors.As(err, &e)
}

// HasActiveBridgePosition delegates to the BridgePositionReader interface.
// HasActiveBridgePosition returns true if ownerBankID has at least one BridgedAssetPosition
// with bridge_state = ACTIVE. Satisfies handlers.SovereignBridgeCheckerIface.
func (s *SovereignLiquidityService) HasActiveBridgePosition(ctx context.Context, ownerBankID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.BridgedAssetPosition{}).
		Where("owner_bank_id = ? AND bridge_state = ?", ownerBankID, domain.BridgeStateActive).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("sovereign: HasActiveBridgePosition: %w", err)
	}
	return count > 0, nil
}

// resolvePoolPair returns the human-readable pair name and AMM address for the given
// pool_pair value, which may be either the literal pair name (e.g. "W-BRL-ARS") or
// its Ethereum keccak256 hash (as sent by the Cacti watcher — Solidity indexed strings
// are replaced by their keccak256 hash in the topic, so topics[1] is a hash, not the
// original string).
func (s *SovereignLiquidityService) resolvePoolPair(poolPair string) (name, ammAddress string, ok bool) {
	// Direct match — literal pair name (e.g. when called from SovereignAddLiquidity).
	if addr, found := s.sovereignPairAMMMap[poolPair]; found {
		return poolPair, addr, true
	}
	// Hash match — Cacti watcher sends keccak256(pairName) from topics[1].
	poolPairLow := strings.ToLower(poolPair)
	for pairName, addr := range s.sovereignPairAMMMap {
		hash := crypto.Keccak256Hash([]byte(pairName)).Hex()
		if strings.ToLower(hash) == poolPairLow {
			return pairName, addr, true
		}
	}
	return "", "", false
}

// ExecuteMatchedCommit processes a CommitMatched event from the Cacti watcher.
// Identifies which side belongs to this gateway and executes the on-chain deposit.
// If neither signer matches this gateway, returns nil (ignored by upstream).
func (s *SovereignLiquidityService) ExecuteMatchedCommit(ctx context.Context, req SovereignExecuteRequest) error {
	poolPairName, ammAddress, ok := s.resolvePoolPair(req.PoolPair)
	if !ok {
		return fmt.Errorf("sovereign liquidity: unknown pool_pair %q — not in SOVEREIGN_PAIR_AMM_MAP", req.PoolPair)
	}

	signerALow := strings.ToLower(req.SignerA)
	signerBLow := strings.ToLower(req.SignerB)
	localLow := s.localCBHubSigner

	// Determine which side this gateway is responsible for.
	var (
		isTokenA bool
		amount   *big.Int
		commitID string
	)
	switch {
	case signerALow == localLow:
		isTokenA = true
		amount = parseBig(req.AmountA)
		commitID = req.CommitIDA
	case signerBLow == localLow:
		isTokenA = false
		amount = parseBig(req.AmountB)
		commitID = req.CommitIDB
	default:
		// This commit does not involve our CB — ignore silently.
		return nil
	}

	if amount == nil || amount.Sign() <= 0 {
		return fmt.Errorf("sovereign liquidity: invalid amount for commit %s", commitID)
	}

	// Idempotency: check if commit is already EXECUTED; if so, skip on-chain call and just ensure LP position exists.
	existingCommit, existingErr := s.commitRepo.FindByOnChainCommitID(ctx, commitID)
	if existingErr == nil && existingCommit.Status == domain.CommitStatusExecuted {
		// Already executed — create LP position if it doesn't exist yet (idempotent).
		s.ensureLPPosition(ctx, existingCommit, poolPairName, isTokenA, nil)
		return nil
	}

	// Escrow this gateway's own side against the shared escrow key, then attempt to finalize
	// (escrow-and-finalize, D6). Shares accrue to this gateway's sovereign signer (D4: the CB
	// owns the shares it funds). Both gateways derive the same key from the matched commit pair.
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(s.ReconcileTimeoutSec)*time.Second)
	defer cancel()

	escrowKey := deriveEscrowKey(req.CommitIDA, req.CommitIDB)
	execErr := s.amm.DepositForCommitAt(execCtx, ammAddress, escrowKey, isTokenA, amount, "")
	if execErr != nil {
		// Transition commit to RECONCILIATION_REQUIRED (only for real on-chain commit IDs).
		if !isSyntheticCommitID(commitID) {
			_ = s.commitRepo.UpdateStatusByOnChainCommitID(ctx, commitID, domain.CommitStatusReconciliationRequired)
		}
		return fmt.Errorf("sovereign liquidity: depositForCommit for commit %s: %w", commitID, execErr)
	}

	// Best-effort finalize: succeeds once both sides are escrowed. A revert here is expected for
	// whichever gateway deposited first (other side pending) or if the peer already finalized —
	// either way the on-chain escrow is safe and the matched commit completes. When OUR finalize
	// is the one that lands, LogCommitFinalized gives us the minted share split so the position
	// can record this CB's exact on-chain LP shares (otherwise LPShares stays "0" and withdrawal
	// falls back to the signer's live on-chain balance).
	var myShares *big.Int
	if sharesA, sharesB, finErr := s.amm.FinalizeCommitAt(execCtx, ammAddress, escrowKey); finErr == nil {
		if isTokenA {
			myShares = sharesA
		} else {
			myShares = sharesB
		}
	}

	// Update commit status to EXECUTED.
	// Synthetic direct-deposit commit IDs (e.g. "direct-deposit-central-bank-b") are not
	// hex-encoded bytes32 values — no DB record exists for them. Skip DB operations gracefully.
	if isSyntheticCommitID(commitID) {
		return nil
	}
	if err := s.commitRepo.UpdateStatusByOnChainCommitID(ctx, commitID, domain.CommitStatusExecuted); err != nil {
		return fmt.Errorf("sovereign liquidity: update commit status for %s: %w", commitID, err)
	}

	// Create a LiquidityPosition record for the provider.
	commit, err := s.commitRepo.FindByOnChainCommitID(ctx, commitID)
	if err != nil {
		// Non-fatal — the on-chain deposit succeeded; log but continue.
		return nil
	}
	s.ensureLPPosition(ctx, commit, poolPairName, isTokenA, myShares)
	return nil
}

// isSyntheticCommitID returns true when the commitID is not a valid hex-encoded bytes32
// (e.g. "direct-deposit-*" strings used for sovereign top-up deposits).
// These commits have no corresponding DB record, so all DB operations should be skipped.
func isSyntheticCommitID(id string) bool {
	s := strings.TrimPrefix(id, "0x")
	if len(s)%2 != 0 {
		s = "0" + s
	}
	_, err := hex.DecodeString(s)
	return err != nil
}

// ensureLPPosition creates a LiquidityPosition record if one doesn't already exist for this commit.
func (s *SovereignLiquidityService) ensureLPPosition(ctx context.Context, commit *domain.PoolCommit, poolPairName string, isTokenA bool, shares *big.Int) {
	if s.lpRepo == nil {
		return
	}
	lpID := commit.CommitID + "-sovereign"
	lpShares := "0"
	if shares != nil && shares.Sign() > 0 {
		lpShares = shares.String()
	}
	now := time.Now().UTC()
	tokenA, tokenB := "0", "0"
	depositSide := domain.DepositSideA
	if isTokenA {
		tokenA = commit.Amount
	} else {
		tokenB = commit.Amount
		depositSide = domain.DepositSideB
	}
	pos := &domain.LiquidityPosition{
		LPID:              lpID,
		ProviderBankID:    commit.ProviderID,
		PoolPair:          poolPairName,
		TokenAContributed: tokenA,
		TokenBContributed: tokenB,
		LPShares:          lpShares,
		Status:            domain.LPStatusActive,
		DepositSide:       depositSide,
		AddedAt:           now,
		CommitID:          &commit.CommitID,
	}
	// Ignore duplicate key errors (idempotency).
	_ = s.lpRepo.Create(ctx, pos)
}

// deriveEscrowKey computes a deterministic, order-independent escrow key from the two matched
// commit ids. Both sovereign gateways observe the same CommitMatched payload and therefore derive
// the same key, allowing each to escrow its own side against a shared identifier.
func deriveEscrowKey(commitIDA, commitIDB string) [32]byte {
	a := strings.ToLower(strings.TrimPrefix(commitIDA, "0x"))
	b := strings.ToLower(strings.TrimPrefix(commitIDB, "0x"))
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	return crypto.Keccak256Hash([]byte(lo + "|" + hi))
}

// parseBig converts a decimal string to *big.Int, returning nil on failure.
func parseBig(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil
	}
	return n
}
