// SPDX-License-Identifier: Apache-2.0

package orchestrator

import "testing"

func TestCentralBankRealmPlans_IncludeNOCAndTreasury(t *testing.T) {
	plans := centralBankRealmPlans("central-bank-brazil")
	if len(plans) != 2 {
		t.Fatalf("expected 2 realms (central-bank + cbweb3), got %d", len(plans))
	}
	cb := plans[0]
	if cb.Realm != "central-bank-brazil" {
		t.Errorf("realm = %q; want central-bank-brazil", cb.Realm)
	}
	if len(cb.Clients) != 2 {
		t.Fatalf("expected governance + treasury clients, got %d", len(cb.Clients))
	}
	if cb.Clients[0].ClientID != "central-bank-brazil-client" || cb.Clients[0].Secret != "central-bank-brazil-local-secret" {
		t.Errorf("governance client mismatch: %+v", cb.Clients[0])
	}
	if cb.Clients[0].Roles[0] != "ROLE_GOVERNANCE" {
		t.Errorf("governance role mismatch: %v", cb.Clients[0].Roles)
	}
	if plans[1].Realm != "cbweb3" {
		t.Errorf("second realm should be cbweb3 (NOC), got %q", plans[1].Realm)
	}
}

func TestCommercialBankRealmPlan(t *testing.T) {
	p := commercialBankRealmPlan("bank-itau")
	if p.Realm != "bank-itau" {
		t.Errorf("realm = %q; want bank-itau", p.Realm)
	}
	if p.Clients[0].ClientID != "bank-itau-client" {
		t.Errorf("client = %q; want bank-itau-client", p.Clients[0].ClientID)
	}
}

func TestGovernanceUserID(t *testing.T) {
	if got := governanceUserID("central-bank-brazil"); got != "service-account-central-bank-brazil-client" {
		t.Errorf("governanceUserID = %q", got)
	}
}
