// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"regexp"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
)

// Keycloak password grant expects the user's login *username*. Clients often send
// the onboarding userId (UUID) as clientId; we resolve it the same way as PKI
// (IssueLoginNonce) when the value looks like a UUID.
var keycloakUserUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func looksLikeKeycloakUserUUID(s string) bool {
	return keycloakUserUUIDPattern.MatchString(strings.TrimSpace(s))
}

// resolveLoginUsernameForKeycloak maps a UUID user id to the Keycloak username
// when possible. On any failure (not a UUID, admin API error, user not found),
// the original string is returned so Keycloak can still authenticate literal
// usernames (e.g. tryout-noc, bank-a).
func resolveLoginUsernameForKeycloak(ctx context.Context, kc keycloak.Client, user string) string {
	u := strings.TrimSpace(user)
	if u == "" || !looksLikeKeycloakUserUUID(u) {
		return u
	}
	adminToken, err := kc.GetAdminToken(ctx)
	if err != nil || strings.TrimSpace(adminToken) == "" {
		return u
	}
	un, err := kc.GetUserUsername(ctx, adminToken, u)
	if err != nil {
		return u
	}
	un = strings.TrimSpace(un)
	if un == "" {
		return u
	}
	return un
}
