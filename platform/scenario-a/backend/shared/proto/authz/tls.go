// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// ServerTLSConfig builds mutual-TLS transport credentials for a gRPC server. It
// presents certFile/keyFile and requires and verifies client certificates against
// the CA bundle in caFile (the consortium / central-bank CA under
// backend/config/pki). This is the server-side half of mTLS; clients must be
// updated to present their own certificate and trust the same CA — see the
// pending client-side rollout documented in the PR.
func ServerTLSConfig(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server keypair (%s / %s): %w", certFile, keyFile, err)
	}
	pool, err := caPool(caFile)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}), nil
}

// ClientTLSConfig builds mutual-TLS transport credentials for a gRPC client
// dialing a peer service. serverName must match the peer certificate's subject
// (SAN/CN). Provided for the pending client-side mTLS rollout so callers can
// replace insecure.NewCredentials().
func ClientTLSConfig(certFile, keyFile, caFile, serverName string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load client keypair (%s / %s): %w", certFile, keyFile, err)
	}
	pool, err := caPool(caFile)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}), nil
}

func caPool(caFile string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caFile) //nolint:gosec // caFile is an operator-provided PKI path
	if err != nil {
		return nil, fmt.Errorf("read CA bundle %s: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no certificates parsed from CA bundle %s", caFile)
	}
	return pool, nil
}
