// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// HeaderMetadataKey is the gRPC metadata key the API gateway uses to forward the
// authenticated caller identity to downstream services. It matches the header the
// gateway already sets today (payment-orchestrator reads it as "x-caller-identity").
const HeaderMetadataKey = "x-caller-identity"

// Authenticator extracts and verifies the caller identity from the request
// context. It returns a non-nil Identity on success, or an error carrying a gRPC
// status code (typically codes.Unauthenticated) when the caller cannot be
// authenticated.
type Authenticator interface {
	Authenticate(ctx context.Context) (*Identity, error)
}

// MTLSAuthenticator derives the caller identity from the verified peer TLS
// certificate presented over mutual TLS. It relies on the transport having
// already verified the client certificate chain (see ServerTLSConfig, which sets
// tls.RequireAndVerifyClientCert); it does not trust any caller-supplied
// metadata. This is the target authenticator once mTLS is rolled out on every hop.
type MTLSAuthenticator struct{}

// Authenticate implements Authenticator.
func (MTLSAuthenticator) Authenticate(ctx context.Context) (*Identity, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return nil, status.Error(codes.Unauthenticated, "no peer transport authentication information")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "peer connection is not mutually authenticated TLS")
	}
	chains := tlsInfo.State.VerifiedChains
	if len(chains) == 0 || len(chains[0]) == 0 {
		return nil, status.Error(codes.Unauthenticated, "no verified client certificate presented")
	}
	cn := chains[0][0].Subject.CommonName
	if cn == "" {
		return nil, status.Error(codes.Unauthenticated, "client certificate has no common name")
	}
	return &Identity{Subject: cn, Method: "mtls"}, nil
}

// HeaderAuthenticator derives the caller identity from a trusted gRPC metadata
// header. It is a TRANSITIONAL authenticator for the period before mTLS is
// deployed on every hop: it trusts the network boundary (the API gateway sets the
// header, internal traffic is not yet mutually authenticated). It is strictly
// stronger than reading an actor field from the request payload — the payload
// path let any caller name any actor — but it still trusts the transport. Once
// mTLS is in place, switch to MTLSAuthenticator.
type HeaderAuthenticator struct {
	// Key is the metadata key to read; defaults to HeaderMetadataKey when empty.
	Key string
}

// Authenticate implements Authenticator.
func (h HeaderAuthenticator) Authenticate(ctx context.Context) (*Identity, error) {
	key := h.Key
	if key == "" {
		key = HeaderMetadataKey
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no request metadata")
	}
	vals := md.Get(key)
	if len(vals) == 0 || vals[0] == "" {
		return nil, status.Errorf(codes.Unauthenticated, "missing %q identity header", key)
	}
	return &Identity{Subject: vals[0], Method: "header"}, nil
}
