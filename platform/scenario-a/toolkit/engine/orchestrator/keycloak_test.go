// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"encoding/json"
	"testing"
)

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

func TestRenderRealmJSON_ClientsRolesSecret(t *testing.T) {
	plans := centralBankRealmPlans("central-bank-brazil")
	data, err := renderRealmJSON(plans[0])
	if err != nil {
		t.Fatalf("renderRealmJSON: %v", err)
	}
	var realm map[string]any
	if err := json.Unmarshal(data, &realm); err != nil {
		t.Fatalf("invalid realm JSON: %v", err)
	}
	if realm["realm"] != "central-bank-brazil" {
		t.Errorf("realm = %v", realm["realm"])
	}
	clients := realm["clients"].([]any)
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	gov := clients[0].(map[string]any)
	if gov["clientId"] != "central-bank-brazil-client" || gov["secret"] != "central-bank-brazil-local-secret" {
		t.Errorf("governance client mismatch: %v", gov)
	}
	if gov["serviceAccountsEnabled"] != true || gov["directAccessGrantsEnabled"] != true {
		t.Errorf("governance client must enable service accounts + direct access grants")
	}
	roles := realm["roles"].(map[string]any)["realm"].([]any)
	if len(roles) < 2 {
		t.Errorf("expected ROLE_GOVERNANCE + ROLE_TREASURY realm roles, got %v", roles)
	}

	// The governance service account must carry ROLE_GOVERNANCE as a realm role, so
	// the client_credentials login token authorizes RequireRole(ROLE_GOVERNANCE)
	// (e.g. approve-kyc). Without this, the Governance Portal approval returns 403.
	users := realm["users"].([]any)
	var govSA map[string]any
	for _, u := range users {
		um := u.(map[string]any)
		if um["serviceAccountClientId"] == "central-bank-brazil-client" {
			govSA = um
			break
		}
	}
	if govSA == nil {
		t.Fatal("governance service-account user not found in realm import")
	}
	realmRoles, _ := govSA["realmRoles"].([]any)
	hasGov := false
	for _, r := range realmRoles {
		if r == "ROLE_GOVERNANCE" {
			hasGov = true
		}
	}
	if !hasGov {
		t.Errorf("governance service account must have realmRoles=[ROLE_GOVERNANCE], got %v", govSA["realmRoles"])
	}
}
