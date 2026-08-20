// SPDX-License-Identifier: Apache-2.0

// Package grpcx holds gRPC client plumbing shared by the services in this
// scenario. It is deliberately separate from authz: a deadline backstop is not an
// authorization concern, even though both are applied at the same dial sites.
package grpcx

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// WithDefaultDeadline returns a unary client interceptor that bounds any call
// arriving without a deadline of its own.
//
// Why this exists. The api-gateway's gRPC adapters bound only the DIAL:
// `context.WithTimeout(context.Background(), timeout)` is spent on
// grpc.DialContext, and the caller's context is then passed straight through to
// every RPC. What a Fiber handler passes is c.Context() or c.UserContext(),
// neither of which carries a deadline, and there is no request-timeout middleware
// in the chain. Fiber's ReadTimeout/WriteTimeout do not help: fasthttp applies
// WriteTimeout to writing the response, not to the handler's duration. So a
// sibling service that accepts the connection and then stops answering pins the
// handler goroutine indefinitely — the connection is bounded, the work behind it
// is not.
//
// Why an interceptor rather than editing each method. It covers methods that do
// not exist yet. A per-method edit bounds today's call sites and silently misses
// the next one added, which is the same class of gap this closes.
//
// It is a BACKSTOP, not a policy override: a caller that already set a deadline
// keeps it, whether tighter or looser than d. A caller that bounded itself has
// made a considered choice, and a "safety" net that overrode it would be a bug
// rather than a guard. d <= 0 disables the interceptor rather than making every
// call fail instantly, because a misconfigured zero must not be worse than the
// unbounded call it replaced.
func WithDefaultDeadline(d time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if d <= 0 {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		if _, ok := ctx.Deadline(); ok {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
