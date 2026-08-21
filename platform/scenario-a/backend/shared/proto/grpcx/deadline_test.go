// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The interceptor is a BACKSTOP, not a policy override. These tests pin the three
// properties that distinction implies, in the order they matter:
//
//  1. a call arriving with no deadline gets one (the defect this closes);
//  2. a call arriving WITH a deadline keeps it, shorter or longer (a caller that
//     has thought about its own bound must win, or the backstop becomes a bug);
//  3. the added deadline is released when the call returns, rather than leaking a
//     timer per RPC for the length of the default.

// captureInvoker records the context the interceptor hands to the transport.
func captureInvoker(seen *context.Context) grpc.UnaryInvoker {
	return func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		*seen = ctx
		return nil
	}
}

func TestAddsDeadlineWhenCallerHasNone(t *testing.T) {
	var seen context.Context
	icept := WithDefaultDeadline(2 * time.Second)

	if err := icept(context.Background(), "/svc/M", nil, nil, nil, captureInvoker(&seen)); err != nil {
		t.Fatalf("interceptor returned %v, want nil", err)
	}

	dl, ok := seen.Deadline()
	if !ok {
		t.Fatal("invoker received a context with no deadline; the backstop did not apply")
	}
	if d := time.Until(dl); d <= 0 || d > 2*time.Second {
		t.Errorf("deadline is %v away, want (0, 2s]", d)
	}
}

func TestCallerDeadlineIsPreserved(t *testing.T) {
	// A caller that set a TIGHTER bound than the backstop must keep it, and a caller
	// that deliberately set a LOOSER one must keep that too: the interceptor's job is
	// to cover the unbounded case, not to impose a ceiling on a considered choice.
	for _, tc := range []struct {
		name        string
		callerBound time.Duration
		backstop    time.Duration
	}{
		{"caller tighter than backstop", 1 * time.Second, 30 * time.Second},
		{"caller looser than backstop", 60 * time.Second, 5 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.callerBound)
			defer cancel()
			want, _ := ctx.Deadline()

			var seen context.Context
			icept := WithDefaultDeadline(tc.backstop)
			if err := icept(ctx, "/svc/M", nil, nil, nil, captureInvoker(&seen)); err != nil {
				t.Fatalf("interceptor returned %v, want nil", err)
			}

			got, ok := seen.Deadline()
			if !ok {
				t.Fatal("invoker received no deadline at all")
			}
			if !got.Equal(want) {
				t.Errorf("deadline was rewritten: got %v, want the caller's %v", got, want)
			}
		})
	}
}

func TestHangingCallIsCutOffAtTheDeadline(t *testing.T) {
	// The point of the whole card: a sibling service that accepts the connection and
	// then stops answering must not pin the caller forever.
	icept := WithDefaultDeadline(80 * time.Millisecond)
	hang := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		<-ctx.Done()
		return status.FromContextError(ctx.Err()).Err()
	}

	start := time.Now()
	err := icept(context.Background(), "/svc/M", nil, nil, nil, hang)
	elapsed := time.Since(start)

	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("got %v (code %s), want DeadlineExceeded", err, status.Code(err))
	}
	if elapsed > time.Second {
		t.Errorf("took %v to give up; the deadline was 80ms", elapsed)
	}
}

func TestAddedDeadlineIsReleasedOnReturn(t *testing.T) {
	// Without `defer cancel()` every RPC leaks a timer that stays armed for the whole
	// default — 30s or 180s of retained context per call under load. Proven by
	// observing that the context handed to the invoker is done once the call returns.
	var seen context.Context
	icept := WithDefaultDeadline(1 * time.Hour)
	if err := icept(context.Background(), "/svc/M", nil, nil, nil, captureInvoker(&seen)); err != nil {
		t.Fatalf("interceptor returned %v, want nil", err)
	}

	if seen.Err() == nil {
		t.Fatal("context handed to the invoker is still live after the call returned; cancel was not deferred")
	}
	if !errors.Is(seen.Err(), context.Canceled) {
		t.Errorf("context ended with %v, want context.Canceled", seen.Err())
	}
}

func TestInvokerErrorIsPropagatedUnchanged(t *testing.T) {
	sentinel := errors.New("transport exploded")
	icept := WithDefaultDeadline(time.Second)
	err := icept(context.Background(), "/svc/M", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v, want the invoker's own error", err)
	}
}

func TestNonPositiveDeadlineIsANoOp(t *testing.T) {
	// A misconfigured zero must not turn every call into an instant DeadlineExceeded,
	// which would be a far worse failure than the unbounded call it replaced.
	for _, d := range []time.Duration{0, -time.Second} {
		var seen context.Context
		icept := WithDefaultDeadline(d)
		if err := icept(context.Background(), "/svc/M", nil, nil, nil, captureInvoker(&seen)); err != nil {
			t.Fatalf("d=%v: interceptor returned %v, want nil", d, err)
		}
		if _, ok := seen.Deadline(); ok {
			t.Errorf("d=%v: a non-positive default must not set a deadline", d)
		}
	}
}
