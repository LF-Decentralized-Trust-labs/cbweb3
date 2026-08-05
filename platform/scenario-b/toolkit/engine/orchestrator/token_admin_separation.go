// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// Separation of W-token ADMINISTRATION from ISSUANCE.
//
// The currency handover leaves the CB's gateway identity holding both DEFAULT_ADMIN_ROLE (who may
// issue) and CENTRAL_BANK_ROLE (issue). Those are different kinds of authority: the gateway mints on
// every liquidity provisioning, so its key is operational and lives in a long-running container,
// while administration is exercised at provisioning time. Fused, a compromise of that container
// yields PERMANENT issuance rights — the attacker grants the role to an address of their own, and
// rotating the gateway key afterwards does not take it back.
//
// This separates them with the keys the toolkit already has, after the handover, so nothing in the
// currency-registration path (which crosses HTTP and gRPC to the hub's compliance service) has to
// change.

// tokenAdminAct is one on-chain act of the separation.
type tokenAdminAct string

const (
	// actGrantRelayerIssuance gives the CB's relayer CENTRAL_BANK_ROLE. It moves here from the
	// gateway's boot sequence: once the gateway is no longer the administrator it cannot grant
	// anything, so the grant becomes a provisioning act.
	actGrantRelayerIssuance tokenAdminAct = "grant-relayer-issuance"
	// actGrantTokenAdmin gives DEFAULT_ADMIN_ROLE to the dedicated administration identity, whose
	// key is never handed to a container.
	actGrantTokenAdmin tokenAdminAct = "grant-token-admin"
	// actRevokeGatewayAdmin takes DEFAULT_ADMIN_ROLE away from the gateway. Without it the split is
	// nominal: the gateway would still be able to grant issuance to anyone.
	actRevokeGatewayAdmin tokenAdminAct = "revoke-gateway-admin"
	// actRevokeGatewayIssuance is declared only to be asserted against: the gateway MUST keep
	// CENTRAL_BANK_ROLE, since it mints when provisioning liquidity. No plan ever emits it.
	actRevokeGatewayIssuance tokenAdminAct = "revoke-gateway-issuance"
)

// tokenAdminState is what the chain currently says.
type tokenAdminState struct {
	// RelayerHasIssuance: the CB's relayer holds CENTRAL_BANK_ROLE.
	RelayerHasIssuance bool
	// AdminHasAdmin: the dedicated administration identity holds DEFAULT_ADMIN_ROLE.
	AdminHasAdmin bool
	// GatewayHasAdmin: the gateway identity still holds DEFAULT_ADMIN_ROLE.
	GatewayHasAdmin bool
}

// planTokenAdminSeparation returns the outstanding acts, in the order they must be applied.
//
// Every act is signed by the GATEWAY key, because it is the current administrator. That single fact
// fixes the order: the revoke of the gateway's own administration disables the signer, so it is
// always last, and it is never emitted while nobody else administers the token. An already
// separated token yields an empty plan, so a re-apply is a no-op rather than a re-grant.
func planTokenAdminSeparation(st tokenAdminState) []tokenAdminAct {
	var plan []tokenAdminAct
	if !st.RelayerHasIssuance {
		plan = append(plan, actGrantRelayerIssuance)
	}
	if !st.AdminHasAdmin {
		plan = append(plan, actGrantTokenAdmin)
	}
	// Administration must rest elsewhere before this revoke, and it always does: either the admin
	// identity already holds it, or the grant above is in this same plan and precedes this act. So
	// the only condition here is that there is something to revoke. The guarantee is asserted by
	// TestPlanTokenAdminSeparation_NeverStrandsAdministration rather than re-encoded as a condition
	// that cannot be false.
	if st.GatewayHasAdmin {
		plan = append(plan, actRevokeGatewayAdmin)
	}
	return plan
}

// --- execution ---
//
// The acts are applied with `cast`, which the toolkit already requires for the contract work
// (`forge` is the same Foundry install). Reads are `cast call`, writes are `cast send --legacy` —
// the hub is zero-gas, so a legacy transaction avoids the EIP-1559 fee path entirely.

// tokenRoleReader reads a role holder's status on the W-token.
type tokenAdminExec struct {
	Runner  exec.CommandRunner
	RPCURL  string
	Token   string
	Signer  string // the GATEWAY private key: the current administrator, and the only signer here
	Gateway string
	Relayer string
	Admin   string
}

// roleID reads a role's bytes32 identifier from the token rather than assuming it. DEFAULT_ADMIN_ROLE
// is zero in OpenZeppelin today, but hard-coding a role id in a security-relevant path is the kind of
// assumption that survives a contract change silently.
func (e tokenAdminExec) roleID(ctx context.Context, name string) (string, error) {
	out, err := e.Runner.Run(ctx, "cast", "call", e.Token, name+"()(bytes32)", "--rpc-url", e.RPCURL)
	if err != nil {
		return "", fmt.Errorf("read %s on %s: %w", name, e.Token, err)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", fmt.Errorf("read %s on %s: empty response", name, e.Token)
	}
	return id, nil
}

func (e tokenAdminExec) hasRole(ctx context.Context, roleID, account string) (bool, error) {
	out, err := e.Runner.Run(ctx, "cast", "call", e.Token,
		"hasRole(bytes32,address)(bool)", roleID, account, "--rpc-url", e.RPCURL)
	if err != nil {
		return false, fmt.Errorf("read hasRole(%s, %s): %w", roleID, account, err)
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "true"), nil
}

func (e tokenAdminExec) submit(ctx context.Context, fn, roleID, account string) error {
	_, err := e.Runner.Run(ctx, "cast", "send", e.Token,
		fn+"(bytes32,address)", roleID, account,
		"--private-key", e.Signer, "--rpc-url", e.RPCURL, "--legacy")
	if err != nil {
		return fmt.Errorf("%s(%s, %s): %w", fn, roleID, account, err)
	}
	return nil
}

// apply reads the current state, plans, and applies the outstanding acts in order.
//
// Reading first is what makes it idempotent: a re-apply on an already separated token issues no
// transaction at all. The read cannot be skipped in favour of a stored flag — the chain is the
// authority on who administers the token, and a state file can disagree with it.
func (e tokenAdminExec) apply(ctx context.Context) ([]tokenAdminAct, error) {
	issuance, err := e.roleID(ctx, "CENTRAL_BANK_ROLE")
	if err != nil {
		return nil, err
	}
	admin, err := e.roleID(ctx, "DEFAULT_ADMIN_ROLE")
	if err != nil {
		return nil, err
	}

	var st tokenAdminState
	if st.RelayerHasIssuance, err = e.hasRole(ctx, issuance, e.Relayer); err != nil {
		return nil, err
	}
	if st.AdminHasAdmin, err = e.hasRole(ctx, admin, e.Admin); err != nil {
		return nil, err
	}
	if st.GatewayHasAdmin, err = e.hasRole(ctx, admin, e.Gateway); err != nil {
		return nil, err
	}

	plan := planTokenAdminSeparation(st)
	for _, act := range plan {
		switch act {
		case actGrantRelayerIssuance:
			err = e.submit(ctx, "grantRole", issuance, e.Relayer)
		case actGrantTokenAdmin:
			err = e.submit(ctx, "grantRole", admin, e.Admin)
		case actRevokeGatewayAdmin:
			// Last, and only reachable once administration rests with e.Admin: this transaction is
			// what ends the signing key's own authority.
			err = e.submit(ctx, "revokeRole", admin, e.Gateway)
		default:
			err = fmt.Errorf("unknown act %q", act)
		}
		if err != nil {
			return plan, fmt.Errorf("apply %s: %w", act, err)
		}
	}
	return plan, nil
}
