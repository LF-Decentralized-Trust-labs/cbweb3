// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
)

// Options configures the server interceptors.
type Options struct {
	// Authenticator establishes the caller identity. Defaults to
	// HeaderAuthenticator{} when nil (transitional, pre-mTLS).
	Authenticator Authenticator
	// Policy authorizes the authenticated identity per method. Defaults to
	// AllowAuthenticated{} when nil.
	Policy Policy
	// Enforce controls failure behaviour:
	//   - true  (production): authentication or authorization failures reject the
	//     RPC with the corresponding gRPC status.
	//   - false (audit / transitional): failures are tolerated; the call proceeds
	//     WITHOUT an authenticated identity in context. Existing insecure callers
	//     keep working during rollout, and actor spoofing is still curtailed
	//     because handlers derive the audit actor from the context identity (empty
	//     here) instead of the request payload.
	Enforce bool
	// Anonymous disables caller authentication entirely. It is the transitional
	// default when neither mTLS nor header identity is configured: the interceptors
	// pass the call through without establishing an identity, so audit actors derive
	// from the (gateway-validated) request payload. Never combine with Enforce.
	Anonymous bool
	// Logger, when set, records denied and audit-tolerated events. Optional.
	Logger *slog.Logger
}

func (o Options) authenticator() Authenticator {
	if o.Authenticator != nil {
		return o.Authenticator
	}
	return HeaderAuthenticator{}
}

func (o Options) policy() Policy {
	if o.Policy != nil {
		return o.Policy
	}
	return AllowAuthenticated{}
}

// authenticate runs authentication + authorization and returns the context to
// pass to the handler. In enforce mode a failure returns a non-nil error and the
// RPC must be rejected. In audit mode a failure returns the original context and
// a nil error (the call proceeds unauthenticated).
func (o Options) authenticate(ctx context.Context, fullMethod string) (context.Context, error) {
	// Transitional anonymous mode: no authenticator is configured. Pass through
	// without establishing an identity (audit actors fall back to the payload).
	if o.Anonymous {
		return ctx, nil
	}
	id, authnErr := o.authenticator().Authenticate(ctx)
	if authnErr != nil {
		// Could not establish WHO is calling. There is no identity to carry, so in
		// audit mode the call proceeds with none and audit actors fall back to the
		// gateway-validated payload.
		if o.Enforce {
			if o.Logger != nil {
				o.Logger.Warn("grpc authz: rejected call", "method", fullMethod, "error", authnErr)
			}
			return ctx, authnErr
		}
		if o.Logger != nil {
			o.Logger.Warn("grpc authz: proceeding in audit mode despite failure",
				"method", fullMethod, "error", authnErr)
		}
		return ctx, nil
	}

	if authzErr := o.policy().Authorize(ctx, id, fullMethod); authzErr != nil {
		if o.Enforce {
			if o.Logger != nil {
				o.Logger.Warn("grpc authz: rejected call", "method", fullMethod, "error", authzErr)
			}
			return ctx, authzErr
		}
		// Audit mode, authorization failed — but authentication did NOT: we know
		// exactly who this is. Carry the identity anyway so the audit trail keeps
		// attributing the call to the real caller, and record what would have been
		// refused under enforcement. Dropping it here would make a per-method policy
		// degrade attribution during the very transition it is meant to prepare:
		// every call the policy does not list would fall back to the payload actor.
		if o.Logger != nil {
			o.Logger.Warn("grpc authz: proceeding in audit mode despite failure",
				"method", fullMethod, "subject", id.Subject, "error", authzErr)
		}
		return NewContext(ctx, id), nil
	}
	if o.Logger != nil {
		// Record how the caller was authenticated so a header-asserted actor is
		// distinguishable from an mTLS-attested one in the audit/access logs.
		o.Logger.Info("grpc authz: authenticated call",
			"rpc", fullMethod, "subject", id.Subject, "auth_method", id.Method)
	}
	return NewContext(ctx, id), nil
}

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that authenticates
// and authorizes the caller before dispatching to the handler, injecting the
// authenticated identity into the handler context.
func UnaryServerInterceptor(o Options) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := o.authenticate(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// StreamServerInterceptor returns a grpc.StreamServerInterceptor with the same
// semantics as UnaryServerInterceptor. The authenticated identity is injected
// into the stream's context.
func StreamServerInterceptor(o Options) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := o.authenticate(ss.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: newCtx})
	}
}

// wrappedStream overrides Context so downstream handlers observe the injected
// authenticated identity.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
