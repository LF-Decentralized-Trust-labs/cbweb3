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
// payload or a transport header. These tests lock the derivation priority:
// authenticated identity > gateway-validated payload actor. The legacy
// x-actor-subject header was removed (R2-H-8 follow-up): it was writable by any peer
// that could reach the port, so it let an unauthenticated caller choose the name
// written to the audit trail.

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

// The header is not merely outranked, it is ignored. Before this, a caller that set
// x-actor-subject and authenticated as nobody had its chosen name recorded as the
// audit actor.
func TestActorFromCtx_IgnoresTheLegacyActorHeader(t *testing.T) {
	md := metadata.New(map[string]string{"x-actor-subject": "spoofed-header"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	if got := actorFromCtx(ctx); got != "" {
		t.Fatalf("the legacy actor header must be ignored, got %q", got)
	}
}

// And it must not sneak back in through the audit derivation: with no authenticated
// identity the actor comes from the gateway-validated payload, not the transport.
func TestActorForAudit_IgnoresTheLegacyActorHeader(t *testing.T) {
	md := metadata.New(map[string]string{"x-actor-subject": "spoofed-header"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	if got := actorForAudit(ctx, "payload-actor"); got != "payload-actor" {
		t.Fatalf("expected the payload actor, got %q", got)
	}
}
