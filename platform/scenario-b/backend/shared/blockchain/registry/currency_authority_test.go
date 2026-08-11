// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// The hub registers a sovereign currency because CurrencyRegistry.registerCurrency demands
// msg.sender == getCentralBankOf(token) and the hub holds the only key available at
// provisioning time. Authority must then move to the CB — issuance AND administration.
// Handing over CENTRAL_BANK_ROLE alone is cosmetic: while the hub keeps DEFAULT_ADMIN_ROLE it
// can grant issuance back to itself at any moment.
func TestPlanCurrencyAuthority(t *testing.T) {
	signer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	cb := common.HexToAddress("0x2222222222222222222222222222222222222222")
	other := common.HexToAddress("0x3333333333333333333333333333333333333333")
	var zero common.Address

	cases := []struct {
		name                     string
		cb                       common.Address
		currentCBOf              common.Address
		cbHasRole, signerHasRole bool
		cbIsAdmin, signerIsAdmin bool
		want                     currencyAuthorityPlan
	}{
		{
			name: "fresh registration hands over issuance and administration",
			cb:   cb, currentCBOf: signer,
			cbHasRole: false, signerHasRole: true,
			cbIsAdmin: false, signerIsAdmin: true,
			want: currencyAuthorityPlan{
				SetCentralBankOf: true, GrantCBRole: true, GrantCBAdmin: true,
				RevokeSignerRole: true, RevokeSignerAdmin: true,
			},
		},
		{
			name: "already handed over is a no-op",
			cb:   cb, currentCBOf: cb,
			cbHasRole: true, signerHasRole: false,
			cbIsAdmin: true, signerIsAdmin: false,
			want: currencyAuthorityPlan{},
		},
		{
			// The state the previous handover left behind: issuance moved, administration did
			// not. The hub can still grant CENTRAL_BANK_ROLE back to itself, so this must not
			// read as complete.
			name: "issuance moved but administration did not is incomplete",
			cb:   cb, currentCBOf: cb,
			cbHasRole: true, signerHasRole: false,
			cbIsAdmin: false, signerIsAdmin: true,
			want: currencyAuthorityPlan{GrantCBAdmin: true, RevokeSignerAdmin: true},
		},
		{
			name: "interrupted after the role grant completes the rest",
			cb:   cb, currentCBOf: signer,
			cbHasRole: true, signerHasRole: true,
			cbIsAdmin: false, signerIsAdmin: true,
			want: currencyAuthorityPlan{
				SetCentralBankOf: true, GrantCBAdmin: true,
				RevokeSignerRole: true, RevokeSignerAdmin: true,
			},
		},
		{
			name: "interrupted after the mapping still revokes the hub",
			cb:   cb, currentCBOf: cb,
			cbHasRole: true, signerHasRole: true,
			cbIsAdmin: true, signerIsAdmin: true,
			want: currencyAuthorityPlan{RevokeSignerRole: true, RevokeSignerAdmin: true},
		},
		{
			name: "mapping pointing at a third party is corrected",
			cb:   cb, currentCBOf: other,
			cbHasRole: true, signerHasRole: false,
			cbIsAdmin: true, signerIsAdmin: false,
			want: currencyAuthorityPlan{SetCentralBankOf: true},
		},
		{
			// Single-entity stacks where the hub signer IS the central bank: revoking would
			// leave the token with neither an issuer nor an administrator.
			name: "cb equal to signer never revokes",
			cb:   signer, currentCBOf: signer,
			cbHasRole: true, signerHasRole: true,
			cbIsAdmin: true, signerIsAdmin: true,
			want: currencyAuthorityPlan{},
		},
		{
			// A zero CB address means the caller named no sovereign; acting on it would strand
			// the token with no administrator at all.
			name: "zero cb address is a no-op",
			cb:   zero, currentCBOf: signer,
			cbHasRole: false, signerHasRole: true,
			cbIsAdmin: false, signerIsAdmin: true,
			want: currencyAuthorityPlan{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planCurrencyAuthority(signer, tc.cb, tc.currentCBOf,
				tc.cbHasRole, tc.signerHasRole, tc.cbIsAdmin, tc.signerIsAdmin)
			if got != tc.want {
				t.Fatalf("planCurrencyAuthority = %+v, want %+v", got, tc.want)
			}
			if got.Empty() != (got == currencyAuthorityPlan{}) {
				t.Fatalf("Empty() disagrees with the zero plan: %+v", got)
			}
		})
	}
}

// A plan that revokes the hub's administration must always also have put administration
// somewhere else first, or the token ends up with no administrator at all.
func TestPlanCurrencyAuthority_NeverStrandsAdministration(t *testing.T) {
	signer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	cb := common.HexToAddress("0x2222222222222222222222222222222222222222")

	for _, cbIsAdmin := range []bool{true, false} {
		for _, signerIsAdmin := range []bool{true, false} {
			p := planCurrencyAuthority(signer, cb, cb, true, false, cbIsAdmin, signerIsAdmin)
			if p.RevokeSignerAdmin && !cbIsAdmin && !p.GrantCBAdmin {
				t.Fatalf("plan revokes the hub's administration without granting it to the CB (cbIsAdmin=%v signerIsAdmin=%v): %+v",
					cbIsAdmin, signerIsAdmin, p)
			}
		}
	}
}

// A re-apply after the SOVEREIGN has moved administration on must be a no-op, not a failed grant.
//
// Provisioning separates W-token administration from issuance: the CB's gateway keeps
// CENTRAL_BANK_ROLE and DEFAULT_ADMIN_ROLE moves to a dedicated identity. This repair path then sees
// a CB that does not administer its own token and used to plan a grant — signed by the HUB, which
// gave up administration during the original handover. The grant reverts, and `register-currency`
// stops being idempotent: re-applying a separated spoke fails and every later step is skipped.
//
// What the handover actually protects is narrower than "the CB administers": it is that the HUB does
// not, so it cannot grant issuance back to itself. Once the hub is out, where the sovereign keeps
// administration is the sovereign's business.
func TestPlanCurrencyAuthority_NoGrantWhenAdministrationHasLeftTheHub(t *testing.T) {
	signer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	cb := common.HexToAddress("0x2222222222222222222222222222222222222222")

	// Post-separation steady state: the CB holds issuance, administration is elsewhere (a third
	// identity this package never sees), and the hub holds neither.
	p := planCurrencyAuthority(signer, cb, cb,
		true,  // cbHasRole
		false, // signerHasRole
		false, // cbIsAdmin — administration moved to the sovereign's admin identity
		false, // signerIsAdmin — the hub gave it up at handover
	)
	if p.GrantCBAdmin {
		t.Fatalf("planned a DEFAULT_ADMIN_ROLE grant the hub can no longer sign: %+v", p)
	}
	if !p.Empty() {
		t.Fatalf("expected nothing to do on a separated token, got %+v", p)
	}
}

// The protection itself must not weaken: while the hub still administers, the handover proceeds.
func TestPlanCurrencyAuthority_StillGrantsWhileTheHubAdministers(t *testing.T) {
	signer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	cb := common.HexToAddress("0x2222222222222222222222222222222222222222")

	p := planCurrencyAuthority(signer, cb, cb, true, false, false, true)
	if !p.GrantCBAdmin {
		t.Fatalf("the hub still administers, so administration must be handed over: %+v", p)
	}
	if !p.RevokeSignerAdmin {
		t.Fatalf("the hub's administration must be revoked: %+v", p)
	}
}
