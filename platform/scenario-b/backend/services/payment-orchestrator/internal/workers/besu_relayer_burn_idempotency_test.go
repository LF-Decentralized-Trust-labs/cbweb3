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

// burnHooks captures the injectable on-chain seams for SubmitBurnEvent.
type burnHooks struct {
	hubBurn      func(ctx context.Context, token, from common.Address, amount *big.Int) (string, error)
	spokeMint    func(ctx context.Context, to, nativeAsset string, amount *big.Int) error
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
		spokeMintFn:    h.spokeMint,
		spokeReleaseFn: h.spokeRelease,
	}
}

func seedBurnPosition(t *testing.T, db *gorm.DB, positionID, burnTxHash string) *podmain.BridgedAssetPosition {
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
	pos := seedBurnPosition(t, db, "pos-burn-unconfirmed", "")

	burnCalls, mintCalls := 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		// Burn cannot be confirmed — simulates insufficient balance so the tx reverts.
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int) (string, error) {
			burnCalls++
			return "", errors.New("execution reverted: burn amount exceeds balance")
		},
		spokeMint: func(context.Context, string, string, *big.Int) error {
			mintCalls++
			return nil
		},
		spokeRelease: func(context.Context, [32]byte) error {
			t.Fatalf("spoke release must not run for a beneficiary bridge-out")
			return nil
		},
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
// then releases native value on the spoke.
func TestSubmitBurnEvent_ConfirmedBurnPersistsAndMints(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-burn-happy", "")

	burnCalls, mintCalls := 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int) (string, error) {
			burnCalls++
			return "0xburnhash", nil
		},
		spokeMint: func(context.Context, string, string, *big.Int) error {
			mintCalls++
			return nil
		},
		spokeRelease: func(context.Context, [32]byte) error {
			t.Fatalf("spoke release must not run for a beneficiary bridge-out")
			return nil
		},
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
	if got.BridgeState != podmain.BridgeStateBurned {
		t.Errorf("expected bridge_state BURNED, got %s", got.BridgeState)
	}
}

// Idempotent retry: a position whose Hub burn was already confirmed (persisted hash) skips
// the burn and resumes at the spoke step — exactly once, no double burn.
func TestSubmitBurnEvent_SkipsBurnWhenAlreadyConfirmed(t *testing.T) {
	db := newTestDB(t)
	pos := seedBurnPosition(t, db, "pos-burn-replay", "0xpriorburn")

	burnCalls, mintCalls := 0, 0
	ex := newBurnTestExecutor(t, db, burnHooks{
		hubBurn: func(context.Context, common.Address, common.Address, *big.Int) (string, error) {
			burnCalls++
			return "0xshouldnothappen", nil
		},
		spokeMint: func(context.Context, string, string, *big.Int) error {
			mintCalls++
			return nil
		},
		spokeRelease: func(context.Context, [32]byte) error {
			t.Fatalf("spoke release must not run for a beneficiary bridge-out")
			return nil
		},
	})

	if err := ex.SubmitBurnEvent(context.Background(), "", pos.PositionID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if burnCalls != 0 {
		t.Errorf("hub burn MUST be skipped when already confirmed; got %d calls", burnCalls)
	}
	if mintCalls != 1 {
		t.Errorf("expected spoke mint to resume once, got %d calls", mintCalls)
	}
	if got := loadPos(t, db, pos.PositionID); got.HubBurnTxHash != "0xpriorburn" {
		t.Errorf("persisted burn hash must be unchanged, got %q", got.HubBurnTxHash)
	}
}
