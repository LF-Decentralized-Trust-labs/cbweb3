package orchestrator

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// Separating W-token administration from issuance is three on-chain acts, and their ORDER is the
// whole correctness question. The gateway key is what signs all three (it is the current
// administrator), so revoking its administration is necessarily the last act — after it, that key
// can no longer grant anything. Getting the order wrong leaves the token either unadministrable or
// with a relayer that cannot mint, and neither is recoverable with the keys the toolkit holds.
//
// The decision lives in a pure function, mirroring planCurrencyAuthority, so the invariants can be
// asserted exhaustively without a chain.

func TestPlanTokenAdminSeparation_OrdersGrantsBeforeTheRevoke(t *testing.T) {
	got := planTokenAdminSeparation(tokenAdminState{
		RelayerHasIssuance: false,
		AdminHasAdmin:      false,
		GatewayHasAdmin:    true,
	})
	want := []tokenAdminAct{actGrantRelayerIssuance, actGrantTokenAdmin, actRevokeGatewayAdmin}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %v, want %v", got, want)
	}
}

// The revoke is what removes the signing key's own authority, so it can never come first — nor can
// it appear at all while nobody else administers the token.
func TestPlanTokenAdminSeparation_NeverStrandsAdministration(t *testing.T) {
	for _, relayerHas := range []bool{true, false} {
		for _, adminHas := range []bool{true, false} {
			for _, gatewayHas := range []bool{true, false} {
				st := tokenAdminState{relayerHas, adminHas, gatewayHas}
				plan := planTokenAdminSeparation(st)

				revokeAt := indexOf(plan, actRevokeGatewayAdmin)
				if revokeAt < 0 {
					continue
				}
				// Someone else must already administer the token, or be granted it earlier in
				// this same plan.
				grantAt := indexOf(plan, actGrantTokenAdmin)
				if !adminHas && (grantAt < 0 || grantAt > revokeAt) {
					t.Fatalf("%+v: plan revokes the gateway's administration with nobody else administering: %v", st, plan)
				}
				// The revoke disables the signer, so nothing may follow it.
				if revokeAt != len(plan)-1 {
					t.Fatalf("%+v: the revoke must be last (nothing can be signed after it): %v", st, plan)
				}
			}
		}
	}
}

// A relayer grant after the revoke would be unsignable: the gateway key no longer administers.
func TestPlanTokenAdminSeparation_RelayerGrantPrecedesTheRevoke(t *testing.T) {
	plan := planTokenAdminSeparation(tokenAdminState{
		RelayerHasIssuance: false,
		AdminHasAdmin:      true,
		GatewayHasAdmin:    true,
	})
	grantAt := indexOf(plan, actGrantRelayerIssuance)
	revokeAt := indexOf(plan, actRevokeGatewayAdmin)
	if grantAt < 0 || revokeAt < 0 || grantAt > revokeAt {
		t.Fatalf("the relayer grant must precede the revoke: %v", plan)
	}
}

// Already separated: nothing to do. A re-apply must be a no-op, not a re-grant.
func TestPlanTokenAdminSeparation_EmptyWhenAlreadySeparated(t *testing.T) {
	plan := planTokenAdminSeparation(tokenAdminState{
		RelayerHasIssuance: true,
		AdminHasAdmin:      true,
		GatewayHasAdmin:    false,
	})
	if len(plan) != 0 {
		t.Fatalf("expected no acts on an already separated token, got %v", plan)
	}
}

// The gateway keeps CENTRAL_BANK_ROLE throughout: it mints on every liquidity provisioning
// (MintAndApproveForAMM). Only its ADMINISTRATION is taken away.
func TestPlanTokenAdminSeparation_NeverTouchesGatewayIssuance(t *testing.T) {
	for _, relayerHas := range []bool{true, false} {
		for _, adminHas := range []bool{true, false} {
			for _, gatewayHas := range []bool{true, false} {
				for _, act := range planTokenAdminSeparation(tokenAdminState{relayerHas, adminHas, gatewayHas}) {
					if act == actRevokeGatewayIssuance {
						t.Fatalf("plan revokes the gateway's issuance — it would stop being able to provision liquidity")
					}
				}
			}
		}
	}
}

func indexOf(acts []tokenAdminAct, want tokenAdminAct) int {
	for i, a := range acts {
		if a == want {
			return i
		}
	}
	return -1
}

// --- execution ---

// scriptedRunner answers each call from a queue and records everything. The shared FakeRunner
// returns one canned output per command name, which cannot express three hasRole reads with
// different answers — and those answers are exactly what drives the plan.
type scriptedRunner struct {
	outputs []string
	calls   [][]string
	err     error
	failOn  string // fail the first send whose joined args contain this
}

func (r *scriptedRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	joined := strings.Join(args, " ")
	if r.failOn != "" && strings.Contains(joined, r.failOn) {
		return nil, fmt.Errorf("injected failure")
	}
	if r.err != nil {
		return nil, r.err
	}
	if len(r.outputs) == 0 {
		return nil, nil
	}
	out := r.outputs[0]
	r.outputs = r.outputs[1:]
	return []byte(out), nil
}

func (r *scriptedRunner) sends() [][]string {
	var out [][]string
	for _, c := range r.calls {
		if len(c) > 1 && c[1] == "send" {
			out = append(out, c)
		}
	}
	return out
}

func newExec(r exec.CommandRunner) tokenAdminExec {
	return tokenAdminExec{
		Runner: r, RPCURL: "http://hub:8545", Token: "0xTOKEN",
		Signer: "0xGATEWAYKEY", Gateway: "0xGATEWAY", Relayer: "0xRELAYER", Admin: "0xADMIN",
	}
}

// The full separation on a freshly handed-over token: nothing granted yet, the gateway administers.
func TestTokenAdminExec_AppliesTheActsInOrder(t *testing.T) {
	r := &scriptedRunner{outputs: []string{
		"0xISSUANCEROLE", // CENTRAL_BANK_ROLE
		"0xADMINROLE",    // DEFAULT_ADMIN_ROLE
		"false",          // relayer has issuance?
		"false",          // admin identity administers?
		"true",           // gateway administers?
	}}
	plan, err := newExec(r).apply(context.Background())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []tokenAdminAct{actGrantRelayerIssuance, actGrantTokenAdmin, actRevokeGatewayAdmin}
	if !reflect.DeepEqual(plan, want) {
		t.Fatalf("plan = %v, want %v", plan, want)
	}

	sends := r.sends()
	if len(sends) != 3 {
		t.Fatalf("expected 3 transactions, got %d: %v", len(sends), sends)
	}
	assertSend(t, sends[0], "grantRole(bytes32,address)", "0xISSUANCEROLE", "0xRELAYER")
	assertSend(t, sends[1], "grantRole(bytes32,address)", "0xADMINROLE", "0xADMIN")
	assertSend(t, sends[2], "revokeRole(bytes32,address)", "0xADMINROLE", "0xGATEWAY")

	// Every act is signed by the gateway key — it is the only administrator until the last one.
	for i, s := range sends {
		if !contains(s, "--private-key") || !contains(s, "0xGATEWAYKEY") {
			t.Fatalf("send %d is not signed by the gateway key: %v", i, s)
		}
		// Zero-gas hub: a legacy transaction avoids the EIP-1559 fee path.
		if !contains(s, "--legacy") {
			t.Fatalf("send %d is missing --legacy: %v", i, s)
		}
	}
}

// A re-apply on an already separated token must issue NO transaction. Re-granting would be
// harmless; re-revoking would not, and a step that writes on every run cannot be trusted to be
// idempotent by inspection.
func TestTokenAdminExec_IdempotentOnAnAlreadySeparatedToken(t *testing.T) {
	r := &scriptedRunner{outputs: []string{
		"0xISSUANCEROLE", "0xADMINROLE",
		"true",  // relayer already issues
		"true",  // admin identity already administers
		"false", // gateway no longer administers
	}}
	plan, err := newExec(r).apply(context.Background())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(plan) != 0 {
		t.Fatalf("expected an empty plan, got %v", plan)
	}
	if sends := r.sends(); len(sends) != 0 {
		t.Fatalf("expected no transactions, got %v", sends)
	}
}

// If the administration grant fails, the revoke must NOT be attempted: that combination is what
// would leave the token with no administrator at all.
func TestTokenAdminExec_DoesNotRevokeWhenTheGrantFails(t *testing.T) {
	r := &scriptedRunner{
		outputs: []string{"0xISSUANCEROLE", "0xADMINROLE", "true", "false", "true"},
		failOn:  "0xADMIN", // the grant of administration to the admin identity
	}
	if _, err := newExec(r).apply(context.Background()); err == nil {
		t.Fatal("expected the failed grant to surface")
	}
	for _, s := range r.sends() {
		if contains(s, "revokeRole(bytes32,address)") {
			t.Fatalf("revoked the gateway's administration after the grant failed: %v", s)
		}
	}
}

// A role id read as empty must fail rather than be sent as "" — a grantRole with a zero role would
// touch DEFAULT_ADMIN_ROLE by accident.
func TestTokenAdminExec_RefusesAnEmptyRoleID(t *testing.T) {
	r := &scriptedRunner{outputs: []string{""}}
	if _, err := newExec(r).apply(context.Background()); err == nil {
		t.Fatal("expected an empty role id to be refused")
	}
	if sends := r.sends(); len(sends) != 0 {
		t.Fatalf("expected no transactions, got %v", sends)
	}
}

func assertSend(t *testing.T, call []string, sig, roleID, account string) {
	t.Helper()
	if !contains(call, sig) || !contains(call, roleID) || !contains(call, account) {
		t.Fatalf("expected send of %s(%s, %s), got %v", sig, roleID, account, call)
	}
}

func contains(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}
