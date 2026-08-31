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

// Funding a sovereign lock used to be decided by reading the signer's balance: mint only when
// `balanceOf(signer) < amount`. On a shared signer that meant a position whose predecessor had
// left tokens behind locked those instead of minting its own — the CB's tCeBM supply stopped
// corresponding to the positions backing it, and a stranded balance was silently consumed by the
// next lock. The decision is now per position and recorded, like the Hub burn (R2-H-12).

type fundHooks struct {
	fund         func(ctx context.Context, nativeAsset string, amount *big.Int, recordIntent func(txHash string)) (string, error)
	approve      func(ctx context.Context, nativeAsset string, amount *big.Int) error
	spokeTxMined func(ctx context.Context, txHash string) (mined bool, success bool, err error)
}

func newFundTestExecutor(t *testing.T, db *gorm.DB, h fundHooks) *BesuRelayerExecutor {
	t.Helper()
	signer, err := evm.NewSigner(anvilTestKey, big.NewInt(1))
	if err != nil {
		t.Fatalf("build test signer: %v", err)
	}
	return &BesuRelayerExecutor{
		db:             db,
		hubSigner:      signer,
		spokeSigner:    signer,
		spokeReady:     true,
		spokeFundFn:    h.fund,
		spokeApproveFn: h.approve,
		spokeTxMinedFn: h.spokeTxMined,
	}
}

func seedFundPosition(t *testing.T, db *gorm.DB, positionID, fundTxHash string) *podmain.BridgedAssetPosition {
	t.Helper()
	pos := &podmain.BridgedAssetPosition{
		PositionID:      positionID,
		OwnerBankID:     "central-bank-a",
		SpokeNetwork:    "spoke",
		NativeAsset:     "0x0000000000000000000000000000000000000abc",
		MirroredAsset:   "0x0000000000000000000000000000000000000def",
		MirroredAmount:  "100",
		BridgeState:     podmain.BridgeStateLocking,
		SpokeFundTxHash: fundTxHash,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return pos
}

func reloadFundHash(t *testing.T, db *gorm.DB, positionID string) string {
	t.Helper()
	var got podmain.BridgedAssetPosition
	if err := db.Where("position_id = ?", positionID).First(&got).Error; err != nil {
		t.Fatalf("reload position: %v", err)
	}
	return got.SpokeFundTxHash
}

// An unfunded position mints exactly its own amount and records the hash — no balance is read.
func TestEnsureSpokeFunds_MintsForTheUnfundedPositionAndRecordsIt(t *testing.T) {
	db := newTestDB(t)
	pos := seedFundPosition(t, db, "pos-fund-1", "")

	var mintedAmount, approvedAmount *big.Int
	ex := newFundTestExecutor(t, db, fundHooks{
		fund: func(_ context.Context, _ string, amount *big.Int, recordIntent func(string)) (string, error) {
			mintedAmount = amount
			recordIntent("0xfundtx") // the real implementation records on broadcast
			return "0xfundtx", nil
		},
		approve: func(_ context.Context, _ string, amount *big.Int) error {
			approvedAmount = amount
			return nil
		},
	})

	if err := ex.ensureSpokeFunds(context.Background(), pos, big.NewInt(100)); err != nil {
		t.Fatalf("ensureSpokeFunds: %v", err)
	}
	if mintedAmount == nil || mintedAmount.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("expected a mint of exactly the position's amount, got %v", mintedAmount)
	}
	if approvedAmount == nil || approvedAmount.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("expected the bridge to be approved for the same amount, got %v", approvedAmount)
	}
	if got := reloadFundHash(t, db, "pos-fund-1"); got != "0xfundtx" {
		t.Fatalf("funding hash not persisted: %q", got)
	}
}

// A position already funded must not mint again, however much or little sits on the signer.
func TestEnsureSpokeFunds_SkipsMintWhenAlreadyFunded(t *testing.T) {
	db := newTestDB(t)
	pos := seedFundPosition(t, db, "pos-fund-2", "0xalready")

	approved := false
	ex := newFundTestExecutor(t, db, fundHooks{
		fund: func(context.Context, string, *big.Int, func(string)) (string, error) {
			t.Fatalf("a funded position must not mint again")
			return "", nil
		},
		approve: func(context.Context, string, *big.Int) error { approved = true; return nil },
		spokeTxMined: func(_ context.Context, txHash string) (bool, bool, error) {
			if txHash != "0xalready" {
				t.Fatalf("reconciled the wrong hash: %s", txHash)
			}
			return true, true, nil
		},
	})

	if err := ex.ensureSpokeFunds(context.Background(), pos, big.NewInt(100)); err != nil {
		t.Fatalf("ensureSpokeFunds: %v", err)
	}
	if !approved {
		t.Fatalf("the approval must still run so the lock can pull the tokens")
	}
}

// A recorded mint that never mined fails closed. Minting again to cover it would double the
// CB's issuance for one position — the lock it precedes would fail anyway.
func TestEnsureSpokeFunds_UnminedFundingAwaitsRatherThanRemints(t *testing.T) {
	db := newTestDB(t)
	pos := seedFundPosition(t, db, "pos-fund-3", "0xpending")

	ex := newFundTestExecutor(t, db, fundHooks{
		fund: func(context.Context, string, *big.Int, func(string)) (string, error) {
			t.Fatalf("must not re-mint while the recorded funding is unconfirmed")
			return "", nil
		},
		approve: func(context.Context, string, *big.Int) error {
			t.Fatalf("must not approve before the funding is confirmed")
			return nil
		},
		spokeTxMined: func(context.Context, string) (bool, bool, error) { return false, false, nil },
	})

	err := ex.ensureSpokeFunds(context.Background(), pos, big.NewInt(100))
	if err == nil {
		t.Fatalf("expected an error while the funding mint is unmined")
	}
}

// A reverted funding mint needs operator review, not an automatic second mint.
func TestEnsureSpokeFunds_RevertedFundingRequiresReconciliation(t *testing.T) {
	db := newTestDB(t)
	pos := seedFundPosition(t, db, "pos-fund-4", "0xreverted")

	ex := newFundTestExecutor(t, db, fundHooks{
		fund: func(context.Context, string, *big.Int, func(string)) (string, error) {
			t.Fatalf("must not re-mint after a reverted funding")
			return "", nil
		},
		approve: func(context.Context, string, *big.Int) error {
			t.Fatalf("must not approve after a reverted funding")
			return nil
		},
		spokeTxMined: func(context.Context, string) (bool, bool, error) { return true, false, nil },
	})

	if err := ex.ensureSpokeFunds(context.Background(), pos, big.NewInt(100)); err == nil {
		t.Fatalf("expected a reverted funding mint to require reconciliation")
	}
}

// The intent is recorded on broadcast, so a crash before the receipt still leaves the hash to
// reconcile by — the window the balance heuristic used to paper over by minting again.
func TestEnsureSpokeFunds_RecordsIntentBeforeConfirmation(t *testing.T) {
	db := newTestDB(t)
	pos := seedFundPosition(t, db, "pos-fund-5", "")

	ex := newFundTestExecutor(t, db, fundHooks{
		fund: func(_ context.Context, _ string, _ *big.Int, recordIntent func(string)) (string, error) {
			recordIntent("0xbroadcast")
			return "", errors.New("wait mined: context deadline exceeded")
		},
		approve: func(context.Context, string, *big.Int) error {
			t.Fatalf("approval must not run when funding did not confirm")
			return nil
		},
	})

	if err := ex.ensureSpokeFunds(context.Background(), pos, big.NewInt(100)); err == nil {
		t.Fatalf("expected the interrupted funding to surface as an error")
	}
	if got := reloadFundHash(t, db, "pos-fund-5"); got != "0xbroadcast" {
		t.Fatalf("broadcast hash must be persisted for reconciliation, got %q", got)
	}
}

// An unusable native asset is refused before anything is minted.
func TestEnsureSpokeFunds_RejectsInvalidNativeAsset(t *testing.T) {
	db := newTestDB(t)
	pos := seedFundPosition(t, db, "pos-fund-6", "")
	pos.NativeAsset = common.Address{}.Hex()

	ex := newFundTestExecutor(t, db, fundHooks{
		fund: func(context.Context, string, *big.Int, func(string)) (string, error) {
			t.Fatalf("must not mint for an unresolved native asset")
			return "", nil
		},
	})

	if err := ex.ensureSpokeFunds(context.Background(), pos, big.NewInt(100)); err == nil {
		t.Fatalf("expected the zero-address native asset to be refused")
	}
}
