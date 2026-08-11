// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// serviceMeshServices are the intra-entity gRPC services that participate in the
// R2-H-8 mutual-TLS mesh. Each gets a leaf certificate whose CN (and SAN) is the
// logical service name — the exact name a client pins via
// authz.ClientDialOptionFromEnv, independent of the dial address, so one cert
// verifies whether the peer is reached by container name (local) or IP (multi-host).
var serviceMeshServices = []string{"compliance", "auth", "api-gateway", "payment-orchestrator"}

// genServiceTLS provisions the per-entity service-mesh PKI into the given named
// volume (mounted read-only at /svc-tls in every backend container): a private,
// entity-local CA (svc-ca.crt) plus one leaf cert+key per service
// (<service>.crt / <service>.key). A service reuses its single leaf for both its
// server and client roles (the leaf carries serverAuth + clientAuth EKUs).
//
// This CA is deliberately separate from the consortium / central-bank CA: it
// exists only to authenticate service-to-service calls WITHIN an entity, so it
// works uniformly for a founding CB, the hub, and a joined commercial bank (which
// has no consortium CA of its own). The CA private key is never persisted — it is
// used in-memory to sign the leaves and then discarded (leaves are long-lived,
// local-dev material). Non-destructive: an existing svc-ca.crt is preserved.
//
// Generating the material is harmless on its own; mutual TLS only activates when a
// service is pointed at these files via GRPC_MTLS_* (gated by GRPC_MTLS_ENABLE in
// the compose templates), so this step is safe to run unconditionally.
func genServiceTLS(ctx context.Context, r exec.CommandRunner, volume string) error {
	if volumeHasFile(ctx, r, volume, "svc-ca.crt") {
		return nil // preserve an existing mesh CA + leaves
	}

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("gen-svc-tls: CA key: %w", err)
	}
	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("gen-svc-tls: CA serial: %w", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: "service-mesh-ca", OrganizationalUnit: []string{"SERVICE_MESH_CA"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("gen-svc-tls: create CA cert: %w", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return fmt.Errorf("gen-svc-tls: parse CA cert: %w", err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	// Emit each service leaf before the CA cert; the CA file's presence is the
	// idempotency marker (checked above), so it must be written last.
	for _, svc := range serviceMeshServices {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return fmt.Errorf("gen-svc-tls: %s key: %w", svc, err)
		}
		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if err != nil {
			return fmt.Errorf("gen-svc-tls: %s serial: %w", svc, err)
		}
		leafTmpl := &x509.Certificate{
			SerialNumber: serial,
			Subject:      pkix.Name{CommonName: svc},
			DNSNames:     []string{svc},
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().AddDate(5, 0, 0),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}
		leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &key.PublicKey, caKey)
		if err != nil {
			return fmt.Errorf("gen-svc-tls: create %s cert: %w", svc, err)
		}
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return fmt.Errorf("gen-svc-tls: marshal %s key: %w", svc, err)
		}
		leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
		// 0644 (not 0600): writeVolumeFile writes as root but the service reads as a
		// different uid; a root-owned 0600 file would be unreadable. Local-dev
		// service-mesh material only (mirrors genCBCA).
		if err := writeVolumeFile(ctx, r, volume, svc+".crt", leafPEM, "0644"); err != nil {
			return err
		}
		if err := writeVolumeFile(ctx, r, volume, svc+".key", keyPEM, "0644"); err != nil {
			return err
		}
	}

	return writeVolumeFile(ctx, r, volume, "svc-ca.crt", caPEM, "0644")
}
