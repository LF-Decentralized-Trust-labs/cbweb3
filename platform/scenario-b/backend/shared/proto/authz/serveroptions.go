// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Environment variables read by ServerOptionsFromEnv:
//
//	GRPC_AUTHZ_ENFORCE   "true"/"1" to reject unauthenticated/unauthorized calls.
//	                     Requires mutual TLS (GRPC_MTLS_*): enforcing over a
//	                     plaintext transport would only gate a caller-settable
//	                     header, a false sense of security, so this combination is
//	                     rejected at startup.
//	GRPC_MTLS_CERT_FILE  server certificate (PEM). When this, KEY and CA are all
//	GRPC_MTLS_KEY_FILE   set, the server enables mutual TLS and derives the caller
//	GRPC_MTLS_CA_FILE    identity from the verified client certificate. Setting only
//	                     one or two of the three is a misconfiguration that would
//	                     silently downgrade to plaintext, so it is rejected.
//	GRPC_AUTHZ_ALLOW_HEADER_IDENTITY
//	                     "true"/"1" to trust the x-caller-identity metadata header as
//	                     the caller identity when mTLS is not configured. This is
//	                     TRANSITIONAL and INSECURE (the header is caller-settable over
//	                     plaintext); it must be a deliberate operator opt-in. Default
//	                     off: without mTLS and without this flag the server runs with
//	                     NO caller authentication and audit actors derive from the
//	                     gateway-validated request payload instead of a spoofable
//	                     transport header.
//	GRPC_AUTHZ_ALLOWED_CALLERS
//	                     Optional comma-separated caller subjects (mTLS CNs or header
//	                     identities). When set and no explicit policy is passed, only
//	                     these subjects are authorized (AllowList); otherwise any
//	                     authenticated caller is allowed.
const (
	EnvEnforce        = "GRPC_AUTHZ_ENFORCE"
	EnvCertFile       = "GRPC_MTLS_CERT_FILE"
	EnvKeyFile        = "GRPC_MTLS_KEY_FILE"
	EnvCAFile         = "GRPC_MTLS_CA_FILE"
	EnvAllowHeader    = "GRPC_AUTHZ_ALLOW_HEADER_IDENTITY"
	EnvAllowedCallers = "GRPC_AUTHZ_ALLOWED_CALLERS"
)

// ServerOptionsFromEnv assembles the gRPC server options (transport credentials
// and authorization interceptors) for a service from environment configuration.
// It is the single call sites use to secure a server:
//
//	opts, err := authz.ServerOptionsFromEnv(logger, policy)
//	if err != nil { ... }
//	grpc.NewServer(opts...)
//
// policy may be nil to accept any authenticated caller (subject to an optional
// GRPC_AUTHZ_ALLOWED_CALLERS allow-list). The interceptors are always installed,
// so no configuration yields an unprotected server. Errors are returned for a
// misconfigured or fail-open security posture (partial mTLS material, or
// enforcement requested without mTLS).
func ServerOptionsFromEnv(logger *slog.Logger, policy Policy) ([]grpc.ServerOption, error) {
	opts, creds, err := optionsFromEnv(logger, policy)
	if err != nil {
		return nil, err
	}

	var serverOpts []grpc.ServerOption
	if creds != nil {
		serverOpts = append(serverOpts, grpc.Creds(creds))
	}
	serverOpts = append(serverOpts,
		grpc.ChainUnaryInterceptor(UnaryServerInterceptor(opts)),
		grpc.ChainStreamInterceptor(StreamServerInterceptor(opts)),
	)
	return serverOpts, nil
}

// optionsFromEnv resolves the interceptor Options and (optional) mTLS transport
// credentials from the environment. It is split out from ServerOptionsFromEnv so
// the security-posture decisions are unit-testable without standing up a server.
func optionsFromEnv(logger *slog.Logger, policy Policy) (Options, credentials.TransportCredentials, error) {
	enforce := boolEnv(os.Getenv(EnvEnforce))
	certFile := os.Getenv(EnvCertFile)
	keyFile := os.Getenv(EnvKeyFile)
	caFile := os.Getenv(EnvCAFile)
	allowHeader := boolEnv(os.Getenv(EnvAllowHeader))

	// A partial mTLS configuration (one or two of the three files) would otherwise
	// silently fall through to plaintext + header identity. Fail closed instead.
	set := 0
	for _, f := range []string{certFile, keyFile, caFile} {
		if f != "" {
			set++
		}
	}
	if set != 0 && set != 3 {
		return Options{}, nil, fmt.Errorf("authz: incomplete mTLS configuration — set all of %s, %s, %s or none", EnvCertFile, EnvKeyFile, EnvCAFile)
	}
	mtlsConfigured := set == 3

	// Enforcing over a non-mTLS transport would only gate a caller-settable header
	// (or nothing at all). Refuse it so enforcement can never be a paper tiger.
	if enforce && !mtlsConfigured {
		return Options{}, nil, fmt.Errorf("authz: %s=true requires mutual TLS — set %s, %s and %s", EnvEnforce, EnvCertFile, EnvKeyFile, EnvCAFile)
	}

	// Default policy: honour an optional operator allow-list; otherwise accept any
	// authenticated caller. An explicit policy argument always wins.
	if policy == nil {
		if subjects := parseAllowedCallers(os.Getenv(EnvAllowedCallers)); len(subjects) > 0 {
			policy = AllowList{Subjects: subjects}
		}
	}

	opts := Options{Policy: policy, Enforce: enforce, Logger: logger}

	switch {
	case mtlsConfigured:
		creds, err := ServerTLSConfig(certFile, keyFile, caFile)
		if err != nil {
			return Options{}, nil, err
		}
		opts.Authenticator = MTLSAuthenticator{}
		if logger != nil {
			logger.Info("grpc authz: mutual TLS enabled", "enforce", enforce)
		}
		return opts, creds, nil

	case allowHeader:
		opts.Authenticator = HeaderAuthenticator{}
		if logger != nil {
			logger.Warn("grpc authz: mTLS not configured — trusting caller-settable identity header over plaintext transport (transitional; set via "+EnvAllowHeader+")",
				"enforce", enforce)
		}
		return opts, nil, nil

	default:
		// Safe default: no caller authentication. Interceptors are still installed
		// (the identity insertion point stays uniform for the mTLS rollout) but
		// establish no identity, so audit actors derive from the gateway-validated
		// request payload rather than a spoofable transport header.
		opts.Anonymous = true
		if logger != nil {
			logger.Warn("grpc authz: mTLS not configured and header identity not opted in — running WITHOUT caller authentication (audit actors from request payload)")
		}
		return opts, nil, nil
	}
}

// parseAllowedCallers parses a comma-separated subject list into an allow-list set.
func parseAllowedCallers(v string) map[string]bool {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	out := map[string]bool{}
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out[s] = true
		}
	}
	return out
}

func boolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// DefaultPolicyFromEnv returns the baseline policy a server should apply to methods
// that carry no per-method restriction: the operator's GRPC_AUTHZ_ALLOWED_CALLERS
// allow-list when one is configured, otherwise "any authenticated caller".
//
// It exists so a service can pass an explicit MethodPolicy without discarding that
// operator setting. ServerOptionsFromEnv only consults the environment when the
// policy argument is nil, so a service that hardens its write methods would
// otherwise silently widen its read methods back to every authenticated peer.
func DefaultPolicyFromEnv() Policy {
	if subjects := parseAllowedCallers(os.Getenv(EnvAllowedCallers)); len(subjects) > 0 {
		return AllowList{Subjects: subjects}
	}
	return AllowAuthenticated{}
}
