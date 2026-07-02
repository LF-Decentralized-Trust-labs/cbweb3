// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"encoding/json"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

func TestCentralBankRealmPlans_RoutesAdminUsersByRole(t *testing.T) {
	admins := []manifest.AdminUser{
		{Role: "ROLE_GOVERNANCE", Username: "admin@brasil.governance.gov", Password: "gov-pw"},
		{Role: "ROLE_TREASURY", Username: "admin@brasil.treasury.gov", Password: "trez-pw"},
		{Role: "ROLE_SUPERVISOR", Username: "admin@brasil.supervisor.gov", Password: "sup-pw"},
		{Role: "ROLE_NOC_ADMIN", Username: "admin@brasil.noc.gov", Password: "noc-pw"},
	}
	plans := centralBankRealmPlans("central-bank-brazil", admins)
	cb, noc := plans[0], plans[1]

	// Governance + treasury + supervisor admins land in the central-bank realm; the
	// NOC admin lands in the shared cbweb3 realm.
	if len(cb.Users) != 3 {
		t.Fatalf("central-bank realm: want 3 admin users, got %d", len(cb.Users))
	}
	var supervisorFound bool
	for _, u := range cb.Users {
		if u.Username == "admin@brasil.supervisor.gov" {
			supervisorFound = len(u.Roles) == 1 && u.Roles[0] == "ROLE_SUPERVISOR"
		}
	}
	if !supervisorFound {
		t.Errorf("central-bank realm: ROLE_SUPERVISOR admin not routed correctly: %+v", cb.Users)
	}
	if len(noc.Users) != 1 || noc.Users[0].Username != "admin@brasil.noc.gov" {
		t.Errorf("NOC realm admin user mismatch: %+v", noc.Users)
	}

	// The rendered governance admin carries a non-temporary password credential
	// and the ROLE_GOVERNANCE realm role.
	data, err := renderRealmJSON(cb)
	if err != nil {
		t.Fatalf("renderRealmJSON: %v", err)
	}
	var realm map[string]any
	if err := json.Unmarshal(data, &realm); err != nil {
		t.Fatalf("invalid realm JSON: %v", err)
	}
	var gov map[string]any
	for _, u := range realm["users"].([]any) {
		um := u.(map[string]any)
		if um["username"] == "admin@brasil.governance.gov" {
			gov = um
		}
	}
	if gov == nil {
		t.Fatal("governance admin user not rendered")
	}
	cred := gov["credentials"].([]any)[0].(map[string]any)
	if cred["type"] != "password" || cred["value"] != "gov-pw" || cred["temporary"] != false {
		t.Errorf("password credential mismatch: %v", cred)
	}
	roles := gov["realmRoles"].([]any)
	if len(roles) != 1 || roles[0] != "ROLE_GOVERNANCE" {
		t.Errorf("realmRoles = %v; want [ROLE_GOVERNANCE]", roles)
	}
	// Keycloak 26's declarative user profile requires firstName/lastName and a
	// verified email; without them the password grant fails with
	// "Account is not fully set up".
	if gov["emailVerified"] != true {
		t.Errorf("emailVerified = %v; want true", gov["emailVerified"])
	}
	if gov["firstName"] == "" || gov["firstName"] == nil {
		t.Errorf("firstName must be set, got %v", gov["firstName"])
	}
	if gov["lastName"] == "" || gov["lastName"] == nil {
		t.Errorf("lastName must be set, got %v", gov["lastName"])
	}
	if _, ok := gov["requiredActions"]; !ok {
		t.Errorf("requiredActions must be present (empty) to avoid default actions blocking login")
	}
}

func TestCommercialBankRealmPlan_AdminUser(t *testing.T) {
	admins := []manifest.AdminUser{
		{Role: "ROLE_BANK", Username: "admin@itau.brasil.com", Password: "bank-pw"},
	}
	p := commercialBankRealmPlan("bank-itau", admins)
	if len(p.Users) != 1 || p.Users[0].Username != "admin@itau.brasil.com" {
		t.Fatalf("bank realm admin user mismatch: %+v", p.Users)
	}
	if len(p.Users[0].Roles) != 1 || p.Users[0].Roles[0] != "ROLE_BANK" {
		t.Errorf("bank admin roles = %v; want [ROLE_BANK]", p.Users[0].Roles)
	}
}

func TestCentralBankRealmPlans_IncludeNOCAndTreasury(t *testing.T) {
	plans := centralBankRealmPlans("central-bank-brazil", nil)
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
	p := commercialBankRealmPlan("bank-itau", nil)
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
	plans := centralBankRealmPlans("central-bank-brazil", nil)
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
