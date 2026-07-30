// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"math/big"
	"testing"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

// anvilTestKey is a throwaway secp256k1 key (well-known dev key). It never touches a chain
// in these tests — NewSigner only parses it so the executor can derive a burnFrom default.
const anvilTestKey = "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

// burnHooks captures the injectable on-chain seams for SubmitBurnEvent. The burn/mint seams
// receive a recordIntent callback that the real implementation fires on broadcast (before the
// receipt wait); tests decide whether to invoke it to simulate the pre-confirmation window.
type burnHooks struct {
	hubBurn      func(ctx context.Context, token, from common.Address, amount *big.Int, recordIntent func(txHash string)) (string, error)
	hubTxMined   func(ctx context.Context, txHash string) (mined bool, success bool, err error)
	spokeMint    func(ctx context.Context, to, nativeAsset string, amount *big.Int, recordIntent func(txHash string)) (string, error)
	spokeTxMined func(ctx context.Context, txHash string) (mined bool, success bool, err error)
	spokeRelease func(ctx context.Context, txID [32]byte) error
}

func newBurnTestExecutor(t *testing.T, db *gorm.DB, h burnHooks) *BesuRelayerExecutor {
	t.Helper()
	signer, err := evm.NewSigner(anvilTestKey, big.NewInt(1))
	if err != nil {
		t.Fatalf("build test signer: %v", err)
	}
	return &BesuRelayerExecutor{
		db:             db,
		hubSigner:      signer,
		spokeReady:     true,
		hubBurnFn:      h.hubBurn,
		hubTxMinedFn:   h.hubTxMined,
		spokeMintFn:    h.spokeMint,
		spokeTxMinedFn: h.spokeTxMined,
		spokeReleaseFn: h.spokeRelease,
	}
}

// failRelease is a spokeRelease hook that fails the test if invoked — bridge-outs with a
// beneficiary route to spoke mint, never release.
func failRelease(t *testing.T) func(context.Context, [32]byte) error {
	return func(context.Context, [32]byte) error {
		t.Helper()
		t.Fatalf("spoke release must not run for a beneficiary bridge-out")
		return nil
	}
}

func seedBurnPosition(t *testing.T, db *gorm.DB, positionID, burnTxHash, spokeMintTxHash string) *podmain.BridgedAssetPosition {
	t.Helper()
	pos := &podmain.BridgedAssetPosition{
		PositionID:              positionID,
		OwnerBankID:             "bank-a",
		SpokeNetwork:            "spoke",
		NativeAsset:             "0x0000000000000000000000000000000000000abc",
		MirroredAsset:           "0x0000000000000000000000000000000000000def",
		MirroredAmount:          "100",
		BridgeState:             podmain.BridgeStateBurning,
		BurnFromHubAddress:      "0x0000000000000000000000000000000000000111",
		BeneficiarySpokeAddress: "0x0000000000000000000000000000000000000222", // routes to spoke mint
		HubBurnTxHash:           burnTxHash,
		SpokeMintTxHash:         spokeMintTxHash,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return pos
}

// R2-H-12: when the Hub burn does not confirm on-chain, the spoke mint MUST NOT run and no
// burn hash may be persisted. Previously the executor inferred "already burned" from a low
// token balance and released native value on the spoke without a confirmed Hub burn.
func TestSubmitBurnEvent_NoSpokeMintWithoutConfirmedBurn(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-burn-unconfirmed", "", "")

	burnCalls, mintCalls := 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		// Burn fails before broadcast (e.g. gas estimation / send error): recordIntent is never
		// invoked, so nothing is persisted and the position can be retried cleanly.
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int, func(string)) (string, error) {
			burnCalls++
			return "", errors.New("send tx: insufficient funds for gas")
		},
		spokeMint: func(context.Context, string, string, *big.Int, func(string)) (string, error) {
			mintCalls++
			return "", nil
		},
		spokeRelease: failRelease(t),
	})

	err := ex.SubmitBurnEvent(context.Background(), "", pos.PositionID)
	if err == nil {
		t.Fatal("expected an error when the Hub burn does not confirm")
	}
	if burnCalls != 1 {
		t.Errorf("expected exactly 1 hub burn attempt, got %d", burnCalls)
	}
	if mintCalls != 0 {
		t.Errorf("spoke mint MUST NOT run without a confirmed burn; got %d calls", mintCalls)
	}
	got := loadPos(t, db, pos.PositionID)
	if got.HubBurnTxHash != "" {
		t.Errorf("no burn hash should be persisted on failure; got %q", got.HubBurnTxHash)
	}
	if got.BridgeState == podmain.BridgeStateBurned {
		t.Error("bridge_state must not advance to BURNED when the burn did not confirm")
	}
}

// Happy path: a confirmed Hub burn persists its tx hash, advances state to BURNED, and only
// then mints on the spoke — persisting the spoke mint hash too.
func TestSubmitBurnEvent_ConfirmedBurnPersistsAndMints(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-burn-happy", "", "")

	burnCalls, mintCalls := 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(_ context.Context, _, _ common.Address, _ *big.Int, recordIntent func(string)) (string, error) {
			burnCalls++
			recordIntent("0xburnhash") // real impl records intent on broadcast, before confirmation
			return "0xburnhash", nil
		},
		spokeMint: func(_ context.Context, _, _ string, _ *big.Int, recordIntent func(string)) (string, error) {
			mintCalls++
			recordIntent("0xminthash")
			return "0xminthash", nil
		},
		spokeRelease: failRelease(t),
	})

	if err := ex.SubmitBurnEvent(context.Background(), "", pos.PositionID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if burnCalls != 1 {
		t.Errorf("expected 1 hub burn, got %d", burnCalls)
	}
	if mintCalls != 1 {
		t.Errorf("expected 1 spoke mint after confirmed burn, got %d", mintCalls)
	}
	got := loadPos(t, db, pos.PositionID)
	if got.HubBurnTxHash != "0xburnhash" {
		t.Errorf("expected persisted burn hash 0xburnhash, got %q", got.HubBurnTxHash)
	}
	if got.SpokeMintTxHash != "0xminthash" {
		t.Errorf("expected persisted spoke mint hash 0xminthash, got %q", got.SpokeMintTxHash)
	}
	if got.BridgeState != podmain.BridgeStateBurned {
		t.Errorf("expected bridge_state BURNED, got %s", got.BridgeState)
	}
}

// Idempotent retry: a position whose Hub burn was already broadcast (persisted hash) reconciles
// the tx by receipt, skips the burn, and resumes at the spoke mint — exactly once, no double burn.
func TestSubmitBurnEvent_SkipsBurnWhenAlreadyConfirmed(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-burn-replay", "0xpriorburn", "")

	burnCalls, mintCalls, reconciled := 0, 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int, func(string)) (string, error) {
			burnCalls++
			return "0xshouldnothappen", nil
		},
		hubTxMined: func(_ context.Context, txHash string) (bool, bool, error) {
			reconciled++
			if txHash != "0xpriorburn" {
				t.Errorf("reconcile should query the persisted hash, got %q", txHash)
			}
			return true, true, nil // mined and successful
		},
		spokeMint: func(_ context.Context, _, _ string, _ *big.Int, recordIntent func(string)) (string, error) {
			mintCalls++
			recordIntent("0xminthash")
			return "0xminthash", nil
		},
		spokeRelease: failRelease(t),
	})

	if err := ex.SubmitBurnEvent(context.Background(), "", pos.PositionID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if burnCalls != 0 {
		t.Errorf("hub burn MUST be skipped when already broadcast; got %d calls", burnCalls)
	}
	if reconciled != 1 {
		t.Errorf("expected exactly 1 reconcile of the prior burn, got %d", reconciled)
	}
	if mintCalls != 1 {
		t.Errorf("expected spoke mint to resume once, got %d calls", mintCalls)
	}
	got := loadPos(t, db, pos.PositionID)
	if got.HubBurnTxHash != "0xpriorburn" {
		t.Errorf("persisted burn hash must be unchanged, got %q", got.HubBurnTxHash)
	}
	if got.BridgeState != podmain.BridgeStateBurned {
		t.Errorf("reconciled burn must advance state to BURNED, got %s", got.BridgeState)
	}
}

// R2-H-12 #1: a burn that reverts *after* broadcast records its intent hash, so the retry
// reconciles it as reverted and fails closed — it never re-burns and never mints.
func TestSubmitBurnEvent_RevertedBurnRecordsIntentAndFailsClosedOnRetry(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-burn-revert", "", "")

	// First attempt: broadcast records the intent, then the receipt comes back reverted.
	firstBurn := 0
	ex1 := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(_ context.Context, _, _ common.Address, _ *big.Int, recordIntent func(string)) (string, error) {
			firstBurn++
			recordIntent("0xrevert")
			return "", errors.New("wait mined: transaction reverted on-chain (tx=0xrevert)")
		},
		spokeMint: func(context.Context, string, string, *big.Int, func(string)) (string, error) {
			t.Fatalf("spoke mint must not run when the burn reverted")
			return "", nil
		},
		spokeRelease: failRelease(t),
	})
	if err := ex1.SubmitBurnEvent(context.Background(), "", pos.PositionID); err == nil {
		t.Fatal("expected an error on a reverted burn")
	}
	if firstBurn != 1 {
		t.Fatalf("expected 1 burn attempt, got %d", firstBurn)
	}
	if got := loadPos(t, db, pos.PositionID); got.HubBurnTxHash != "0xrevert" {
		t.Fatalf("intent hash must be persisted for reconciliation, got %q", got.HubBurnTxHash)
	}

	// Retry: reconcile finds the tx mined-but-reverted → fail closed, no re-burn, no mint.
	secondBurn, mintCalls := 0, 0
	ex2 := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int, func(string)) (string, error) {
			secondBurn++
			return "0xshouldnothappen", nil
		},
		hubTxMined: func(context.Context, string) (bool, bool, error) {
			return true, false, nil // mined, reverted
		},
		spokeMint: func(context.Context, string, string, *big.Int, func(string)) (string, error) {
			mintCalls++
			return "", nil
		},
		spokeRelease: failRelease(t),
	})
	err := ex2.SubmitBurnEvent(context.Background(), "", pos.PositionID)
	if err == nil {
		t.Fatal("expected reconciliation-required error for a reverted burn")
	}
	if secondBurn != 0 {
		t.Errorf("MUST NOT re-burn a reverted intent; got %d calls", secondBurn)
	}
	if mintCalls != 0 {
		t.Errorf("MUST NOT mint after a reverted burn; got %d calls", mintCalls)
	}
	if got := loadPos(t, db, pos.PositionID); got.BridgeState == podmain.BridgeStateBurned {
		t.Error("state must not advance to BURNED for a reverted burn")
	}
}

// R2-H-12 #1: a burn broadcast whose receipt never arrived (dropped/pending) reconciles as
// not-yet-mined → the retry waits (returns an error to re-queue) rather than re-burning.
func TestSubmitBurnEvent_UnminedBurnAwaitsRatherThanReburns(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-burn-pending", "0xpending", "")

	burnCalls, mintCalls := 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int, func(string)) (string, error) {
			burnCalls++
			return "0xshouldnothappen", nil
		},
		hubTxMined: func(context.Context, string) (bool, bool, error) {
			return false, false, nil // not mined yet
		},
		spokeMint: func(context.Context, string, string, *big.Int, func(string)) (string, error) {
			mintCalls++
			return "", nil
		},
		spokeRelease: failRelease(t),
	})
	if err := ex.SubmitBurnEvent(context.Background(), "", pos.PositionID); err == nil {
		t.Fatal("expected an await error while the burn tx is unmined")
	}
	if burnCalls != 0 {
		t.Errorf("MUST NOT re-burn while the prior tx is pending; got %d calls", burnCalls)
	}
	if mintCalls != 0 {
		t.Errorf("MUST NOT mint before the burn is confirmed; got %d calls", mintCalls)
	}
}

// R2-H-12 #2: once the spoke mint is confirmed (persisted hash), a retry reconciles it and skips
// the mint — no unbacked double issuance against a single Hub burn.
func TestSubmitBurnEvent_SkipsSpokeMintWhenAlreadyMinted(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-mint-replay", "0xpriorburn", "0xpriormint")

	burnCalls, mintCalls, mintReconciled := 0, 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int, func(string)) (string, error) {
			burnCalls++
			return "", nil
		},
		hubTxMined: func(context.Context, string) (bool, bool, error) {
			return true, true, nil
		},
		spokeMint: func(context.Context, string, string, *big.Int, func(string)) (string, error) {
			mintCalls++
			return "", nil
		},
		spokeTxMined: func(_ context.Context, txHash string) (bool, bool, error) {
			mintReconciled++
			if txHash != "0xpriormint" {
				t.Errorf("mint reconcile should query the persisted mint hash, got %q", txHash)
			}
			return true, true, nil
		},
		spokeRelease: failRelease(t),
	})
	if err := ex.SubmitBurnEvent(context.Background(), "", pos.PositionID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if burnCalls != 0 {
		t.Errorf("hub burn must be skipped; got %d calls", burnCalls)
	}
	if mintCalls != 0 {
		t.Errorf("spoke mint MUST be skipped when already confirmed; got %d calls", mintCalls)
	}
	if mintReconciled != 1 {
		t.Errorf("expected exactly 1 spoke mint reconcile, got %d", mintReconciled)
	}
}
