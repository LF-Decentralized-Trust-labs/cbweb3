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
	"net"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// addTransportSAN adds host to a Paladin transport cert's SAN in the correct
// field so a dns:///<host>:9000 dial validates: an IP literal goes in IPAddresses
// (Go verifies an IP ServerName against the IP SAN), a DNS name in DNSNames. It is
// a no-op for the empty string or a duplicate already present. Used to carry a
// node's routable advertisedHost (see paladinDialHost) so cross-VM peers can dial
// the on-chain endpoint directly without a per-peer extra_hosts entry.
func addTransportSAN(tmpl *x509.Certificate, host string) {
	if host == "" {
		return
	}
	if ip := net.ParseIP(host); ip != nil {
		for _, existing := range tmpl.IPAddresses {
			if existing.Equal(ip) {
				return
			}
		}
		tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		return
	}
	for _, d := range tmpl.DNSNames {
		if d == host {
			return
		}
	}
	tmpl.DNSNames = append(tmpl.DNSNames, host)
}

// genTLSStep generates the central bank's self-signed TLS cert/key and seeds
// them directly into named Docker volumes — no host filesystem involved
// (deviation from the original SPOKE_DATA_DIR bind-mount design; see the
// addendum in specs/026-tk4-compose-central-bank/plan.md).
type genTLSStep struct {
	spokeID        string
	advertisedHost string // CB routable host; when routable, added to the transport cert SAN
	keyProvider    keyprovider.KeyProvider
}

func newGenTLSStep(spokeID, advertisedHost string, kp keyprovider.KeyProvider) Step {
	return &genTLSStep{spokeID: spokeID, advertisedHost: advertisedHost, keyProvider: kp}
}

func (s *genTLSStep) Name() string { return StepGenTLS }

// tlsVolume holds central-bank.{crt,key}, read by EmitBundle (trust anchor) and
// mounted read-only into the CB's backend containers as ENTITY_PKI_DIR.
func (s *genTLSStep) tlsVolume() string { return s.spokeID + "_cb_tls" }

// paladinConfigVolume holds config.yaml (rendered by render-configs) plus
// tls.{crt,key}, mounted at /etc/paladin by paladin-compose.yaml.
func (s *genTLSStep) paladinConfigVolume() string { return s.spokeID + "_cb_paladin_config" }

func (s *genTLSStep) Check(ctx context.Context) (bool, error) {
	return volumeFileExists(ctx, s.tlsVolume(), "central-bank.crt")
}

func (s *genTLSStep) Run(ctx context.Context) error {
	// Generate an ephemeral ECDSA P-256 key for the TLS certificate.
	// The CB blockchain key (for signing transactions) is held by KeyProvider.
	// The TLS key is a transport-layer key generated fresh here.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate TLS key: %w", err)
	}

	// Build a self-signed X.509 certificate for the central-bank node.
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}
	// found generates ONLY the central bank's Paladin cert. Each commercial bank
	// generates its own cert dynamically at join time (mode:join), per the
	// dynamic-registration architecture (concat.md / spk-02 join-paladin.sh) —
	// the found step must not bake in a fixed set of bank nodes.
	// The Paladin gRPC transport authenticates a peer by matching the cert's TLS
	// identity against the EXPECTED NODE NAME from the registry (PD030011), not the
	// dial hostname. The node is registered as cbNodeName (e.g. "spoke-brl-cb"), so
	// the cert CN must be that node name — using the container hostname
	// ("paladin-spoke-brl-cb") makes the mutual-TLS handshake fail. The hostname is
	// kept in the SAN so the dns:/// dial still validates.
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         cbNodeName(s.spokeID),
			OrganizationalUnit: []string{"ROLE_CENTRAL_BANK"},
			Organization:       []string{s.spokeID},
		},
		DNSNames: []string{
			cbNodeName(s.spokeID),
			cbGrpcHostname(s.spokeID),
			"localhost",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	// Cross-VM: peers dial this node at its routable advertisedHost (register-nodes
	// publishes dns:///<advertisedHost>:9000 via paladinDialHost), so that host must
	// be in the SAN for the mutual-TLS handshake to validate. Single-host keeps only
	// the container-name SAN above.
	if isRoutableHost(s.advertisedHost) {
		addTransportSAN(tmpl, s.advertisedHost)
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshal TLS key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Seed the cert/key into the volumes the engine and the CB Paladin compose
	// expect them in. Legacy tls/central-bank.{crt,key} is kept (as a volume); the
	// CB Paladin node config volume gets tls.{crt,key} (mounted into /etc/paladin;
	// the config references /etc/paladin/tls.{crt,key}).
	//
	// Key files are 0644, not the conventional 0600: writeVolumeFile always writes
	// as root (the throwaway seed container), but the readers run as a different,
	// non-root uid (Paladin: PALADIN_UID, default 1000, seen 1001 in one deployment;
	// compliance reads central-bank.key per step_render_cb_env.go's CAKeyFile). A
	// root-owned 0600 file is unreadable by any of them — this bit a live join
	// (PD020402: open /etc/paladin/tls.key: permission denied) because the old
	// bind-mount design happened to make this work only when the host operator's
	// own uid coincided with the container's uid. 0644 is safe here: the boundary
	// is now "which containers mount this volume", not "which host users can read
	// this path" (SPOKE_DATA_DIR's original threat model).
	targets := []struct{ volume, certName, keyName string }{
		{s.tlsVolume(), "central-bank.crt", "central-bank.key"},
		{s.paladinConfigVolume(), "tls.crt", "tls.key"},
	}
	for _, t := range targets {
		if err := writeVolumeFile(ctx, t.volume, t.certName, certPEM, "0644"); err != nil {
			return fmt.Errorf("write cert %s to volume %s: %w", t.certName, t.volume, err)
		}
		if err := writeVolumeFile(ctx, t.volume, t.keyName, keyPEM, "0644"); err != nil {
			return fmt.Errorf("write key %s to volume %s: %w", t.keyName, t.volume, err)
		}
	}

	return nil
}
