// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func futureExpiry() uint64 { return uint64(time.Now().Add(24 * time.Hour).Unix()) }

func proposeValid(t *testing.T, env *flowEnv) string {
	t.Helper()
	resp, err := env.client.ProposeFXAgreement(context.Background(), &pb.ProposeFXAgreementRequest{
		Originator:      "0xorig",
		CounterpartyB:   "0xcp",
		OriginAmount:    "100",
		CounterAmount:   "500",
		OriginCurrency:  "BRL",
		CounterCurrency: "USD",
		Rate:            "5", // counter/origin = 500/100 = 5
		ExpiryDate:      futureExpiry(),
	})
	if err != nil {
		t.Fatalf("ProposeFXAgreement: %v", err)
	}
	return resp.TradeId
}

func TestFX_ProposeAcceptSettle(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	tradeID := proposeValid(t, env)

	get, err := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tradeID})
	if err != nil {
		t.Fatalf("GetFXAgreement: %v", err)
	}
	if get.Agreement.State != pb.FXAgreementState_FX_STATE_PROPOSED {
		t.Errorf("expected PROPOSED, got %s", get.Agreement.State)
	}

	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tradeID}); err != nil {
		t.Fatalf("AcceptFXAgreement: %v", err)
	}
	if _, err := env.client.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{TradeId: tradeID}); err != nil {
		t.Fatalf("SettleFXAgreement: %v", err)
	}

	get, _ = env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tradeID})
	if get.Agreement.State != pb.FXAgreementState_FX_STATE_SETTLED {
		t.Errorf("expected SETTLED, got %s", get.Agreement.State)
	}

	// Audit trail should have PROPOSED, ACCEPTED, SETTLED.
	ev, err := env.client.ListFXAgreementEvents(ctx, &pb.ListFXAgreementEventsRequest{TradeId: tradeID})
	if err != nil {
		t.Fatalf("ListFXAgreementEvents: %v", err)
	}
	if len(ev.Events) != 3 {
		t.Errorf("expected 3 audit events, got %d", len(ev.Events))
	}
}

func TestFX_Reject(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	tradeID := proposeValid(t, env)
	if _, err := env.client.RejectFXAgreement(ctx, &pb.RejectFXAgreementRequest{TradeId: tradeID}); err != nil {
		t.Fatalf("RejectFXAgreement: %v", err)
	}
	// Cannot accept a rejected agreement (terminal).
	_, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tradeID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

func TestFX_Cancel(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	tradeID := proposeValid(t, env)
	if _, err := env.client.CancelFXAgreement(ctx, &pb.CancelFXAgreementRequest{TradeId: tradeID}); err != nil {
		t.Fatalf("CancelFXAgreement: %v", err)
	}
	get, _ := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tradeID})
	if get.Agreement.State != pb.FXAgreementState_FX_STATE_CANCELLED {
		t.Errorf("expected CANCELLED, got %s", get.Agreement.State)
	}
}

func TestFX_SettleRequiresAccepted(t *testing.T) {
	env := newFlowEnv(t)
	tradeID := proposeValid(t, env)
	// Settling a PROPOSED (not ACCEPTED) agreement is a precondition failure.
	_, err := env.client.SettleFXAgreement(context.Background(), &pb.SettleFXAgreementRequest{TradeId: tradeID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

func TestFX_Propose_Validation(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	base := func() *pb.ProposeFXAgreementRequest {
		return &pb.ProposeFXAgreementRequest{
			CounterpartyB: "0xcp", OriginAmount: "100", CounterAmount: "500",
			OriginCurrency: "BRL", CounterCurrency: "USD", Rate: "5", ExpiryDate: futureExpiry(),
		}
	}
	tests := []struct {
		name   string
		mutate func(*pb.ProposeFXAgreementRequest)
	}{
		{"missing fields", func(r *pb.ProposeFXAgreementRequest) { r.CounterpartyB = "" }},
		{"past expiry", func(r *pb.ProposeFXAgreementRequest) { r.ExpiryDate = 1 }},
		{"same currency", func(r *pb.ProposeFXAgreementRequest) { r.CounterCurrency = "BRL" }},
		{"bad origin amount", func(r *pb.ProposeFXAgreementRequest) { r.OriginAmount = "-1" }},
		{"bad counter amount", func(r *pb.ProposeFXAgreementRequest) { r.CounterAmount = "abc" }},
		{"bad rate", func(r *pb.ProposeFXAgreementRequest) { r.Rate = "0" }},
		{"rate inconsistent", func(r *pb.ProposeFXAgreementRequest) { r.Rate = "99" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := base()
			tc.mutate(req)
			_, err := env.client.ProposeFXAgreement(ctx, req)
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestFX_GetNotFound(t *testing.T) {
	env := newFlowEnv(t)
	_, err := env.client.GetFXAgreement(context.Background(), &pb.GetFXAgreementRequest{TradeId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestFX_List_FilterByStateAndCounterparty(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	t1 := proposeValid(t, env)
	_ = proposeValid(t, env)
	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: t1}); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Filter by ACCEPTED state should return exactly t1.
	resp, err := env.client.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{State: "ACCEPTED"})
	if err != nil {
		t.Fatalf("ListFXAgreements: %v", err)
	}
	if len(resp.Agreements) != 1 || resp.Agreements[0].TradeId != t1 {
		t.Errorf("expected only %s ACCEPTED, got %+v", t1, resp.Agreements)
	}

	// Filter by counterparty returns both.
	all, _ := env.client.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{Counterparty: "0xcp"})
	if len(all.Agreements) != 2 {
		t.Errorf("expected 2 for counterparty, got %d", len(all.Agreements))
	}
}

func TestFX_ListEventsRequiresRepo(t *testing.T) {
	// No FXRepo configured → events listing is a precondition failure.
	env := setupFlowEnv(t, server.Config{Token: &mockToken{}, Fiat: &mockFiat{}})
	_, err := env.client.ListFXAgreementEvents(context.Background(), &pb.ListFXAgreementEventsRequest{TradeId: "x"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

// With an FXAgreementContractPort configured, lifecycle transitions invoke the
// matching on-chain method and persist the returned tx hash.
func TestFX_OnChainBranches(t *testing.T) {
	fx := &fakeFXContract{}
	escrowRepo := newFakeEscrowRepo()
	fxRepo := newFakeFXRepo()
	env := setupFlowEnv(t, server.Config{
		Token: &mockToken{}, Fiat: &mockFiat{},
		EscrowRepo: escrowRepo, FXRepo: fxRepo, FXAgreementBesu: fx,
	})
	ctx := context.Background()
	tradeID := proposeValid(t, env)

	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tradeID, OnBehalf: true}); err != nil {
		t.Fatalf("accept onbehalf: %v", err)
	}
	if _, err := env.client.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{TradeId: tradeID}); err != nil {
		t.Fatalf("settle: %v", err)
	}

	get, _ := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tradeID})
	if get.Agreement.State != pb.FXAgreementState_FX_STATE_SETTLED {
		t.Errorf("expected SETTLED, got %s", get.Agreement.State)
	}
	// AcceptOnBehalf (not Accept) and Settle must have been invoked on-chain.
	wantCalls := map[string]bool{"AcceptOnBehalf": false, "Settle": false}
	for _, c := range fx.called {
		if _, ok := wantCalls[c]; ok {
			wantCalls[c] = true
		}
	}
	for name, seen := range wantCalls {
		if !seen {
			t.Errorf("expected on-chain %s to be called; calls=%v", name, fx.called)
		}
	}
}

func TestFX_OnChainRejectAndCancel(t *testing.T) {
	ctx := context.Background()
	t.Run("reject onbehalf", func(t *testing.T) {
		fx := &fakeFXContract{}
		env := setupFlowEnv(t, server.Config{Token: &mockToken{}, Fiat: &mockFiat{}, EscrowRepo: newFakeEscrowRepo(), FXRepo: newFakeFXRepo(), FXAgreementBesu: fx})
		id := proposeValid(t, env)
		if _, err := env.client.RejectFXAgreement(ctx, &pb.RejectFXAgreementRequest{TradeId: id, OnBehalf: true}); err != nil {
			t.Fatalf("reject: %v", err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		fx := &fakeFXContract{}
		env := setupFlowEnv(t, server.Config{Token: &mockToken{}, Fiat: &mockFiat{}, EscrowRepo: newFakeEscrowRepo(), FXRepo: newFakeFXRepo(), FXAgreementBesu: fx})
		id := proposeValid(t, env)
		if _, err := env.client.CancelFXAgreement(ctx, &pb.CancelFXAgreementRequest{TradeId: id}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
	})
}

// On-chain failure surfaces as Internal and the state must not advance.
func TestFX_OnChainErrorAborts(t *testing.T) {
	fx := &fakeFXContract{err: errors.New("chain revert")}
	env := setupFlowEnv(t, server.Config{Token: &mockToken{}, Fiat: &mockFiat{}, EscrowRepo: newFakeEscrowRepo(), FXRepo: newFakeFXRepo(), FXAgreementBesu: fx})
	ctx := context.Background()
	id := proposeValid(t, env)
	_, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: id})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
	get, _ := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: id})
	if get.Agreement.State != pb.FXAgreementState_FX_STATE_PROPOSED {
		t.Errorf("state must remain PROPOSED after on-chain failure, got %s", get.Agreement.State)
	}
}

// In-memory fallback path (no FXRepo): propose/accept/list work off the in-memory map.
func TestFX_InMemoryFallback(t *testing.T) {
	env := setupFlowEnv(t, server.Config{Token: &mockToken{}, Fiat: &mockFiat{}, EscrowRepo: newFakeEscrowRepo()})
	ctx := context.Background()
	id := proposeValid(t, env)
	if _, err := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: id}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	list, err := env.client.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{State: "ACCEPTED"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Agreements) != 1 {
		t.Errorf("expected 1 in-memory ACCEPTED agreement, got %d", len(list.Agreements))
	}
}

func TestFX_TradeIDRequired(t *testing.T) {
	env := newFlowEnv(t)
	ctx := context.Background()
	for _, fn := range []func() error{
		func() error { _, e := env.client.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{}); return e },
		func() error { _, e := env.client.RejectFXAgreement(ctx, &pb.RejectFXAgreementRequest{}); return e },
		func() error { _, e := env.client.CancelFXAgreement(ctx, &pb.CancelFXAgreementRequest{}); return e },
		func() error { _, e := env.client.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{}); return e },
		func() error { _, e := env.client.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{}); return e },
	} {
		if status.Code(fn()) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for empty trade_id")
		}
	}
}
