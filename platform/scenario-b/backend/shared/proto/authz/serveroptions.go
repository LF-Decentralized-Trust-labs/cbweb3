// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"log/slog"
	"os"
	"strings"

	"google.golang.org/grpc"
)

// Environment variables read by ServerOptionsFromEnv:
//
//	GRPC_AUTHZ_ENFORCE   "true"/"1" to reject unauthenticated/unauthorized calls.
//	                     Any other value (or unset) selects audit mode, which
//	                     never rejects — safe default during rollout.
//	GRPC_MTLS_CERT_FILE  server certificate (PEM). When this, KEY and CA are all
//	GRPC_MTLS_KEY_FILE   set, the server enables mutual TLS and derives the caller
//	GRPC_MTLS_CA_FILE    identity from the verified client certificate. Otherwise
//	                     the caller identity comes from the trusted metadata header
//	                     (HeaderAuthenticator, transitional).
const (
	EnvEnforce  = "GRPC_AUTHZ_ENFORCE"
	EnvCertFile = "GRPC_MTLS_CERT_FILE"
	EnvKeyFile  = "GRPC_MTLS_KEY_FILE"
	EnvCAFile   = "GRPC_MTLS_CA_FILE"
)

// ServerOptionsFromEnv assembles the gRPC server options (transport credentials
// and authorization interceptors) for a service from environment configuration.
// It is the single call sites use to secure a server:
//
//	opts, err := authz.ServerOptionsFromEnv(logger, policy)
//	if err != nil { ... }
//	grpc.NewServer(opts...)
//
// policy may be nil to accept any authenticated caller (AllowAuthenticated). When
// mTLS env vars are absent the server stays plaintext (unchanged transport) and
// falls back to header-based identity so existing deployments keep working; the
// interceptors are always installed. Errors are only returned for misconfigured
// TLS material.
func ServerOptionsFromEnv(logger *slog.Logger, policy Policy) ([]grpc.ServerOption, error) {
	enforce := boolEnv(os.Getenv(EnvEnforce))
	certFile := os.Getenv(EnvCertFile)
	keyFile := os.Getenv(EnvKeyFile)
	caFile := os.Getenv(EnvCAFile)

	opts := Options{Policy: policy, Enforce: enforce, Logger: logger}

	var serverOpts []grpc.ServerOption
	if certFile != "" && keyFile != "" && caFile != "" {
		creds, err := ServerTLSConfig(certFile, keyFile, caFile)
		if err != nil {
			return nil, err
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		opts.Authenticator = MTLSAuthenticator{}
		if logger != nil {
			logger.Info("grpc authz: mutual TLS enabled", "enforce", enforce)
		}
	} else {
		opts.Authenticator = HeaderAuthenticator{}
		if logger != nil {
			logger.Warn("grpc authz: mTLS not configured — using header identity over plaintext transport (transitional)",
				"enforce", enforce)
		}
	}

	serverOpts = append(serverOpts,
		grpc.ChainUnaryInterceptor(UnaryServerInterceptor(opts)),
		grpc.ChainStreamInterceptor(StreamServerInterceptor(opts)),
	)
	return serverOpts, nil
}

func boolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
