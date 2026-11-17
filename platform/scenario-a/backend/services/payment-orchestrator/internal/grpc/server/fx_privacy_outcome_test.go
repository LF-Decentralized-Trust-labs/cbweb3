// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A private submission can end three ways that are not "it failed, try again": this node has a
// stuck transaction and refused to add another, Paladin could not be reached, or the transaction
// was (or may have been) accepted and its outcome is unknown. The gateway turns FailedPrecondition into 409 and Unavailable
// into 503; Internal (500) is what callers used to see for all three, and they resubmitted.

// outcomeFXClient returns proposeErr from Propose and lifecycleErr from every other call.
// lifecycleErr is set by the test while the server goroutine reads it, hence the mutex.
type outcomeFXClient struct {
	proposeErr error
	mu         sync.Mutex
	lcErr      error
}

func (c *outcomeFXClient) setLifecycleErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lcErr = err
}

func (c *outcomeFXClient) lifecycleErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lcErr
}

func (c *outcomeFXClient) Propose(context.Context, ports.FXProposalParams) (string, error) {
	return "tx-propose", c.proposeErr
}
func (c *outcomeFXClient) ProposeOnBehalf(context.Context, ports.FXProposalParams) (string, error) {
	return "tx-propose-ob", c.proposeErr
}
func (c *outcomeFXClient) Accept(context.Context, [32]byte) (string, error) {
	return "tx-accept", c.lifecycleErr()
}
func (c *outcomeFXClient) AcceptOnBehalf(context.Context, [32]byte) (string, error) {
	return "tx-accept-ob", c.lifecycleErr()
}
func (c *outcomeFXClient) Reject(context.Context, [32]byte) (string, error) {
	return "tx-reject", c.lifecycleErr()
}
func (c *outcomeFXClient) RejectOnBehalf(context.Context, [32]byte) (string, error) {
	return "tx-reject-ob", c.lifecycleErr()
}
func (c *outcomeFXClient) Cancel(context.Context, [32]byte) (string, error) {
	return "tx-cancel", c.lifecycleErr()
}
func (c *outcomeFXClient) Settle(context.Context, [32]byte) (string, error) {
	return "tx-settle", c.lifecycleErr()
}

var privacyOutcomes = []struct {
	name string
	err  error
	want codes.Code
}{
	{"stuck", fmt.Errorf("%w: Pente transaction abc pending for 10m", ports.ErrPrivacyLayerStuck), codes.FailedPrecondition},
	{"unconfirmed", fmt.Errorf("%w: pgroup transaction abc", ports.ErrPrivateTxUnconfirmed), codes.FailedPrecondition},
	{"unavailable", fmt.Errorf("%w: no answer", ports.ErrPrivacyLayerUnavailable), codes.Unavailable},
	{"plain failure", errors.New("reverted: canTransact"), codes.Internal},
}

func proposeReq(tid string) *pb.ProposeFXAgreementRequest {
	return &pb.ProposeFXAgreementRequest{
		TradeId: tid, Originator: "bank-a", CounterpartyB: "bank-b",
		OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD", CounterCurrency: "BRL",
		Rate: "1.2", ExpiryDate: uint64(time.Now().Add(time.Hour).Unix()),
		SourceSpokeId: "spoke-a", DestSpokeId: "spoke-b",
		SourceReceiver: "recv@spoke-a-bank-a", DestReceiver: "recv@spoke-b-bank-b",
	}
}

// Both propose paths: the bank's own propose, and the relay's on-behalf propose on the
// destination spoke, which is the one most likely to be retried by automation.
func TestFX_Propose_MapsPrivacyOutcomes(t *testing.T) {
	for _, onBehalf := range []bool{false, true} {
		for _, tc := range privacyOutcomes {
			name := tc.name
			if onBehalf {
				name = "on-behalf/" + name
			}
			t.Run(name, func(t *testing.T) {
				fx := &outcomeFXClient{proposeErr: tc.err}
				env := setupFXChainEnv(t, server.Config{
					FXRepo: newMemFXRepo(), Pente: &fakePente{}, FXAgreementBesu: fx, FXAgreementPente: fx,
				})
				req := proposeReq("T-OUT-" + strings.ReplaceAll(name, "/", "-"))
				req.OnBehalf = onBehalf
				_, err := env.client.ProposeFXAgreement(context.Background(), req)
				if status.Code(err) != tc.want {
					t.Fatalf("got %v, want %s", err, tc.want)
				}
				msg := status.Convert(err).Message()
				if !strings.Contains(msg, "on-chain FX propose") {
					t.Errorf("message lost the operation: %q", msg)
				}
				// An unconfirmed propose returns no response, so the error is the only place the
				// caller learns which trade to follow up.
				if tc.want != codes.Internal && !strings.Contains(msg, "trade_id="+req.TradeId) {
					t.Errorf("message does not carry the trade id: %q", msg)
				}
			})
		}
	}
}

func TestFX_Lifecycle_MapsPrivacyOutcomes(t *testing.T) {
	ops := map[string]func(c pb.PaymentOrchestratorServiceClient, tid string) error{
		"accept": func(c pb.PaymentOrchestratorServiceClient, tid string) error {
			_, err := c.AcceptFXAgreement(context.Background(), &pb.AcceptFXAgreementRequest{TradeId: tid})
			return err
		},
		"reject": func(c pb.PaymentOrchestratorServiceClient, tid string) error {
			_, err := c.RejectFXAgreement(context.Background(), &pb.RejectFXAgreementRequest{TradeId: tid})
			return err
		},
		"cancel": func(c pb.PaymentOrchestratorServiceClient, tid string) error {
			_, err := c.CancelFXAgreement(context.Background(), &pb.CancelFXAgreementRequest{TradeId: tid})
			return err
		},
		"accept-on-behalf": func(c pb.PaymentOrchestratorServiceClient, tid string) error {
			_, err := c.AcceptFXAgreement(context.Background(), &pb.AcceptFXAgreementRequest{TradeId: tid, OnBehalf: true})
			return err
		},
		"reject-on-behalf": func(c pb.PaymentOrchestratorServiceClient, tid string) error {
			_, err := c.RejectFXAgreement(context.Background(), &pb.RejectFXAgreementRequest{TradeId: tid, OnBehalf: true})
			return err
		},
		// settle needs an ACCEPTED agreement; acceptFirst below gets it there.
		"settle": func(c pb.PaymentOrchestratorServiceClient, tid string) error {
			_, err := c.SettleFXAgreement(context.Background(), &pb.SettleFXAgreementRequest{TradeId: tid})
			return err
		},
	}
	acceptFirst := map[string]bool{"settle": true}
	for opName, op := range ops {
		for _, tc := range privacyOutcomes {
			t.Run(opName+"/"+tc.name, func(t *testing.T) {
				fx := &outcomeFXClient{}
				env := setupFXChainEnv(t, server.Config{
					FXRepo: newMemFXRepo(), Pente: &fakePente{}, FXAgreementBesu: fx, FXAgreementPente: fx,
				})
				tid := proposeChain(t, env.client, "T-LC-"+opName+"-"+tc.name)
				if acceptFirst[opName] {
					if _, err := env.client.AcceptFXAgreement(context.Background(), &pb.AcceptFXAgreementRequest{TradeId: tid}); err != nil {
						t.Fatalf("accept before %s: %v", opName, err)
					}
				}
				fx.setLifecycleErr(tc.err)
				err := op(env.client, tid)
				if status.Code(err) != tc.want {
					t.Fatalf("got %v, want %s", err, tc.want)
				}
				if tc.want != codes.Internal && !strings.Contains(status.Convert(err).Message(), "trade_id="+tid) {
					t.Errorf("message does not carry the trade id: %q", status.Convert(err).Message())
				}
			})
		}
	}
}
