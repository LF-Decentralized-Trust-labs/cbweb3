// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

// In-process gRPC exercise of the payment-orchestrator escrow (tokenization)
// path over bufconn against the REAL grpc/server: RequestEscrow → ApproveEscrow
// performs the atomic fCeBM burn → tCeBM mint pair via the fake token/fiat ports.
// Validates the bufconn wiring and the orchestrator's burn→mint atomicity at the
// gRPC boundary (no silent partial settlement when a leg reverts).
package integrationlite

import (
	"context"
	"errors"
	"testing"

	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

func TestOrchestratorGRPC_EscrowBurnMint_Atomic(t *testing.T) {
	env := newOrchestratorEnv(t)
	ctx := context.Background()

	dep, err := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{
		RequesterBesuAddress: "0xbankA",
		Amount:               "5000",
	})
	if err != nil {
		t.Fatalf("RegisterDeposit: %v", err)
	}

	esc, err := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress: "0xbankA",
		Amount:               "5000",
		DepositId:            dep.DepositId,
	})
	if err != nil {
		t.Fatalf("RequestEscrow: %v", err)
	}

	resp, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: esc.EscrowId})
	if err != nil {
		t.Fatalf("ApproveEscrow: %v", err)
	}
	if resp.BurnTxHash == "" || resp.MintTxHash == "" {
		t.Fatalf("expected both burn and mint tx hashes, got burn=%q mint=%q", resp.BurnTxHash, resp.MintTxHash)
	}
	if env.fiat.burnCount() != 1 {
		t.Errorf("expected exactly 1 fCeBM burn, got %d", env.fiat.burnCount())
	}
	if env.token.mintCount() != 1 {
		t.Errorf("expected exactly 1 tCeBM mint, got %d", env.token.mintCount())
	}
}

// When the tCeBM mint leg reverts, ApproveEscrow must surface an error — no
// silent partial settlement at the gRPC boundary.
func TestOrchestratorGRPC_EscrowMintFails_Errors(t *testing.T) {
	env := newOrchestratorEnv(t)
	env.token.mintErr = errors.New("revert: mint cap exceeded")
	ctx := context.Background()

	dep, _ := env.client.RegisterDeposit(ctx, &pb.RegisterDepositRequest{RequesterBesuAddress: "0xbankA", Amount: "5000"})
	esc, _ := env.client.RequestEscrow(ctx, &pb.RequestEscrowRequest{RequesterBesuAddress: "0xbankA", Amount: "5000", DepositId: dep.DepositId})

	if _, err := env.client.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: esc.EscrowId}); err == nil {
		t.Fatal("expected ApproveEscrow to fail when tCeBM mint reverts")
	}
}
