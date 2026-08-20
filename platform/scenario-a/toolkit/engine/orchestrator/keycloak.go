// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"encoding/json"
	"fmt"
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

// Audience values stamped into access tokens via an oidc-audience-mapper so the
// backends can enforce the "aud" claim (KEYCLOAK_AUDIENCE). Backend login clients
// (governance/treasury/bank) carry keycloakBackendAudience, which the api-gateway
// auth service expects; the NOC client carries keycloakNOCAudience. iss is already
// deterministic — the auth service password-grants server-side against its
// configured KEYCLOAK_BASE_URL, the exact host Keycloak stamps into iss.
const (
	keycloakBackendAudience = "cbweb3-backend"
	keycloakNOCAudience     = "cbweb3-noc"
)

// KeycloakClientPlan describes one confidential service-account client.
type KeycloakClientPlan struct {
	ClientID string
	Secret   string
	Roles    []string
	// Audience, when non-empty, adds an oidc-audience-mapper stamping this value
	// into the client's access tokens (so a backend can enforce KEYCLOAK_AUDIENCE).
	Audience string
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
	// Environment is the manifest's spec.environment ("local" | "staging" | "prod").
	// It decides sslRequired: "none" is a local-only affordance, since a developer
	// reaches Keycloak and the portals over plain HTTP. An empty value is treated as
	// NOT local — the permissive path must be asked for, never fallen into.
	Environment string
	// Origins are the browser origins this entity serves its portals from — the same
	// list the api-gateway receives as CORS_ALLOW_ORIGINS, so the two cannot drift.
	// They become the clients' webOrigins, and their "/*" forms the redirectUris.
	// Required: renderRealmJSON refuses a plan without them rather than falling back
	// to a wildcard, which is the defect this replaces (finding R1-10.7).
	Origins []string
}

// adminUsersForRealmRoles selects the manifest admin users whose role is one of
// the realm's roles, mapping them to KeycloakUserPlans. This routes each admin
// to the realm that actually defines its role (central-bank realm for
// ROLE_GOVERNANCE/ROLE_TREASURY/ROLE_SUPERVISOR, the cbweb3/NOC realm for
// ROLE_NOC_*, the bank realm for ROLE_BANK).
//
// Entries are grouped by username so a single operator granted several roles
// (multiple spec.adminUsers entries sharing one username) becomes ONE
// realm-import user carrying all of its roles that belong to THIS realm. Keycloak
// realm import requires usernames unique per realm, so emitting one plan per role
// entry would produce a duplicate-username collision; grouping also correctly
// gives the account every role it was granted. First-seen order is preserved
// (stable output), and the first occurrence's password wins (the validator
// rejects a username that repeats with a different password).
func adminUsersForRealmRoles(admins []manifest.AdminUser, realmRoles ...string) []KeycloakUserPlan {
	want := map[string]bool{}
	for _, r := range realmRoles {
		want[r] = true
	}
	order := []string{}
	byUser := map[string]*KeycloakUserPlan{}
	for _, a := range admins {
		role := strings.TrimSpace(a.Role)
		if !want[role] {
			continue
		}
		u, ok := byUser[a.Username]
		if !ok {
			u = &KeycloakUserPlan{Username: a.Username, Password: a.Password}
			byUser[a.Username] = u
			order = append(order, a.Username)
		}
		if !containsString(u.Roles, role) {
			u.Roles = append(u.Roles, role)
		}
	}
	users := make([]KeycloakUserPlan, 0, len(order))
	for _, name := range order {
		users = append(users, *byUser[name])
	}
	return users
}

// containsString reports whether s is already present in xs (small slices — a
// linear scan keeps role de-duplication order-preserving without a set).
func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// nocRealmPlan is the NOC realm hosted on the central bank's Keycloak.
// The client ID must match the VITE_KEYCLOAK_CLIENT_ID baked into the NOC
// frontend at build time (see orchestrator.go: KeycloakClient: "cbweb3-noc").
func nocRealmPlan(environment string, origins []string) KeycloakRealmPlan {
	return KeycloakRealmPlan{
		Realm:       "cbweb3",
		Environment: environment,
		Origins:     origins,
		Clients: []KeycloakClientPlan{
			{ClientID: "cbweb3-noc", Secret: "", Roles: []string{"ROLE_NOC_VIEWER", "ROLE_NOC_OPERATOR", "ROLE_NOC_ADMIN"}, Audience: keycloakNOCAudience},
		},
	}
}

// centralBankRealmPlans returns the realms the CB Keycloak must host: the
// central-bank realm (governance + treasury clients) plus the NOC realm. The
// manifest's admin users are routed to the realm that defines their role:
// ROLE_GOVERNANCE/ROLE_TREASURY/ROLE_SUPERVISOR into the central-bank realm,
// ROLE_NOC_ADMIN into the shared cbweb3/NOC realm.
func centralBankRealmPlans(entity string, admins []manifest.AdminUser, environment string, origins []string) []KeycloakRealmPlan {
	noc := nocRealmPlan(environment, origins)
	noc.Users = adminUsersForRealmRoles(admins, "ROLE_NOC_ADMIN", "ROLE_NOC_OPERATOR", "ROLE_NOC_VIEWER")
	return []KeycloakRealmPlan{
		{
			Realm:       entity,
			Environment: environment,
			Origins:     origins,
			Clients: []KeycloakClientPlan{
				{ClientID: entity + "-client", Secret: entity + "-local-secret", Roles: []string{"ROLE_GOVERNANCE"}, Audience: keycloakBackendAudience},
				{ClientID: entity + "-treasury-client", Secret: entity + "-treasury-local-secret", Roles: []string{"ROLE_TREASURY"}, Audience: keycloakBackendAudience},
			},
			Users: adminUsersForRealmRoles(admins, "ROLE_GOVERNANCE", "ROLE_TREASURY", "ROLE_SUPERVISOR"),
		},
		noc,
	}
}

// commercialBankRealmPlan returns the realm/client for a commercial bank, with
// the manifest's ROLE_BANK admin user provisioned for portal login.
func commercialBankRealmPlan(entity string, admins []manifest.AdminUser, environment string, origins []string) KeycloakRealmPlan {
	return KeycloakRealmPlan{
		Realm:       entity,
		Environment: environment,
		Origins:     origins,
		Clients: []KeycloakClientPlan{
			{ClientID: entity + "-client", Secret: entity + "-local-secret", Roles: []string{"ROLE_BANK"}, Audience: keycloakBackendAudience},
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
	// Fail closed. An empty origin list used to render as `["*"]`, which is an open
	// redirector for the authorization code and lets any page read token responses.
	// A plan that reaches here without origins is a provisioning bug, and the right
	// answer is to stop rather than to publish the permissive document.
	if len(plan.Origins) == 0 {
		return nil, fmt.Errorf("keycloak realm %q: no browser origins in the plan; "+
			"redirectUris/webOrigins would fall back to a wildcard", plan.Realm)
	}
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
			"redirectUris":              redirectURIsFor(plan.Origins),
			"webOrigins":                append([]string(nil), plan.Origins...),
		}
		// Stamp a fixed "aud" via an audience mapper so the consuming backend can
		// enforce KEYCLOAK_AUDIENCE. Without this Keycloak omits the client id from
		// "aud" and aud enforcement would reject every token (fails closed).
		if c.Audience != "" {
			client["protocolMappers"] = []map[string]any{{
				"name":           "cbweb3-audience",
				"protocol":       "openid-connect",
				"protocolMapper": "oidc-audience-mapper",
				"config": map[string]any{
					"included.custom.audience": c.Audience,
					"access.token.claim":       "true",
					"id.token.claim":           "false",
				},
			}}
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
		// "none" only for a local stack, where Keycloak and the portals are reached
		// over plain HTTP on the developer's machine. Anything else gets Keycloak's
		// own default, "external": TLS demanded on every non-private address. A
		// deployment that terminates TLS in front therefore needs no override; one
		// that serves the portals over plain HTTP on a routable address has to say so
		// by declaring spec.environment: local (finding R1-10.7).
		"sslRequired": sslRequiredFor(plan.Environment),
		"roles":       map[string]any{"realm": realmRoles},
		"clients":     clients,
	}
	if len(users) > 0 {
		realm["users"] = users
	}
	return json.MarshalIndent(realm, "", "  ")
}

// sslRequiredFor maps a manifest environment onto Keycloak's sslRequired. Only an
// explicit "local" gets "none"; every other value — including an empty one — gets
// "external", so a forgotten environment hardens rather than opens.
func sslRequiredFor(environment string) string {
	if environment == "local" {
		return "none"
	}
	return "external"
}

// redirectURIsFor turns portal origins into Keycloak redirect URIs. Keycloak matches
// redirect URIs by path, so each origin contributes "<origin>/*": scoped to the origin
// the portal is actually served from, unlike the "*" this replaces.
func redirectURIsFor(origins []string) []string {
	uris := make([]string, 0, len(origins))
	for _, o := range origins {
		uris = append(uris, strings.TrimRight(o, "/")+"/*")
	}
	return uris
}
