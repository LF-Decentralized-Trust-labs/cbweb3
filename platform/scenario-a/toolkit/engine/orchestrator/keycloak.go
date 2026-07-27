// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"encoding/json"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

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

// KeycloakUserPlan describes one human operator account (password grant) to
// provision in a realm: the login username (email-style), its password, and the
// realm roles it is granted. Portal/operator login uses these instead of the
// confidential client credentials so audit logs carry a real actor.
type KeycloakUserPlan struct {
	Username string
	Password string
	Roles    []string
}

// KeycloakRealmPlan describes one realm and its clients/roles/users to provision.
type KeycloakRealmPlan struct {
	Realm   string
	Clients []KeycloakClientPlan
	Users   []KeycloakUserPlan
}

// adminUsersForRealmRoles selects the manifest admin users whose role is one of
// the realm's roles, mapping each to a KeycloakUserPlan. This routes each admin
// to the realm that actually defines its role (central-bank realm for
// ROLE_GOVERNANCE/ROLE_TREASURY, the cbweb3/NOC realm for ROLE_NOC_ADMIN, the bank
// realm for ROLE_BANK).
func adminUsersForRealmRoles(admins []manifest.AdminUser, realmRoles ...string) []KeycloakUserPlan {
	want := map[string]bool{}
	for _, r := range realmRoles {
		want[r] = true
	}
	var users []KeycloakUserPlan
	for _, a := range admins {
		if want[strings.TrimSpace(a.Role)] {
			users = append(users, KeycloakUserPlan{
				Username: a.Username,
				Password: a.Password,
				Roles:    []string{a.Role},
			})
		}
	}
	return users
}

// nocRealmPlan is the NOC realm hosted on the central bank's Keycloak.
// The client ID must match the VITE_KEYCLOAK_CLIENT_ID baked into the NOC
// frontend at build time (see orchestrator.go: KeycloakClient: "cbweb3-noc").
func nocRealmPlan() KeycloakRealmPlan {
	return KeycloakRealmPlan{
		Realm: "cbweb3",
		Clients: []KeycloakClientPlan{
			{ClientID: "noc-portal", Secret: "", Roles: []string{"ROLE_NOC_VIEWER", "ROLE_NOC_OPERATOR", "ROLE_NOC_ADMIN"}},
		},
	}
}

// centralBankRealmPlans returns the realms the CB Keycloak must host: the
// central-bank realm (governance + treasury clients) plus the NOC realm. The
// manifest's admin users are routed to the realm that defines their role:
// ROLE_GOVERNANCE/ROLE_TREASURY/ROLE_SUPERVISOR into the central-bank realm,
// ROLE_NOC_ADMIN into the shared cbweb3/NOC realm.
func centralBankRealmPlans(entity string, admins []manifest.AdminUser) []KeycloakRealmPlan {
	noc := nocRealmPlan()
	noc.Users = adminUsersForRealmRoles(admins, "ROLE_NOC_ADMIN", "ROLE_NOC_OPERATOR", "ROLE_NOC_VIEWER")
	return []KeycloakRealmPlan{
		{
			Realm: entity,
			Clients: []KeycloakClientPlan{
				{ClientID: entity + "-client", Secret: entity + "-local-secret", Roles: []string{"ROLE_GOVERNANCE"}},
				{ClientID: entity + "-treasury-client", Secret: entity + "-treasury-local-secret", Roles: []string{"ROLE_TREASURY"}},
			},
			Users: adminUsersForRealmRoles(admins, "ROLE_GOVERNANCE", "ROLE_TREASURY", "ROLE_SUPERVISOR"),
		},
		noc,
	}
}

// commercialBankRealmPlan returns the realm/client for a commercial bank, with
// the manifest's ROLE_BANK admin user provisioned for portal login.
func commercialBankRealmPlan(entity string, admins []manifest.AdminUser) KeycloakRealmPlan {
	return KeycloakRealmPlan{
		Realm: entity,
		Clients: []KeycloakClientPlan{
			{ClientID: entity + "-client", Secret: entity + "-local-secret", Roles: []string{"ROLE_BANK"}},
		},
		Users: adminUsersForRealmRoles(admins, "ROLE_BANK"),
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
	// Service accounts need realm-management roles so the auth service can create
	// and manage users (e.g. provisioning a joining bank's admin user). These are
	// granted via a users entry per the Keycloak realm-import format.
	var users []map[string]any
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
			saUser := map[string]any{
				"username":               "service-account-" + c.ClientID,
				"enabled":                true,
				"serviceAccountClientId": c.ClientID,
				"clientRoles": map[string]any{
					"realm-management": []string{"manage-users", "view-users", "query-users", "manage-clients"},
				},
			}
			// Grant the client's realm roles (e.g. ROLE_GOVERNANCE) to its service
			// account: portal login is a client_credentials grant, so the resulting
			// token's realm_access.roles must carry these for RequireRole to pass
			// (e.g. the Governance Portal's approve-kyc requires ROLE_GOVERNANCE).
			if len(c.Roles) > 0 {
				saUser["realmRoles"] = c.Roles
			}
			users = append(users, saUser)
		}
		clients = append(clients, client)
	}

	// Human operator accounts (password grant). Each lands in this realm with its
	// role(s) granted. Local profile: a non-temporary password read from the
	// manifest. FUTURE: source the password from a secret store and set
	// temporary=true (force reset on first login) for non-local environments.
	for _, u := range plan.Users {
		for _, r := range u.Roles {
			if !roleSet[r] {
				roleSet[r] = true
				realmRoles = append(realmRoles, map[string]any{"name": r})
			}
		}
		lastName := "Operator"
		if len(u.Roles) > 0 {
			lastName = u.Roles[0]
		}
		users = append(users, map[string]any{
			"username":      u.Username,
			"email":         u.Username,
			"emailVerified": true,
			"enabled":       true,
			// firstName/lastName are required by the Keycloak 26 declarative user
			// profile; without them the password grant fails with "Account is not
			// fully set up". emailVerified=true avoids the verify-email required action;
			// requiredActions=[] prevents any realm-default action from blocking login.
			"firstName":       "Admin",
			"lastName":        lastName,
			"requiredActions": []string{},
			"credentials": []map[string]any{
				{"type": "password", "value": u.Password, "temporary": false},
			},
			"realmRoles": u.Roles,
		})
	}

	realm := map[string]any{
		"realm":   plan.Realm,
		"enabled": true,
		// sslRequired "none" allows HTTP access from browsers in local/dev
		// deployments. External (prod) deployments must place a TLS terminator
		// in front; this flag must be changed to "external" or "all" there.
		"sslRequired": "none",
		"roles":       map[string]any{"realm": realmRoles},
		"clients":     clients,
	}
	if len(users) > 0 {
		realm["users"] = users
	}
	return json.MarshalIndent(realm, "", "  ")
}
