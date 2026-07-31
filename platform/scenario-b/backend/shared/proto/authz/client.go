// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientDialOptionFromEnv returns the transport-credentials dial option a gRPC
// client should use to reach an internal peer service. When the GRPC_MTLS_* env
// vars are configured it presents this service's certificate and verifies the
// peer against the shared CA (mutual TLS); otherwise it falls back to insecure
// credentials so existing plaintext deployments keep working during the rollout.
//
// A service reuses the SAME certificate for its server and client roles (the leaf
// carries both serverAuth and clientAuth EKUs), so the client reads the same
// GRPC_MTLS_CERT_FILE / KEY / CA that ServerOptionsFromEnv reads.
//
// serverName is the logical name the peer certificate must present in its
// SAN/CN (e.g. "compliance", "auth", "payment-orchestrator"). It is pinned
// independently of the dial address so a single certificate verifies whether the
// peer is reached by container name (local compose) or by IP (multi-host).
// A partial mTLS configuration is rejected rather than silently downgraded.
func ClientDialOptionFromEnv(serverName string) (grpc.DialOption, error) {
	certFile := os.Getenv(EnvCertFile)
	keyFile := os.Getenv(EnvKeyFile)
	caFile := os.Getenv(EnvCAFile)

	set := 0
	for _, f := range []string{certFile, keyFile, caFile} {
		if f != "" {
			set++
		}
	}
	switch set {
	case 0:
		// Transitional default: no client mTLS material configured → plaintext.
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	case 3:
		if serverName == "" {
			return nil, fmt.Errorf("authz: client mTLS requires a non-empty serverName for peer verification")
		}
		creds, err := ClientTLSConfig(certFile, keyFile, caFile, serverName)
		if err != nil {
			return nil, err
		}
		return grpc.WithTransportCredentials(creds), nil
	default:
		return nil, fmt.Errorf("authz: incomplete client mTLS configuration — set all of %s, %s, %s or none", EnvCertFile, EnvKeyFile, EnvCAFile)
	}
}
