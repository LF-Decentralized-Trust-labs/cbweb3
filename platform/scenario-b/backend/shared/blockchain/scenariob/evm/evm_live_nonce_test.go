// SPDX-License-Identifier: Apache-2.0

package evm

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// --- live probe: does the recovery actually recover on a real Besu? ---
//
// The unit tests pin the classification and the counter's behaviour, but two facts they cannot reach
// are what the fix rests on, and both are properties of the node, not of this package:
//
//  1. Besu QUEUES a future-nonce transaction instead of rejecting it — so the broadcast succeeds and
//     the receipt wait is the only place the drift can ever be noticed.
//  2. PendingNonceAt then returns the account's REAL nonce, not the queued transaction's + 1. If it
//     returned the drifted value, MarkNonceStale would re-read the drift and the fix would be a
//     silent no-op that still passes every unit test in this file.
//
// It also exercises the one piece of wiring a fake cannot: WaitForReceipt takes a concrete
// *ethclient.Client, so only a real node proves submitRawInternal invalidates the counter.
//
// Skips with a warning unless EVM_LIVE_RPC points at a node whose account below is funded. A Besu
// dev node is enough:
//
//	docker run -d --name nonce-probe-besu -p 8545:8545 hyperledger/besu:25.8.0 \
//	  --network=dev --miner-enabled --miner-coinbase=0xfe3b557e8fb62b89f4916b721be55ceb828dbd73 \
//	  --rpc-http-enabled --rpc-http-api=ETH,NET,WEB3 --rpc-http-host=0.0.0.0 \
//	  --host-allowlist="*" --min-gas-price=0
//	EVM_LIVE_RPC=http://127.0.0.1:8545 go test ./scenariob/evm/ -run TestLive -v

// besuDevKey is the well-known prefunded account of Besu's `--network=dev` genesis. It is a public
// test key with no production use; nothing outside a throwaway dev node accepts it.
const besuDevKey = "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63"

func TestLiveNonceDriftRecoversAfterReceiptTimeout(t *testing.T) {
	rpc := os.Getenv("EVM_LIVE_RPC")
	if rpc == "" {
		t.Skip("warning: EVM_LIVE_RPC not set — skipping the live nonce-drift probe (see the comment above for the one-line Besu bring-up)")
	}

	ctx := context.Background()
	ec, err := Dial(ctx, rpc, 10*time.Second)
	if err != nil {
		t.Skipf("warning: cannot dial %s (%v) — skipping the live nonce-drift probe", rpc, err)
	}
	defer ec.Close()

	chainID, err := ec.ChainID(ctx)
	if err != nil {
		t.Fatalf("chain id: %v", err)
	}
	// NewSigner, not SharedSigner: this test drifts the counter on purpose and must not hand a
	// poisoned one to another test in the same process.
	s, err := NewSigner(besuDevKey, chainID)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	// Any address without code: the calldata is ignored and the transaction still consumes its
	// nonce, which is the only thing under test here.
	target := common.HexToAddress("0x00000000000000000000000000000000000000aa")

	// Phase A — a healthy submission, which also seeds the counter from the node.
	if _, _, err := SubmitRawTxReceipt(ctx, ec, s, target, []byte{0x01}, "probe baseline"); err != nil {
		t.Fatalf("baseline submission failed — the probe needs a working node and a funded account: %v", err)
	}
	if !s.nonceInit {
		t.Fatal("counter was not seeded by the baseline submission")
	}

	real, err := ec.PendingNonceAt(ctx, s.Address())
	if err != nil {
		t.Fatalf("pending nonce: %v", err)
	}

	// Phase B — drift the counter ahead, exactly as a chain reset under a live process does, and
	// shorten the wait so the probe does not take 90 s.
	const drift = 50
	s.mu.Lock()
	s.nonce = real + drift
	s.nonceInit = true
	s.mu.Unlock()

	previous := ReceiptWaitTimeout
	ReceiptWaitTimeout = 10 * time.Second
	defer func() { ReceiptWaitTimeout = previous }()

	_, _, err = SubmitRawTxReceipt(ctx, ec, s, target, []byte{0x01}, "probe drifted")
	if err == nil {
		t.Fatal("a transaction 50 nonces ahead was mined — the drift scenario did not reproduce")
	}
	// Fact 1: the failure must come from the receipt wait, not from the broadcast. If Besu had
	// rejected the future nonce, the existing isNonceTooLow path would already have handled it and
	// none of this code would be needed.
	if !strings.Contains(err.Error(), "wait mined") {
		t.Fatalf("the drifted submission failed at broadcast, not at the receipt wait: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the wait to die on its deadline, got: %v", err)
	}

	// The wiring: submitRawInternal must have marked the counter stale.
	s.mu.Lock()
	stillTrusted := s.nonceInit
	s.mu.Unlock()
	if stillTrusted {
		t.Fatal("counter still claims to be in step after a receipt wait died on its deadline")
	}

	// Fact 2: the node must report the account's real nonce, ignoring the gapped transaction still
	// sitting in its pool. This is the assumption the whole fix rests on.
	afterDrift, err := ec.PendingNonceAt(ctx, s.Address())
	if err != nil {
		t.Fatalf("pending nonce: %v", err)
	}
	if afterDrift != real {
		t.Fatalf("PendingNonceAt returned %d with a gapped tx at %d in the pool, want %d — re-seeding would restore the drift, not fix it",
			afterDrift, real+drift, real)
	}

	// Phase C — the recovery itself: the next submission re-seeds and mines.
	ReceiptWaitTimeout = previous
	if _, _, err := SubmitRawTxReceipt(ctx, ec, s, target, []byte{0x01}, "probe recovery"); err != nil {
		t.Fatalf("the submission after the timeout did not recover: %v", err)
	}
	s.mu.Lock()
	recovered := s.nonce
	s.mu.Unlock()
	if recovered != real+1 {
		t.Fatalf("counter is at %d after recovery, want %d", recovered, real+1)
	}
}
