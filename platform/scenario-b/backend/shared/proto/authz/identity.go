// SPDX-License-Identifier: Apache-2.0

// Package authz provides gRPC server interceptors and helpers that authenticate
// the calling service (or user forwarded by the API gateway) and derive the
// audit actor from the authenticated identity rather than from the request
// payload, which any network peer could forge.
//
// Threat model (finding R2-H-8): intra-cluster gRPC is plaintext and has no
// authentication or authorization. Anyone with internal network reach can call
// mint/burn/settle/freeze directly and spoof the "actor" field written to the
// audit log. This package supplies the server-side insertion point for that
// control: an interceptor that (1) establishes the caller identity from the
// transport (mTLS peer certificate) or, transitionally, from a trusted metadata
// header set by the gateway, (2) rejects unauthorized calls, and (3) injects the
// authenticated identity into the handler context so audit actors are derived
// from it — never from the request body.
package authz

import "context"

// Identity is an authenticated caller identity established by an Authenticator.
type Identity struct {
	// Subject uniquely identifies the caller: an mTLS certificate common name
	// (e.g. "payment-orchestrator", "api-gateway") or the Paladin/bank identity
	// forwarded by the API gateway over an authenticated channel.
	Subject string
	// Method records how the identity was established: "mtls" or "header".
	Method string
}

type ctxKey struct{}

// NewContext returns a copy of ctx carrying id as the authenticated identity.
func NewContext(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the authenticated identity stored by an interceptor, or nil
// when the call carried no authenticated identity.
func FromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(ctxKey{}).(*Identity)
	return id
}

// Actor returns the authenticated subject for use as an audit actor, or the empty
// string when the call was not authenticated. Handlers MUST prefer this over any
// actor field carried in the request payload — the payload value is caller
// controlled and therefore spoofable.
func Actor(ctx context.Context) string {
	if id := FromContext(ctx); id != nil {
		return id.Subject
	}
	return ""
}
