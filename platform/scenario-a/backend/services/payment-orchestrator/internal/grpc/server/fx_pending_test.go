// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// An unconfirmed operation may still commit. The record must keep it, so the database can be
// reconciled with the chain when it does, and so no second operation is submitted meanwhile.

func unconfirmed(txID string) error {
	return &ports.PrivateTxUnconfirmedError{TxID: txID, Msg: "pgroup transaction " + txID + " has no receipt"}
}

func TestFX_UnconfirmedProposeIsRecordedAsPending(t *testing.T) {
	fx := &outcomeFXClient{proposeErr: unconfirmed("ptx-p")}
	repo := newMemFXRepo()
	env := setupFXChainEnv(t, server.Config{FXRepo: repo, Pente: &fakePente{}, FXAgreementBesu: fx, FXAgreementPente: fx})

	_, err := env.client.ProposeFXAgreement(context.Background(), proposeReq("T-PEND-P"))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("got %v, want FailedPrecondition", err)
	}
	rec, _ := repo.GetAgreement(context.Background(), "T-PEND-P")
	if rec == nil {
		t.Fatal("no record kept for an unconfirmed propose: if it commits later, the agreement exists on chain with no row")
	}
	if rec.State != domain.FXStateInvalid {
		t.Errorf("state = %s, want INVALID (not on chain yet, so the relay does not forward it)", rec.State)
	}
	if rec.Pending.Op != domain.FXPendingPropose || rec.Pending.TxID != "ptx-p" || rec.Pending.Since.IsZero() {
		t.Errorf("pending = %+v, want propose / ptx-p / a time", rec.Pending)
	}
	if rec.GroupID == "" || rec.ContractAddress == "" {
		t.Errorf("the record lacks the group the reconciler must read from: %+v", rec)
	}

	// A retry under the same trade id must not submit a second transaction.
	fx.proposeErr = nil
	_, err = env.client.ProposeFXAgreement(context.Background(), proposeReq("T-PEND-P"))
	if status.Code(err) != codes.FailedPrecondition || !strings.Contains(status.Convert(err).Message(), "ptx-p") {
		t.Fatalf("retry: got %v, want FailedPrecondition naming the pending transaction", err)
	}
}

func TestFX_UnconfirmedLifecycleIsRecordedAndBlocksTheNextOperation(t *testing.T) {
	type call func(c pb.PaymentOrchestratorServiceClient, tid string) error
	accept := func(c pb.PaymentOrchestratorServiceClient, tid string) error {
		_, err := c.AcceptFXAgreement(context.Background(), &pb.AcceptFXAgreementRequest{TradeId: tid})
		return err
	}
	cancel := func(c pb.PaymentOrchestratorServiceClient, tid string) error {
		_, err := c.CancelFXAgreement(context.Background(), &pb.CancelFXAgreementRequest{TradeId: tid})
		return err
	}
	reject := func(c pb.PaymentOrchestratorServiceClient, tid string) error {
		_, err := c.RejectFXAgreement(context.Background(), &pb.RejectFXAgreementRequest{TradeId: tid})
		return err
	}
	settle := func(c pb.PaymentOrchestratorServiceClient, tid string) error {
		_, err := c.SettleFXAgreement(context.Background(), &pb.SettleFXAgreementRequest{TradeId: tid})
		return err
	}
	cases := []struct {
		name        string
		acceptFirst bool
		op          call
		wantOp      domain.FXPendingOp
		wantState   domain.FXState
	}{
		{"accept", false, accept, domain.FXPendingAccept, domain.FXStateProposed},
		{"reject", false, reject, domain.FXPendingReject, domain.FXStateProposed},
		{"cancel", false, cancel, domain.FXPendingCancel, domain.FXStateProposed},
		{"settle", true, settle, domain.FXPendingSettle, domain.FXStateAccepted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := &outcomeFXClient{}
			repo := newMemFXRepo()
			env := setupFXChainEnv(t, server.Config{FXRepo: repo, Pente: &fakePente{}, FXAgreementBesu: fx, FXAgreementPente: fx})
			tid := proposeChain(t, env.client, "T-PEND-"+tc.name)
			if tc.acceptFirst {
				if err := accept(env.client, tid); err != nil {
					t.Fatalf("accept: %v", err)
				}
			}

			fx.setLifecycleErr(unconfirmed("ptx-" + tc.name))
			if err := tc.op(env.client, tid); status.Code(err) != codes.FailedPrecondition {
				t.Fatalf("got %v, want FailedPrecondition", err)
			}
			rec, _ := repo.GetAgreement(context.Background(), tid)
			if rec.State != tc.wantState {
				t.Errorf("state = %s, want %s unchanged until the chain confirms", rec.State, tc.wantState)
			}
			if rec.Pending.Op != tc.wantOp || rec.Pending.TxID != "ptx-"+tc.name {
				t.Errorf("pending = %+v, want %s / ptx-%s", rec.Pending, tc.wantOp, tc.name)
			}

			// Any further operation on the agreement is refused without reaching the chain.
			fx.setLifecycleErr(errors.New("must not be called"))
			err := cancel(env.client, tid)
			if status.Code(err) != codes.FailedPrecondition || !strings.Contains(status.Convert(err).Message(), "still pending") {
				t.Fatalf("follow-up cancel: got %v, want FailedPrecondition naming the pending operation", err)
			}
		})
	}
}

// Only an unknown outcome is recorded. A refusal (stuck node, unreachable node) or an ordinary
// failure means nothing was accepted, and the agreement stays operable.
func TestFX_OnlyUnconfirmedOutcomesAreRecorded(t *testing.T) {
	for name, err := range map[string]error{
		"stuck":       fmt.Errorf("%w: x", ports.ErrPrivacyLayerStuck),
		"unavailable": fmt.Errorf("%w: x", ports.ErrPrivacyLayerUnavailable),
		"reverted":    errors.New("reverted"),
	} {
		t.Run(name, func(t *testing.T) {
			fx := &outcomeFXClient{}
			repo := newMemFXRepo()
			env := setupFXChainEnv(t, server.Config{FXRepo: repo, Pente: &fakePente{}, FXAgreementBesu: fx, FXAgreementPente: fx})
			tid := proposeChain(t, env.client, "T-NOPEND-"+name)
			fx.setLifecycleErr(err)
			_, _ = env.client.AcceptFXAgreement(context.Background(), &pb.AcceptFXAgreementRequest{TradeId: tid})
			if rec, _ := repo.GetAgreement(context.Background(), tid); rec.Pending.IsSet() {
				t.Fatalf("pending recorded for %s: %+v", name, rec.Pending)
			}

			fx.proposeErr = err
			_, _ = env.client.ProposeFXAgreement(context.Background(), proposeReq("T-NOPEND-P-"+name))
			if rec, _ := repo.GetAgreement(context.Background(), "T-NOPEND-P-"+name); rec != nil {
				t.Fatalf("a record was kept for a propose that was not accepted (%s): %+v", name, rec)
			}
		})
	}
}
