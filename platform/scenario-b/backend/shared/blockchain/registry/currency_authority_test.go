// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// The hub registers a sovereign currency because CurrencyRegistry.registerCurrency demands
// msg.sender == getCentralBankOf(token) and the hub holds the only key available at
// provisioning time. Issuance authority must then move to the CB — otherwise the hub can mint
// another sovereign's money, which is what a single shared hub key used to hide.
func TestPlanCurrencyAuthority(t *testing.T) {
	signer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	cb := common.HexToAddress("0x2222222222222222222222222222222222222222")
	other := common.HexToAddress("0x3333333333333333333333333333333333333333")
	var zero common.Address

	cases := []struct {
		name          string
		cb            common.Address
		currentCBOf   common.Address
		cbHasRole     bool
		signerHasRole bool
		want          currencyAuthorityPlan
	}{
		{
			name:          "fresh registration hands over everything",
			cb:            cb,
			currentCBOf:   signer,
			cbHasRole:     false,
			signerHasRole: true,
			want:          currencyAuthorityPlan{SetCentralBankOf: true, GrantCBRole: true, RevokeSignerRole: true},
		},
		{
			name:          "already handed over is a no-op",
			cb:            cb,
			currentCBOf:   cb,
			cbHasRole:     true,
			signerHasRole: false,
			want:          currencyAuthorityPlan{},
		},
		{
			name:          "interrupted after grant completes the rest",
			cb:            cb,
			currentCBOf:   signer,
			cbHasRole:     true,
			signerHasRole: true,
			want:          currencyAuthorityPlan{SetCentralBankOf: true, RevokeSignerRole: true},
		},
		{
			name:          "interrupted after mapping still revokes the hub",
			cb:            cb,
			currentCBOf:   cb,
			cbHasRole:     true,
			signerHasRole: true,
			want:          currencyAuthorityPlan{RevokeSignerRole: true},
		},
		{
			name:          "mapping pointing at a third party is corrected",
			cb:            cb,
			currentCBOf:   other,
			cbHasRole:     true,
			signerHasRole: false,
			want:          currencyAuthorityPlan{SetCentralBankOf: true},
		},
		{
			// Single-entity stacks where the hub signer IS the central bank: revoking would
			// leave the token with no issuer at all.
			name:          "cb equal to signer never revokes",
			cb:            signer,
			currentCBOf:   signer,
			cbHasRole:     true,
			signerHasRole: true,
			want:          currencyAuthorityPlan{},
		},
		{
			// A zero CB address means the caller named no sovereign; touching authority on
			// that basis would strand the token.
			name:          "zero cb address is a no-op",
			cb:            zero,
			currentCBOf:   signer,
			cbHasRole:     false,
			signerHasRole: true,
			want:          currencyAuthorityPlan{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planCurrencyAuthority(signer, tc.cb, tc.currentCBOf, tc.cbHasRole, tc.signerHasRole)
			if got != tc.want {
				t.Fatalf("planCurrencyAuthority = %+v, want %+v", got, tc.want)
			}
			if got.Empty() != (got == currencyAuthorityPlan{}) {
				t.Fatalf("Empty() disagrees with the zero plan: %+v", got)
			}
		})
	}
}
