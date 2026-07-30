// SPDX-License-Identifier: Apache-2.0

// Package workers provides BesuRelayerExecutor — the production RelayerEventExecutor
// that performs real on-chain operations via go-ethereum clients.
//
// Flow:
//   - SubmitLockEvent: mints W-tCeBM on Hub for the CB signer (and optionally locks native
//     tokens on SpokeBridge first when SPOKE_BESU_RPC_URL + SPOKE_BRIDGE_ADDRESS are set).
//   - SubmitBurnEvent: burns W-tCeBM on Hub (and optionally releases native tokens on
//     SpokeBridge when spoke is configured).
//
// Hub configuration is required; spoke configuration is optional and safe to omit in
// environments where the spoke bridge lock is handled out-of-band.
package workers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

// wcebmABI is the minimal ABI for FiatCentralBankMoney / W-tCeBM tokens.
// Covers mint(), burn() (CENTRAL_BANK_ROLE), balanceOf() and approve() (ERC-20 standard).
const wcebmABI = `[
{"type":"function","name":"mint","stateMutability":"nonpayable","inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[]},
{"type":"function","name":"burn","stateMutability":"nonpayable","inputs":[{"name":"from","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[]},
{"type":"function","name":"balanceOf","stateMutability":"view","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
{"type":"function","name":"approve","stateMutability":"nonpayable","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]}
]`

// spokeBridgeRelayerABI is the minimal ABI for SpokeBridge.sol used by the executor.
const spokeBridgeRelayerABI = `[
{"type":"function","name":"lock","stateMutability":"nonpayable","inputs":[{"name":"token","type":"address"},{"name":"amount","type":"uint256"},{"name":"txId","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"release","stateMutability":"nonpayable","inputs":[{"name":"txId","type":"bytes32"}],"outputs":[]},
{"type":"function","name":"getLock","stateMutability":"view","inputs":[{"name":"txId","type":"bytes32"}],"outputs":[{"name":"sender","type":"address"},{"name":"token","type":"address"},{"name":"amount","type":"uint256"},{"name":"released","type":"bool"}]}
]`

// BesuRelayerConfig holds the on-chain parameters for BesuRelayerExecutor.
type BesuRelayerConfig struct {
	// Hub chain configuration (required).
	HubRPCURL    string
	HubChainID   int64
	HubSignerKey string // hex-encoded secp256k1 private key (SIGNER_PRIVATE_KEY)

	// Spoke chain configuration (optional).
	// When SpokeRPCURL or SpokeBridgeAddr is empty, spoke lock/release operations are skipped.
	SpokeRPCURL     string
	SpokeChainID    int64
	SpokeSignerKey  string // defaults to HubSignerKey when empty
	SpokeBridgeAddr string // SpokeBridge.sol contract address on the spoke chain

	// HubMintRecipient is the Hub address that receives W-tCeBM on lock-mint (commercial bank wallet).
	// When empty, mints to HubSigner (CB self-mint). HubSigner must hold CENTRAL_BANK_ROLE on the token.
	HubMintRecipient string

	// SkipSpokeLock skips SpokeBridge.lock when true (local dev — Hub-only mint to commercial wallet).
	SkipSpokeLock bool
}

// BesuRelayerExecutor implements RelayerEventExecutor with real Besu on-chain calls.
type BesuRelayerExecutor struct {
	db          *gorm.DB
	cfg         BesuRelayerConfig
	hubEC       *ethclient.Client
	hubSigner   *evm.Signer
	hubABI      abi.ABI
	spokeEC     *ethclient.Client // nil when spoke is not configured
	spokeSigner *evm.Signer       // nil when spoke is not configured
	spokeABI    abi.ABI

	// spokeReady reports whether spoke-side lock/release/mint operations are configured.
	spokeReady bool

	// On-chain seams for the burn flow. NewBesuRelayerExecutor wires these to the real
	// go-ethereum implementations; tests override them to exercise the R2-H-12 burn
	// idempotency contract without a live chain.
	hubBurnFn      func(ctx context.Context, token, from common.Address, amount *big.Int) (string, error)
	spokeMintFn    func(ctx context.Context, to, nativeAsset string, amount *big.Int) error
	spokeReleaseFn func(ctx context.Context, txID [32]byte) error
}

// NewBesuRelayerExecutor dials the Hub (and optionally Spoke) chain and returns a ready executor.
// The context deadline controls how long the initial RPC dial may take.
func NewBesuRelayerExecutor(ctx context.Context, db *gorm.DB, cfg BesuRelayerConfig) (*BesuRelayerExecutor, error) {
	if cfg.HubRPCURL == "" || cfg.HubSignerKey == "" {
		return nil, fmt.Errorf("BesuRelayerConfig: HubRPCURL and HubSignerKey are required")
	}

	hubEC, err := evm.Dial(ctx, cfg.HubRPCURL, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial hub RPC %s: %w", cfg.HubRPCURL, err)
	}

	hubSigner, err := evm.NewSigner(cfg.HubSignerKey, big.NewInt(cfg.HubChainID))
	if err != nil {
		hubEC.Close()
		return nil, fmt.Errorf("hub signer: %w", err)
	}

	parsedHubABI, err := evm.ParseABI(wcebmABI)
	if err != nil {
		hubEC.Close()
		return nil, fmt.Errorf("parse hub W-tCeBM ABI: %w", err)
	}

	ex := &BesuRelayerExecutor{
		db:        db,
		cfg:       cfg,
		hubEC:     hubEC,
		hubSigner: hubSigner,
		hubABI:    parsedHubABI,
	}
	ex.hubBurnFn = ex.hubBurnOnChain

	if cfg.SpokeRPCURL != "" && cfg.SpokeBridgeAddr != "" {
		spokeEC, dialErr := evm.Dial(ctx, cfg.SpokeRPCURL, 10*time.Second)
		if dialErr != nil {
			hubEC.Close()
			return nil, fmt.Errorf("dial spoke RPC %s: %w", cfg.SpokeRPCURL, dialErr)
		}

		spokeKey := cfg.SpokeSignerKey
		if spokeKey == "" {
			spokeKey = cfg.HubSignerKey
		}
		spokeSigner, signerErr := evm.NewSigner(spokeKey, big.NewInt(cfg.SpokeChainID))
		if signerErr != nil {
			hubEC.Close()
			spokeEC.Close()
			return nil, fmt.Errorf("spoke signer: %w", signerErr)
		}

		parsedSpokeABI, abiErr := evm.ParseABI(spokeBridgeRelayerABI)
		if abiErr != nil {
			hubEC.Close()
			spokeEC.Close()
			return nil, fmt.Errorf("parse SpokeBridge ABI: %w", abiErr)
		}

		ex.spokeEC = spokeEC
		ex.spokeSigner = spokeSigner
		ex.spokeABI = parsedSpokeABI
		ex.spokeReady = true
		ex.spokeMintFn = ex.spokeMint
		ex.spokeReleaseFn = ex.spokeRelease

		log.Printf("[BesuRelayerExecutor] spoke bridge configured: rpc=%s contract=%s", cfg.SpokeRPCURL, cfg.SpokeBridgeAddr)
	} else {
		log.Printf("[BesuRelayerExecutor] spoke bridge not configured — spoke lock/release will be skipped")
	}

	log.Printf("[BesuRelayerExecutor] initialized: hub_rpc=%s signer=%s", cfg.HubRPCURL, hubSigner.Address().Hex())
	return ex, nil
}

// SubmitLockEvent mints W-tCeBM on the Hub chain for the CB Hub signer.
//
// Two spoke-side paths:
//   - Commercial bank bridge-in (pos.BurnFromSpokeAddress set): burns the bank's tCeBM on Spoke
//     using CENTRAL_BANK_ROLE. The bank must hold tCeBM obtained via Reserve Tokenisation.
//     The CB MUST NOT create new tCeBM here — that would be unauthorized money creation.
//   - Sovereign CB self-service (BurnFromSpokeAddress empty): auto-funds CB signer then locks
//     via SpokeBridge. Used for CB's own sovereign liquidity positions (e.g. Phase 1 commit).
func (e *BesuRelayerExecutor) SubmitLockEvent(ctx context.Context, _ /*idempotencyKey*/, positionID string) error {
	pos, err := e.loadPosition(ctx, positionID)
	if err != nil {
		return err
	}

	amount, ok := new(big.Int).SetString(pos.MirroredAmount, 10)
	if !ok {
		return fmt.Errorf("invalid mirrored_amount %q for position %s", pos.MirroredAmount, positionID)
	}

	if bankWallet := strings.TrimSpace(pos.BurnFromSpokeAddress); bankWallet != "" {
		// ── Commercial bank bridge-in: burn the bank's pre-tokenised tCeBM ──────────────
		// The bank must have obtained tCeBM via Reserve Tokenisation (ApproveEscrow).
		// CB-A holds CENTRAL_BANK_ROLE and can burn from any address, but MUST NOT mint
		// new tCeBM here. If the burn fails (insufficient balance), the bridge-in is rejected.
		if e.spokeEC == nil {
			return fmt.Errorf("spoke chain not configured — cannot burn tCeBM for position %s", positionID)
		}
		if burnErr := e.spokeBurnFrom(ctx, pos.NativeAsset, bankWallet, amount); burnErr != nil {
			return fmt.Errorf("spoke burn-from bank (position=%s bank=%s): %w", positionID, bankWallet, burnErr)
		}
		log.Printf("[BesuRelayerExecutor] spoke burn-from ok — position=%s bank=%s token=%s amount=%s",
			positionID, bankWallet, pos.NativeAsset, amount.String())
	} else if e.spokeEC != nil && pos.NativeAsset != "" && !e.cfg.SkipSpokeLock {
		// ── Sovereign CB self-service: auto-fund signer then lock via SpokeBridge ────────
		if fundErr := e.ensureSpokeFunds(ctx, pos.NativeAsset, amount); fundErr != nil {
			return fmt.Errorf("spoke auto-fund (position=%s): %w", positionID, fundErr)
		}
		txID := deriveSpokeTxID(positionID)
		if lockErr := e.spokeLock(ctx, pos.NativeAsset, amount, txID); lockErr != nil {
			return fmt.Errorf("spoke lock (position=%s): %w", positionID, lockErr)
		}
	}

	// Hub mint: W-tCeBM.mint(recipient, amount). CB hub signer must hold CENTRAL_BANK_ROLE.
	// Priority: pos.MintToHubAddress (set for cross-currency bridge-in — the initiating
	// gateway's swap signer) > HUB_MINT_RECIPIENT env var > hub signer self.
	mintTo := e.hubSigner.Address()
	if r := strings.TrimSpace(e.cfg.HubMintRecipient); r != "" {
		mintTo = common.HexToAddress(r)
	}
	if r := strings.TrimSpace(pos.MintToHubAddress); r != "" {
		mintTo = common.HexToAddress(r)
	}
	mirroredAddr := common.HexToAddress(pos.MirroredAsset)
	if mirroredAddr == (common.Address{}) {
		return fmt.Errorf("invalid mirrored_asset %q for position %s (expected ERC-20 address)", pos.MirroredAsset, positionID)
	}
	if _, mintErr := evm.SubmitTx(ctx, e.hubEC, e.hubSigner, mirroredAddr, e.hubABI,
		"mint", mintTo, amount,
	); mintErr != nil {
		return fmt.Errorf("hub mint (token=%s amount=%s position=%s recipient=%s): %w",
			pos.MirroredAsset, pos.MirroredAmount, positionID, mintTo.Hex(), mintErr)
	}

	log.Printf("[BesuRelayerExecutor] lock-mint ok — positionID=%s token=%s amount=%s recipient=%s minter=%s",
		positionID, pos.MirroredAsset, pos.MirroredAmount, mintTo.Hex(), e.hubSigner.Address().Hex())
	return nil
}

// SubmitBurnEvent burns W-tCeBM on the Hub chain.
// If the spoke is configured, native tokens are released on SpokeBridge after burn (idempotent via txId).
func (e *BesuRelayerExecutor) SubmitBurnEvent(ctx context.Context, _ /*idempotencyKey*/, positionID string) error {
	pos, err := e.loadPosition(ctx, positionID)
	if err != nil {
		return err
	}

	amount, ok := new(big.Int).SetString(pos.MirroredAmount, 10)
	if !ok {
		return fmt.Errorf("invalid mirrored_amount %q for position %s", pos.MirroredAmount, positionID)
	}

	// Hub burn: W-tCeBM.burn(holder, amount) — holder is commercial bank wallet when configured.
	// Priority: pos.BurnFromHubAddress (set for cross-currency bridge-out by CB-B) >
	//           HUB_MINT_RECIPIENT env var > hub signer self.
	burnFrom := e.hubSigner.Address()
	if r := strings.TrimSpace(e.cfg.HubMintRecipient); r != "" {
		burnFrom = common.HexToAddress(r)
	}
	if r := strings.TrimSpace(pos.BurnFromHubAddress); r != "" {
		burnFrom = common.HexToAddress(r)
	}
	mirroredAddr := common.HexToAddress(pos.MirroredAsset)

	// Idempotency (R2-H-12): the Hub burn counts as done ONLY when its confirmed transaction
	// hash is persisted on the position. Completion is never inferred from token balance — a
	// low balance can arise from causes other than this burn, and acting on it would release
	// native value on the spoke without a confirmed Hub burn. This mirrors the bridge-out
	// replay guard keyed on swap_tx_hash (R2-CR-6): idempotency is tracked by persisted state.
	if pos.HubBurnTxHash == "" {
		txHash, burnErr := e.hubBurnFn(ctx, mirroredAddr, burnFrom, amount)
		if burnErr != nil {
			return fmt.Errorf("hub burn (token=%s amount=%s position=%s): %w",
				pos.MirroredAsset, pos.MirroredAmount, positionID, burnErr)
		}
		// evm.SubmitTx only returns nil after a successful receipt, so the burn is now
		// confirmed on-chain. Persist it BEFORE any spoke-side value movement so a retry after
		// a spoke failure resumes at the spoke step instead of re-burning.
		if perr := e.markHubBurnConfirmed(ctx, pos, txHash); perr != nil {
			return fmt.Errorf("persist hub burn confirmation (position=%s tx=%s): %w", positionID, txHash, perr)
		}
		log.Printf("[BesuRelayerExecutor] hub burn confirmed — position=%s burnFrom=%s amount=%s tx=%s",
			positionID, burnFrom.Hex(), amount.String(), txHash)
	} else {
		log.Printf("[BesuRelayerExecutor] idempotency: hub burn already confirmed for position=%s (tx=%s) — skipping burn",
			positionID, pos.HubBurnTxHash)
	}

	// The Hub burn is confirmed on-chain (this attempt or a prior one). Only now may native
	// value move on the spoke.
	//
	// The action is decided by planSpokeDelivery so the gating rule is unit-testable without a
	// live chain (develop refactor); execution goes through the spokeMintFn/spokeReleaseFn seams
	// so the R2-H-12 burn-idempotency tests can stub the chain calls. Cross-currency bridge-out
	// (BeneficiarySpokeAddress set) mints tCeBM directly — CB-B holds CENTRAL_BANK_ROLE, no prior
	// lock to unwind — and must NOT be suppressed by SkipSpokeLock (doing so burns the Hub W-token
	// yet leaves the beneficiary uncredited: silent half-settlement, forbidden by the atomicity
	// rule). Standard bridge-out (no beneficiary) uses SpokeBridge.release() and stays gated by
	// SkipSpokeLock.
	beneficiary := strings.TrimSpace(pos.BeneficiarySpokeAddress)
	switch planSpokeDelivery(e.spokeReady, e.cfg.SkipSpokeLock, beneficiary) {
	case spokeDeliveryMint:
		if mintErr := e.spokeMintFn(ctx, beneficiary, pos.NativeAsset, amount); mintErr != nil {
			return fmt.Errorf("spoke mint (position=%s beneficiary=%s): %w", positionID, beneficiary, mintErr)
		}
	case spokeDeliveryRelease:
		txID := deriveSpokeTxID(positionID)
		if releaseErr := e.spokeReleaseFn(ctx, txID); releaseErr != nil {
			return fmt.Errorf("spoke release (position=%s): %w", positionID, releaseErr)
		}
	case spokeDeliveryNone:
		// No spoke-side leg (spoke not configured, or legacy release suppressed by SkipSpokeLock).
	}

	log.Printf("[BesuRelayerExecutor] burn-unlock ok — positionID=%s token=%s amount=%s signer=%s",
		positionID, pos.MirroredAsset, pos.MirroredAmount, e.hubSigner.Address().Hex())
	return nil
}

// hubBurnOnChain burns W-tCeBM on the Hub and returns the confirmed transaction hash.
// evm.SubmitTx waits for the receipt and returns an error when the transaction reverts
// (receipt.Status == 0), so a nil error means the burn is confirmed on-chain.
func (e *BesuRelayerExecutor) hubBurnOnChain(ctx context.Context, token, from common.Address, amount *big.Int) (string, error) {
	return evm.SubmitTx(ctx, e.hubEC, e.hubSigner, token, e.hubABI, "burn", from, amount)
}

// markHubBurnConfirmed persists the confirmed Hub burn transaction hash on the position and
// advances bridge_state to BURNED. The persisted hash is the idempotency key consulted by
// SubmitBurnEvent on retries (R2-H-12).
func (e *BesuRelayerExecutor) markHubBurnConfirmed(ctx context.Context, pos *podmain.BridgedAssetPosition, txHash string) error {
	if err := e.db.WithContext(ctx).Model(&podmain.BridgedAssetPosition{}).
		Where("position_id = ?", pos.PositionID).
		Updates(map[string]interface{}{
			"hub_burn_tx_hash": txHash,
			"bridge_state":     podmain.BridgeStateBurned,
		}).Error; err != nil {
		return err
	}
	pos.HubBurnTxHash = txHash
	pos.BridgeState = podmain.BridgeStateBurned
	return nil
}

// spokeDeliveryAction is the spoke-side settlement leg chosen for a bridge-out.
type spokeDeliveryAction int

const (
	// spokeDeliveryNone performs no spoke-side leg (spoke not configured, or the legacy
	// release-from-lock path suppressed by SkipSpokeLock).
	spokeDeliveryNone spokeDeliveryAction = iota
	// spokeDeliveryMint mints native tCeBM to the beneficiary (cross-currency bridge-out).
	spokeDeliveryMint
	// spokeDeliveryRelease returns tokens locked by a prior SpokeBridge.lock() (standard bridge-out).
	spokeDeliveryRelease
)

// planSpokeDelivery decides the spoke-side settlement leg for a bridge-out.
//
// Cross-currency bridge-out (beneficiary set) always MINTS native tCeBM when the spoke is
// configured: it involves no SpokeBridge.lock() to unwind, so SkipSpokeLock — which only
// governs the SpokeBridge lock/release dance — must never suppress it. Suppressing it is what
// burns the hub W-token yet leaves the beneficiary uncredited (silent half-settlement,
// forbidden by the atomicity rule). This mirrors the commercial-bank bridge-in burn, which is
// likewise ungated. The legacy release-from-lock path (no beneficiary) stays gated by
// SkipSpokeLock.
func planSpokeDelivery(spokeConfigured, skipSpokeLock bool, beneficiary string) spokeDeliveryAction {
	if !spokeConfigured {
		return spokeDeliveryNone
	}
	if strings.TrimSpace(beneficiary) != "" {
		return spokeDeliveryMint
	}
	if skipSpokeLock {
		return spokeDeliveryNone
	}
	return spokeDeliveryRelease
}

// spokeBurnFrom calls tCeBM.burn(from, amount) on the Spoke chain using the CB signer.
// Used for commercial bank cross-currency bridge-in: burns the bank's tCeBM that was
// obtained via Reserve Tokenisation (ApproveEscrow). Requires CENTRAL_BANK_ROLE.
// Returns an error if the bank has insufficient tCeBM — the bridge-in is then rejected.
func (e *BesuRelayerExecutor) spokeBurnFrom(ctx context.Context, nativeAsset, fromAddress string, amount *big.Int) error {
	tokenAddr := common.HexToAddress(nativeAsset)
	fromAddr := common.HexToAddress(fromAddress)
	signer := e.spokeSigner
	if signer == nil {
		signer = e.hubSigner
	}
	_, err := evm.SubmitTx(ctx, e.spokeEC, signer, tokenAddr, e.hubABI,
		"burn", fromAddr, amount,
	)
	if err != nil {
		return fmt.Errorf("tCeBM.burn(from=%s token=%s amount=%s): %w", fromAddress, nativeAsset, amount.String(), err)
	}
	return nil
}

// spokeMint calls tCeBM.mint(to, amount) on the Spoke chain.
// Used for cross-currency bridge-out where there is no prior SpokeBridge.lock() on Spoke-B.
// Requires CENTRAL_BANK_ROLE on the spoke fiat token (pos.NativeAsset).
func (e *BesuRelayerExecutor) spokeMint(ctx context.Context, to, nativeAssetAddr string, amount *big.Int) error {
	toAddr := common.HexToAddress(to)
	tokenAddr := common.HexToAddress(nativeAssetAddr)
	signer := e.spokeSigner
	if signer == nil {
		signer = e.hubSigner
	}
	_, err := evm.SubmitTx(ctx, e.spokeEC, signer, tokenAddr, e.hubABI,
		"mint", toAddr, amount,
	)
	if err != nil {
		return fmt.Errorf("tCeBM.mint(to=%s token=%s amount=%s): %w", to, nativeAssetAddr, amount.String(), err)
	}
	log.Printf("[BesuRelayerExecutor] spoke mint ok — to=%s token=%s amount=%s", to, nativeAssetAddr, amount.String())
	return nil
}

// ensureSpokeFunds guarantees the spoke signer holds at least `amount` of the native
// tCeBM token and has approved the SpokeBridge to spend it, so SpokeBridge.lock's
// safeTransferFrom(signer → bridge) cannot revert for insufficient balance/allowance.
//
// Both steps are idempotent: minting is skipped when the signer balance already covers
// the amount, and approval is set to the exact amount (SpokeBridge.lock pulls it in full).
// The signer must hold CENTRAL_BANK_ROLE on the native token (true for the CB hub/spoke
// signer in Scenario B).
func (e *BesuRelayerExecutor) ensureSpokeFunds(ctx context.Context, nativeAsset string, amount *big.Int) error {
	tokenAddr := common.HexToAddress(nativeAsset)
	if tokenAddr == (common.Address{}) {
		return fmt.Errorf("invalid native_asset %q (expected ERC-20 address)", nativeAsset)
	}
	signer := e.spokeSigner

	// Mint up to `amount` if the signer is short on balance.
	var bal big.Int
	if balErr := evm.Call(ctx, e.spokeEC, tokenAddr, e.hubABI, "balanceOf",
		[]interface{}{signer.Address()}, &bal,
	); balErr != nil {
		return fmt.Errorf("read native balance (token=%s): %w", nativeAsset, balErr)
	}
	if bal.Cmp(amount) < 0 {
		if _, mintErr := evm.SubmitTx(ctx, e.spokeEC, signer, tokenAddr, e.hubABI,
			"mint", signer.Address(), amount,
		); mintErr != nil {
			return fmt.Errorf("native mint (token=%s to=%s amount=%s): %w",
				nativeAsset, signer.Address().Hex(), amount.String(), mintErr)
		}
		log.Printf("[BesuRelayerExecutor] auto-mint ok — token=%s to=%s amount=%s",
			nativeAsset, signer.Address().Hex(), amount.String())
	}

	// Approve the SpokeBridge to pull `amount` for the upcoming lock.
	bridgeAddr := common.HexToAddress(e.cfg.SpokeBridgeAddr)
	if _, approveErr := evm.SubmitTx(ctx, e.spokeEC, signer, tokenAddr, e.hubABI,
		"approve", bridgeAddr, amount,
	); approveErr != nil {
		return fmt.Errorf("native approve (token=%s spender=%s amount=%s): %w",
			nativeAsset, bridgeAddr.Hex(), amount.String(), approveErr)
	}
	return nil
}

// spokeLock calls SpokeBridge.lock() with idempotency: checks getLock() before transacting.
func (e *BesuRelayerExecutor) spokeLock(ctx context.Context, nativeAsset string, amount *big.Int, txID [32]byte) error {
	bridgeAddr := common.HexToAddress(e.cfg.SpokeBridgeAddr)

	// Idempotency: skip if already locked and not yet released.
	var senderAddr, tokenAddr common.Address
	var lockedAmt big.Int
	var released bool
	if callErr := evm.Call(ctx, e.spokeEC, bridgeAddr, e.spokeABI, "getLock",
		[]interface{}{txID},
		&senderAddr, &tokenAddr, &lockedAmt, &released,
	); callErr == nil && lockedAmt.Sign() > 0 && !released {
		log.Printf("[BesuRelayerExecutor] spoke lock already exists for txId=%x — skipping", txID)
		return nil
	}

	_, err := evm.SubmitTx(ctx, e.spokeEC, e.spokeSigner, bridgeAddr, e.spokeABI,
		"lock", common.HexToAddress(nativeAsset), amount, txID,
	)
	return err
}

// spokeRelease calls SpokeBridge.release() with idempotency: skips if already released.
func (e *BesuRelayerExecutor) spokeRelease(ctx context.Context, txID [32]byte) error {
	bridgeAddr := common.HexToAddress(e.cfg.SpokeBridgeAddr)

	// Idempotency: skip if already released.
	var senderAddr, tokenAddr common.Address
	var lockedAmt big.Int
	var released bool
	if callErr := evm.Call(ctx, e.spokeEC, bridgeAddr, e.spokeABI, "getLock",
		[]interface{}{txID},
		&senderAddr, &tokenAddr, &lockedAmt, &released,
	); callErr == nil && released {
		log.Printf("[BesuRelayerExecutor] spoke lock already released for txId=%x — skipping", txID)
		return nil
	}

	_, err := evm.SubmitTx(ctx, e.spokeEC, e.spokeSigner, bridgeAddr, e.spokeABI,
		"release", txID,
	)
	return err
}

func (e *BesuRelayerExecutor) loadPosition(ctx context.Context, positionID string) (*podmain.BridgedAssetPosition, error) {
	var pos podmain.BridgedAssetPosition
	if err := e.db.WithContext(ctx).Where("position_id = ?", positionID).First(&pos).Error; err != nil {
		return nil, fmt.Errorf("load position %s: %w", positionID, err)
	}
	return &pos, nil
}

// deriveSpokeTxID creates a deterministic bytes32 txId from a positionID.
// The same positionID always produces the same txId across retries, ensuring idempotency.
func deriveSpokeTxID(positionID string) [32]byte {
	return sha256.Sum256([]byte("spoke-lock:" + positionID))
}
