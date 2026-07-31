// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// passHandler is a unary handler that records the context it was invoked with and
// returns a sentinel success value.
func passHandler(seen *context.Context) grpc.UnaryHandler {
	return func(ctx context.Context, _ any) (any, error) {
		if seen != nil {
			*seen = ctx
		}
		return "ok", nil
	}
}

func unaryInfo(method string) *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: method}
}

func headerCtx(key, val string) context.Context {
	md := metadata.New(map[string]string{key: val})
	return metadata.NewIncomingContext(context.Background(), md)
}

// --- HeaderAuthenticator -----------------------------------------------------

func TestHeaderAuthenticator_MissingHeaderIsUnauthenticated(t *testing.T) {
	_, err := HeaderAuthenticator{}.Authenticate(context.Background())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestHeaderAuthenticator_ExtractsSubject(t *testing.T) {
	id, err := HeaderAuthenticator{}.Authenticate(headerCtx(HeaderMetadataKey, "bank-a"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Subject != "bank-a" {
		t.Fatalf("expected subject bank-a, got %q", id.Subject)
	}
	if id.Method != "header" {
		t.Fatalf("expected method header, got %q", id.Method)
	}
}

// --- MTLSAuthenticator -------------------------------------------------------

func TestMTLSAuthenticator_NoPeerIsUnauthenticated(t *testing.T) {
	_, err := MTLSAuthenticator{}.Authenticate(context.Background())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestMTLSAuthenticator_UsesVerifiedCertCommonName(t *testing.T) {
	leaf := &x509.Certificate{}
	leaf.Subject.CommonName = "payment-orchestrator"
	tlsInfo := credentials.TLSInfo{State: tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{leaf}},
	}}
	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: tlsInfo})

	id, err := MTLSAuthenticator{}.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Subject != "payment-orchestrator" {
		t.Fatalf("expected subject payment-orchestrator, got %q", id.Subject)
	}
	if id.Method != "mtls" {
		t.Fatalf("expected method mtls, got %q", id.Method)
	}
}

func TestMTLSAuthenticator_NoVerifiedChainIsUnauthenticated(t *testing.T) {
	tlsInfo := credentials.TLSInfo{State: tls.ConnectionState{}}
	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: tlsInfo})
	_, err := MTLSAuthenticator{}.Authenticate(ctx)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

// --- Policy ------------------------------------------------------------------

func TestAllowAuthenticated_RejectsAnonymous(t *testing.T) {
	err := AllowAuthenticated{}.Authorize(context.Background(), nil, "/svc/M")
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAllowList_RejectsUnlistedCaller(t *testing.T) {
	p := AllowList{Subjects: map[string]bool{"api-gateway": true}}
	err := p.Authorize(context.Background(), &Identity{Subject: "attacker"}, "/svc/Mint")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
	if err := p.Authorize(context.Background(), &Identity{Subject: "api-gateway"}, "/svc/Mint"); err != nil {
		t.Fatalf("expected listed caller allowed, got %v", err)
	}
}

// --- Unary interceptor: enforce mode ----------------------------------------

func TestUnaryInterceptor_Enforce_RejectsUnauthenticated(t *testing.T) {
	interceptor := UnaryServerInterceptor(Options{Enforce: true})
	_, err := interceptor(context.Background(), nil, unaryInfo("/svc/Mint"), passHandler(nil))
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated rejection, got %v", err)
	}
}

func TestUnaryInterceptor_Enforce_RejectsUnauthorized(t *testing.T) {
	interceptor := UnaryServerInterceptor(Options{
		Enforce: true,
		Policy:  AllowList{Subjects: map[string]bool{"api-gateway": true}},
	})
	ctx := headerCtx(HeaderMetadataKey, "attacker")
	_, err := interceptor(ctx, nil, unaryInfo("/svc/Mint"), passHandler(nil))
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestUnaryInterceptor_Enforce_AllowsAndInjectsIdentity(t *testing.T) {
	var seen context.Context
	interceptor := UnaryServerInterceptor(Options{Enforce: true})
	ctx := headerCtx(HeaderMetadataKey, "bank-a")
	resp, err := interceptor(ctx, nil, unaryInfo("/svc/Mint"), passHandler(&seen))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("handler not invoked, got %v", resp)
	}
	if got := Actor(seen); got != "bank-a" {
		t.Fatalf("expected actor derived from authenticated identity 'bank-a', got %q", got)
	}
}

// --- Unary interceptor: audit mode (transitional) ---------------------------

func TestUnaryInterceptor_Audit_AllowsUnauthenticatedButNoIdentity(t *testing.T) {
	var seen context.Context
	interceptor := UnaryServerInterceptor(Options{Enforce: false})
	resp, err := interceptor(context.Background(), nil, unaryInfo("/svc/Mint"), passHandler(&seen))
	if err != nil {
		t.Fatalf("audit mode must not reject: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("handler not invoked in audit mode")
	}
	if got := Actor(seen); got != "" {
		t.Fatalf("expected empty actor for unauthenticated audit-mode call, got %q", got)
	}
}

func TestUnaryInterceptor_Audit_InjectsIdentityWhenPresent(t *testing.T) {
	var seen context.Context
	interceptor := UnaryServerInterceptor(Options{Enforce: false})
	ctx := headerCtx(HeaderMetadataKey, "bank-b")
	if _, err := interceptor(ctx, nil, unaryInfo("/svc/M"), passHandler(&seen)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := Actor(seen); got != "bank-b" {
		t.Fatalf("expected actor bank-b, got %q", got)
	}
}

// --- Actor spoof prevention --------------------------------------------------

// The audit actor MUST come from the authenticated identity, never from a value a
// caller could place in the request payload. Here the "payload" claims a
// privileged actor while the authenticated identity is a different subject.
func TestActor_DerivedFromIdentityNotPayload(t *testing.T) {
	const payloadActor = "central-bank" // attacker-supplied in the request body
	ctx := NewContext(context.Background(), &Identity{Subject: "bank-a", Method: "mtls"})

	authenticated := Actor(ctx)
	if authenticated == payloadActor {
		t.Fatalf("actor must not be taken from payload")
	}
	if authenticated != "bank-a" {
		t.Fatalf("expected actor from authenticated identity, got %q", authenticated)
	}
}

// --- Stream interceptor ------------------------------------------------------

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeStream) Context() context.Context { return f.ctx }

func TestStreamInterceptor_Enforce_RejectsUnauthenticated(t *testing.T) {
	interceptor := StreamServerInterceptor(Options{Enforce: true})
	err := interceptor(nil, &fakeStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/svc/Watch"},
		func(any, grpc.ServerStream) error { return nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestStreamInterceptor_InjectsIdentityIntoStreamContext(t *testing.T) {
	var seen context.Context
	interceptor := StreamServerInterceptor(Options{Enforce: true})
	ctx := headerCtx(HeaderMetadataKey, "bank-c")
	err := interceptor(nil, &fakeStream{ctx: ctx},
		&grpc.StreamServerInfo{FullMethod: "/svc/Watch"},
		func(_ any, ss grpc.ServerStream) error { seen = ss.Context(); return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := Actor(seen); got != "bank-c" {
		t.Fatalf("expected stream actor bank-c, got %q", got)
	}
}

// --- Custom authenticator error propagation ---------------------------------

type errAuthenticator struct{ err error }

func (e errAuthenticator) Authenticate(context.Context) (*Identity, error) { return nil, e.err }

func TestUnaryInterceptor_Enforce_PropagatesAuthenticatorError(t *testing.T) {
	sentinel := status.Error(codes.Unauthenticated, "boom")
	interceptor := UnaryServerInterceptor(Options{Enforce: true, Authenticator: errAuthenticator{err: sentinel}})
	_, err := interceptor(context.Background(), nil, unaryInfo("/svc/M"), passHandler(nil))
	if !errors.Is(err, sentinel) && status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected authenticator error propagated, got %v", err)
	}
}

// --- Anonymous (transitional default) mode ----------------------------------

func TestUnaryInterceptor_Anonymous_PassesThroughWithoutIdentity(t *testing.T) {
	var seen context.Context
	interceptor := UnaryServerInterceptor(Options{Anonymous: true})
	// Even with a caller-settable header present, anonymous mode must not derive an
	// identity from it — audit actors fall back to the payload downstream.
	ctx := headerCtx(HeaderMetadataKey, "attacker")
	resp, err := interceptor(ctx, nil, unaryInfo("/svc/Mint"), passHandler(&seen))
	if err != nil {
		t.Fatalf("anonymous mode must not reject: %v", err)
	}
	if resp != "ok" {
		t.Fatal("handler not invoked in anonymous mode")
	}
	if got := Actor(seen); got != "" {
		t.Fatalf("anonymous mode must not inject an identity, got %q", got)
	}
}

// --- ServerOptionsFromEnv posture decisions ---------------------------------

func TestOptionsFromEnv_DefaultIsAnonymousAndUnenforced(t *testing.T) {
	// No env set (the state of every current deployment): no TLS, no authenticator,
	// and enforcement off — the server accepts existing plaintext callers.
	opts, creds, err := optionsFromEnv(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Fatal("expected no TLS credentials by default")
	}
	if !opts.Anonymous {
		t.Fatal("expected anonymous transitional default")
	}
	if opts.Enforce {
		t.Fatal("enforcement must be off by default")
	}
}

func TestOptionsFromEnv_HeaderIdentityIsOptIn(t *testing.T) {
	t.Setenv(EnvAllowHeader, "true")
	opts, creds, err := optionsFromEnv(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Fatal("header identity must not enable TLS")
	}
	if opts.Anonymous {
		t.Fatal("header opt-in must establish an authenticator, not anonymous mode")
	}
	if _, ok := opts.Authenticator.(HeaderAuthenticator); !ok {
		t.Fatalf("expected HeaderAuthenticator, got %T", opts.Authenticator)
	}
}

func TestOptionsFromEnv_PartialMTLSIsRejected(t *testing.T) {
	t.Setenv(EnvCertFile, "/pki/service.crt")
	// KEY and CA unset: a partial config must fail closed, not downgrade to plaintext.
	if _, _, err := optionsFromEnv(nil, nil); err == nil {
		t.Fatal("expected error for partial mTLS configuration")
	}
}

func TestOptionsFromEnv_EnforceWithoutMTLSIsRejected(t *testing.T) {
	t.Setenv(EnvEnforce, "true")
	// Enforcing over plaintext would only gate an attacker-settable header.
	if _, _, err := optionsFromEnv(nil, nil); err == nil {
		t.Fatal("expected error: enforce requires mTLS")
	}
}

func TestOptionsFromEnv_AllowedCallersBuildsAllowList(t *testing.T) {
	t.Setenv(EnvAllowHeader, "true")
	t.Setenv(EnvAllowedCallers, " api-gateway , payment-orchestrator ")
	opts, _, err := optionsFromEnv(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	al, ok := opts.Policy.(AllowList)
	if !ok {
		t.Fatalf("expected AllowList policy, got %T", opts.Policy)
	}
	if !al.Subjects["api-gateway"] || !al.Subjects["payment-orchestrator"] {
		t.Fatalf("allow-list missing expected subjects: %v", al.Subjects)
	}
	if al.Subjects[""] {
		t.Fatal("allow-list must not contain an empty subject")
	}
}

func TestOptionsFromEnv_ExplicitPolicyWinsOverAllowedCallers(t *testing.T) {
	t.Setenv(EnvAllowedCallers, "api-gateway")
	explicit := AllowList{Subjects: map[string]bool{"only-me": true}}
	opts, _, err := optionsFromEnv(nil, explicit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	al, ok := opts.Policy.(AllowList)
	if !ok || !al.Subjects["only-me"] || al.Subjects["api-gateway"] {
		t.Fatalf("explicit policy argument must win, got %#v", opts.Policy)
	}
}
