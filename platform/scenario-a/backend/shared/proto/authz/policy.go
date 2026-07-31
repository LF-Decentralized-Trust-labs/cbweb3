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
