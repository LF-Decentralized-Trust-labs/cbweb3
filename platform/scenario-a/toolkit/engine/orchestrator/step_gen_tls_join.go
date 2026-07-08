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
)

// genTLSJoinStep generates the commercial bank's Paladin TLS transport cert
// (mode:join). Distinct from the bank's blockchain identity (gen-csr/request-cert):
// this is a self-signed cert for the bank's Paladin gRPC transport, CN/SAN derived
// from the bank id. Each joining bank generates its own cert dynamically — there is
// no shared cert and no fixed bank list (concat.md / spk-02 generate-paladin-certs).
//
// Seeded directly into the named volume ${spokeID}_${bankID}_paladin_config — no
// host filesystem involved (deviation from the original SPOKE_DATA_DIR bind-mount
// design; mirrors the central-bank found-mode equivalent, see the addendum in
// specs/026-tk4-compose-central-bank/plan.md).
type genTLSJoinStep struct {
	spokeID string
	bankID  string
}

func newGenTLSJoinStep(spokeID, bankID string) Step {
	return &genTLSJoinStep{spokeID: spokeID, bankID: bankID}
}

func (s *genTLSJoinStep) Name() string { return StepGenTLSJoin }

// paladinConfigVolume mirrors genTLSStep's naming for the CB
// (${spokeID}_cb_paladin_config): here scoped per bank.
func (s *genTLSJoinStep) paladinConfigVolume() string {
	return s.spokeID + "_" + s.bankID + "_paladin_config"
}

func (s *genTLSJoinStep) Check(ctx context.Context) (bool, error) {
	return volumeFileExists(ctx, s.paladinConfigVolume(), "tls.crt")
}

func (s *genTLSJoinStep) Run(ctx context.Context) error {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate TLS key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}
	// The Paladin gRPC transport authenticates a peer by matching the cert's TLS
	// identity against the EXPECTED NODE NAME from the registry (PD030011), not the
	// dial hostname. The bank node is registered as bankNodeName (e.g.
	// "spoke-brl-bank-itau"), so the cert CN must be that node name — using the
	// container hostname ("paladin-spoke-brl-bank-itau") makes the mutual-TLS
	// handshake fail. The hostname is kept in the SAN so the dns:/// dial validates.
	node := bankNodeName(s.spokeID, s.bankID)
	host := bankGrpcHostname(s.spokeID, s.bankID)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         node,
			OrganizationalUnit: []string{"ROLE_COMMERCIAL_BANK"},
			Organization:       []string{s.spokeID},
		},
		DNSNames:              []string{node, host, "localhost"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
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

	// 0644, not the conventional 0600, for the same reason as genTLSStep (CB):
	// writeVolumeFile always writes as root, but Paladin reads as a different,
	// non-root uid — a root-owned 0600 key is unreadable by it (this bit a live
	// join: PD020402 open /etc/paladin/tls.key: permission denied).
	if err := writeVolumeFile(ctx, s.paladinConfigVolume(), "tls.crt", certPEM, "0644"); err != nil {
		return fmt.Errorf("write tls.crt to volume %s: %w", s.paladinConfigVolume(), err)
	}
	if err := writeVolumeFile(ctx, s.paladinConfigVolume(), "tls.key", keyPEM, "0644"); err != nil {
		return fmt.Errorf("write tls.key to volume %s: %w", s.paladinConfigVolume(), err)
	}
	return nil
}
