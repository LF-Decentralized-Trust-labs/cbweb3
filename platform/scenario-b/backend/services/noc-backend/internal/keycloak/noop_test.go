// SPDX-License-Identifier: Apache-2.0

package keycloak

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
)

// unsignedToken builds a JWT-shaped string with the given payload and a bogus
// signature — what NOC_SKIP_AUTH deployments receive and never verify.
func unsignedToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(payload) + ".signature"
}

func TestNoOpClientAttributesTheRealOperator(t *testing.T) {
	claims, err := NewNoOp().ValidateToken(context.Background(), unsignedToken(t, map[string]any{
		"sub":                "5b1f-uuid",
		"preferred_username": "admin@brasil.noc.gov",
	}))
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if got, want := claims.Username, "admin@brasil.noc.gov"; got != want {
		t.Errorf("Username = %q, want %q", got, want)
	}
	if got, want := claims.Subject, "5b1f-uuid"; got != want {
		t.Errorf("Subject = %q, want %q", got, want)
	}
	if got, want := claims.Actor(), "admin@brasil.noc.gov"; got != want {
		t.Errorf("Actor() = %q, want %q — the audit trail must name the operator", got, want)
	}
	if len(claims.Roles) == 0 {
		t.Error("Roles is empty; the no-op client must keep granting the NOC roles")
	}
}

func TestNoOpClientFallsBackWhenTheTokenCarriesNoIdentity(t *testing.T) {
	cases := map[string]string{
		"garbage":            "not-a-token",
		"empty":              "",
		"no identity claims": unsignedToken(t, map[string]any{"exp": 1_800_000_000}),
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			claims, err := NewNoOp().ValidateToken(context.Background(), token)
			if err != nil {
				t.Fatalf("ValidateToken: %v", err)
			}
			if got, want := claims.Actor(), devUser; got != want {
				t.Errorf("Actor() = %q, want %q", got, want)
			}
		})
	}
}

func TestActorPrefersUsernameButFallsBackToSubject(t *testing.T) {
	if got, want := (TokenClaims{Subject: "uuid"}).Actor(), "uuid"; got != want {
		t.Errorf("Actor() = %q, want %q", got, want)
	}
	if got, want := (TokenClaims{Subject: "uuid", Username: "  "}).Actor(), "uuid"; got != want {
		t.Errorf("blank username should not win: Actor() = %q, want %q", got, want)
	}
}
