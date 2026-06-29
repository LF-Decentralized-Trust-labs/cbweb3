// SPDX-License-Identifier: Apache-2.0

package orchestrator

import "encoding/json"

// (see renderRealmJSON below for the Keycloak realm-import document.)

// keycloak.go derives the per-entity Keycloak provisioning plan (feature 034):
// realm name, confidential service-account client (id + fixed local secret +
// directAccessGrants), realm roles, and the governance service-account user id.
// Mirrors what deploy/local/keycloak/init.sh provisions for the reference
// entities, but parametrized — without reusing/altering init.sh.
//
// Strategy (FR-001b): NOC and the Governance portal always authenticate against
// the central bank's Keycloak instance. So the CB's Keycloak hosts the
// central-bank realm AND the `cbweb3` realm (NOC). Commercial banks use the CB's
// instance in `shared` mode, or their own in `per-entity` mode.

// KeycloakClientPlan describes one confidential service-account client.
type KeycloakClientPlan struct {
	ClientID string
	Secret   string
	Roles    []string
}

// KeycloakRealmPlan describes one realm and its clients/roles to provision.
type KeycloakRealmPlan struct {
	Realm   string
	Clients []KeycloakClientPlan
}

// nocRealmPlan is the NOC realm hosted on the central bank's Keycloak.
func nocRealmPlan() KeycloakRealmPlan {
	return KeycloakRealmPlan{
		Realm: "cbweb3",
		Clients: []KeycloakClientPlan{
			{ClientID: "noc-portal", Secret: "", Roles: []string{"noc-viewer", "noc-operator", "noc-admin"}},
		},
	}
}

// centralBankRealmPlans returns the realms the CB Keycloak must host: the
// central-bank realm (governance + treasury clients) plus the NOC realm.
func centralBankRealmPlans(entity string) []KeycloakRealmPlan {
	return []KeycloakRealmPlan{
		{
			Realm: entity,
			Clients: []KeycloakClientPlan{
				{ClientID: entity + "-client", Secret: entity + "-local-secret", Roles: []string{"ROLE_GOVERNANCE"}},
				{ClientID: entity + "-treasury-client", Secret: entity + "-treasury-local-secret", Roles: []string{"ROLE_TREASURY"}},
			},
		},
		nocRealmPlan(),
	}
}

// commercialBankRealmPlan returns the realm/client for a commercial bank.
func commercialBankRealmPlan(entity string) KeycloakRealmPlan {
	return KeycloakRealmPlan{
		Realm: entity,
		Clients: []KeycloakClientPlan{
			{ClientID: entity + "-client", Secret: entity + "-local-secret", Roles: []string{"ROLE_BANK"}},
		},
	}
}

// governanceUserID is the service-account user the compliance service uses to
// register the governance participant on first startup (CB only).
func governanceUserID(entity string) string {
	return "service-account-" + entity + "-client"
}

// renderRealmJSON produces a Keycloak realm-import document for a plan: the realm,
// its realm roles, and confidential service-account clients (fixed secret,
// directAccessGrants for ROPC). Keycloak imports this on startup (--import-realm).
func renderRealmJSON(plan KeycloakRealmPlan) ([]byte, error) {
	roleSet := map[string]bool{}
	var realmRoles []map[string]any
	clients := make([]map[string]any, 0, len(plan.Clients))
	for _, c := range plan.Clients {
		for _, r := range c.Roles {
			if !roleSet[r] {
				roleSet[r] = true
				realmRoles = append(realmRoles, map[string]any{"name": r})
			}
		}
		client := map[string]any{
			"clientId":                  c.ClientID,
			"name":                      c.ClientID,
			"enabled":                   true,
			"protocol":                  "openid-connect",
			"publicClient":              c.Secret == "",
			"standardFlowEnabled":       true,
			"directAccessGrantsEnabled": true,
			"serviceAccountsEnabled":    c.Secret != "",
			"redirectUris":              []string{"*"},
			"webOrigins":                []string{"*"},
		}
		if c.Secret != "" {
			client["secret"] = c.Secret
		}
		clients = append(clients, client)
	}
	realm := map[string]any{
		"realm":   plan.Realm,
		"enabled": true,
		"roles":   map[string]any{"realm": realmRoles},
		"clients": clients,
	}
	return json.MarshalIndent(realm, "", "  ")
}
