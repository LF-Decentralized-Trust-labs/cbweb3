// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Policy authorizes an authenticated identity for a specific gRPC method. It runs
// after authentication; id is the identity the Authenticator established.
type Policy interface {
	Authorize(ctx context.Context, id *Identity, fullMethod string) error
}

// AllowAuthenticated is the baseline policy: it rejects anonymous callers but
// places no per-method restriction on an authenticated caller. Services that need
// finer control (e.g. only the payment-orchestrator signer may mint) supply their
// own Policy or an AllowList.
type AllowAuthenticated struct{}

// Authorize implements Policy.
func (AllowAuthenticated) Authorize(_ context.Context, id *Identity, _ string) error {
	if id == nil || id.Subject == "" {
		return status.Error(codes.Unauthenticated, "caller is not authenticated")
	}
	return nil
}

// AllowList authorizes only callers whose Subject is present (and true) in the
// set. Use it to restrict sensitive services to their known callers — for
// example, restricting the compliance and payment-orchestrator services to the
// api-gateway identity.
type AllowList struct {
	Subjects map[string]bool
}

// Authorize implements Policy.
func (a AllowList) Authorize(_ context.Context, id *Identity, fullMethod string) error {
	if id == nil || id.Subject == "" {
		return status.Error(codes.Unauthenticated, "caller is not authenticated")
	}
	if !a.Subjects[id.Subject] {
		return status.Errorf(codes.PermissionDenied, "caller %q is not authorized to call %s", id.Subject, fullMethod)
	}
	return nil
}

// MethodPolicy applies a per-method Policy and falls back to Default for anything
// not listed. It is the answer to "restrict each server to its known callers, not
// merely any authenticated peer" (R2-H-8 follow-up): a service-wide AllowList is
// too coarse, because the same server exposes reads that many callers need and
// value-moving writes that exactly one caller may perform.
//
// Keys are full gRPC method names ("/pkg.Service/Method"). Call sites build them
// from the generated *_FullMethodName constants rather than string literals, so a
// renamed RPC breaks the build instead of silently leaving a method unprotected.
//
// Default MUST NOT be nil: a nil default on an unlisted method would authorize it
// with no check at all, which is the failure mode this type exists to remove.
type MethodPolicy struct {
	Default  Policy
	ByMethod map[string]Policy
}

// Authorize implements Policy.
func (m MethodPolicy) Authorize(ctx context.Context, id *Identity, fullMethod string) error {
	if p, ok := m.ByMethod[fullMethod]; ok && p != nil {
		return p.Authorize(ctx, id, fullMethod)
	}
	if m.Default == nil {
		// Fail closed. Reaching here means the policy was constructed without a
		// default, which is a programming error; refusing is the only safe answer.
		return status.Errorf(codes.PermissionDenied,
			"no policy configured for %s (MethodPolicy.Default is nil)", fullMethod)
	}
	return m.Default.Authorize(ctx, id, fullMethod)
}

// RestrictMethods returns a MethodPolicy that admits only the named subjects on
// each of the given methods, and applies base to every other method.
//
// The subject list is the set of services that legitimately call the method,
// established by reading the call sites — not a configurable value. These are mTLS
// certificate CNs issued by the toolkit's service-mesh CA, whose names are the
// logical service names ("api-gateway", "auth", "compliance",
// "payment-orchestrator"), so the expected caller of a given RPC is known at
// compile time and needs no operator action to be enforced.
func RestrictMethods(base Policy, subjects []string, methods ...string) MethodPolicy {
	allowed := make(map[string]bool, len(subjects))
	for _, s := range subjects {
		allowed[s] = true
	}
	byMethod := make(map[string]Policy, len(methods))
	for _, m := range methods {
		byMethod[m] = AllowList{Subjects: allowed}
	}
	return MethodPolicy{Default: base, ByMethod: byMethod}
}

// WithRestriction adds another per-method restriction to an existing MethodPolicy,
// so a server can declare several caller sets (for example: only the gateway may
// approve KYC, but the gateway and the auth service may both sign a CSR).
func (m MethodPolicy) WithRestriction(subjects []string, methods ...string) MethodPolicy {
	allowed := make(map[string]bool, len(subjects))
	for _, s := range subjects {
		allowed[s] = true
	}
	// Copy, do not alias: the value receiver copies the struct but not the map behind
	// it, so writing through m.ByMethod would edit the policy the caller derived this
	// one from. Chained calls in a serverPolicy builder read as pure, and one built
	// from another service's base must not be able to widen it.
	byMethod := make(map[string]Policy, len(m.ByMethod)+len(methods))
	for k, v := range m.ByMethod {
		byMethod[k] = v
	}
	for _, method := range methods {
		byMethod[method] = AllowList{Subjects: allowed}
	}
	m.ByMethod = byMethod
	return m
}
