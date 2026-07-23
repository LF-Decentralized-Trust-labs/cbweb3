// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	"google.golang.org/grpc/metadata"
)

// R2-H-8 regression: the audit actor must be derived from the authenticated
// caller identity, never from a value the caller can place in the request
// payload or a legacy header. These tests lock the derivation priority:
// authenticated identity > x-actor-subject header > payload actor.

func TestActorForAudit_PrefersAuthenticatedIdentityOverPayload(t *testing.T) {
	// An attacker names a privileged actor in the request body; the authenticated
	// identity (e.g. from the mTLS peer certificate) must win.
	ctx := authz.NewContext(context.Background(), &authz.Identity{Subject: "bank-a", Method: "mtls"})
	if got := actorForAudit(ctx, "central-bank-spoofed"); got != "bank-a" {
		t.Fatalf("expected authenticated actor %q, got %q", "bank-a", got)
	}
}

func TestActorForAudit_FallsBackToPayloadOnlyWhenUnauthenticated(t *testing.T) {
	// Transitional path (pre-mTLS): with no authenticated identity the payload
	// actor is used. Documented as spoofable until enforcement is enabled.
	if got := actorForAudit(context.Background(), "payload-actor"); got != "payload-actor" {
		t.Fatalf("expected payload fallback %q, got %q", "payload-actor", got)
	}
}

func TestActorFromCtx_AuthenticatedIdentityBeatsHeader(t *testing.T) {
	md := metadata.New(map[string]string{"x-actor-subject": "spoofed-header"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ctx = authz.NewContext(ctx, &authz.Identity{Subject: "authenticated-bank"})
	if got := actorFromCtx(ctx); got != "authenticated-bank" {
		t.Fatalf("expected authenticated identity to win, got %q", got)
	}
}
