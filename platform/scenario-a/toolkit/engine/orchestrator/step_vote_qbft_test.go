// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/hex"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
)

// testEnodeAndAddr generates a node key and returns its enode pubkey hex (128 chars)
// and the derived QBFT validator address.
func testEnodeAndAddr(t *testing.T) (enode, addr string) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pub := crypto.FromECDSAPub(&key.PublicKey) // 65 bytes, 0x04-prefixed
	pubHex := hex.EncodeToString(pub[1:])      // strip 0x04 → 128 hex chars
	enode = "enode://" + pubHex + "@10.0.0.5:30303"
	addr = crypto.PubkeyToAddress(key.PublicKey).Hex()
	return
}

func TestVoteQBFTStep_QuorumReached(t *testing.T) {
	enode, joinerAddr := testEnodeAndAddr(t)
	var activated int64

	srv := rpcServer(map[string]func([]any) (any, error){
		"admin_nodeInfo": func([]any) (any, error) {
			return map[string]any{"enode": enode}, nil
		},
		"qbft_proposeValidatorVote": func([]any) (any, error) {
			return true, nil
		},
		"qbft_getValidatorsByBlockNumber": func([]any) (any, error) {
			// After the vote, report the joiner as a validator.
			if atomic.AddInt64(&activated, 1) >= 1 {
				return []string{joinerAddr}, nil
			}
			return []string{}, nil
		},
	})
	defer srv.Close()

	validators := []bundle.ValidatorSpec{{Address: "0xCB", RPCURL: srv.URL}}
	step := newVoteQBFTStep("spoke-test", srv.URL, validators, 5*time.Second, 10*time.Millisecond, nil)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestVoteQBFTStep_QuorumNotReached(t *testing.T) {
	enode, _ := testEnodeAndAddr(t)
	srv := rpcServer(map[string]func([]any) (any, error){
		"admin_nodeInfo": func([]any) (any, error) {
			return map[string]any{"enode": enode}, nil
		},
		"qbft_proposeValidatorVote": func([]any) (any, error) {
			return false, nil // vote rejected
		},
	})
	defer srv.Close()

	validators := []bundle.ValidatorSpec{{Address: "0xCB", RPCURL: srv.URL}}
	step := newVoteQBFTStep("spoke-test", srv.URL, validators, 500*time.Millisecond, 10*time.Millisecond, nil)
	if err := step.Run(context.Background()); err == nil {
		t.Fatal("Run should fail when quorum is not reached")
	}
}

func TestVoteQBFTStep_NoValidators(t *testing.T) {
	enode, _ := testEnodeAndAddr(t)
	srv := rpcServer(map[string]func([]any) (any, error){
		"admin_nodeInfo": func([]any) (any, error) { return map[string]any{"enode": enode}, nil },
	})
	defer srv.Close()

	step := newVoteQBFTStep("spoke-test", srv.URL, nil, 500*time.Millisecond, 10*time.Millisecond, nil)
	if err := step.Run(context.Background()); err == nil {
		t.Fatal("Run should fail with no validators in bundle")
	}
}
