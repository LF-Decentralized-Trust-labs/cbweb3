// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// registerPaladinNodeStep registers the joining commercial bank's Paladin node
// identity on-chain (mode:join), using the same native primitive as the found CB
// node — parametrized by spoke id and bank id, no hardcoded bank list. Mirrors
// spk-02 register-paladin-nodes for the joiner.
type registerPaladinNodeStep struct {
	spokeID        string
	bankID         string
	dataDir        string
	besuRPCURL     string
	registryAddr   string
	advertisedHost string // bank routable host; when routable, published as the transport endpoint
	keyProvider    kp.KeyProvider
	timeout        time.Duration
}

func newRegisterPaladinNodeStep(spokeID, bankID, dataDir, besuRPCURL, registryAddr, advertisedHost string, keyProvider kp.KeyProvider, timeout time.Duration) Step {
	return &registerPaladinNodeStep{
		spokeID:        spokeID,
		bankID:         bankID,
		dataDir:        dataDir,
		besuRPCURL:     besuRPCURL,
		registryAddr:   registryAddr,
		advertisedHost: advertisedHost,
		keyProvider:    keyProvider,
		timeout:        timeout,
	}
}

func (s *registerPaladinNodeStep) Name() string { return StepRegisterPaladinNode }

// paladinTLSFingerprintFile records the sha256 of the transport cert last
// published on-chain. gen-tls-join may regenerate the volume cert on a later
// run; without this drift check, Check would stay "done" while Paladin presents
// a cert the CB no longer trusts → tls: bad certificate on create-pente-context.
func (s *registerPaladinNodeStep) paladinTLSFingerprintFile() string {
	return filepath.Join(s.dataDir, ".paladin-tls.sha256")
}

func (s *registerPaladinNodeStep) paladinConfigVolume() string {
	return s.spokeID + "_" + s.bankID + "_paladin_config"
}

// Check is done only when state says so AND the volume cert still matches the
// fingerprint published on the last successful Run.
func (s *registerPaladinNodeStep) Check(ctx context.Context) (bool, error) {
	state, err := LoadState(s.dataDir)
	if err != nil {
		return false, err
	}
	if statusFor(state, StepRegisterPaladinNode) != "done" {
		return false, nil
	}
	certPEM, err := readVolumeFile(ctx, s.paladinConfigVolume(), "tls.crt")
	if err != nil {
		// Volume missing → must re-run after gen-tls-join recreates it.
		return false, nil
	}
	stored, err := os.ReadFile(s.paladinTLSFingerprintFile())
	if err != nil {
		// Pre-drift-check runs: force one republish so fingerprint is recorded.
		return false, nil
	}
	sum := sha256.Sum256(certPEM)
	return hex.EncodeToString(sum[:]) == string(stored), nil
}

func (s *registerPaladinNodeStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if s.registryAddr == "" {
		return fmt.Errorf("bundle registry address is empty")
	}
	// Written by gen-tls-join into the named volume — no host filesystem
	// involved (deviation from the original SPOKE_DATA_DIR bind-mount design;
	// see specs/032-commercial-bank-join/research.md addendum).
	certPEM, err := readVolumeFile(ctx, s.paladinConfigVolume(), "tls.crt")
	if err != nil {
		return fmt.Errorf("read bank Paladin cert from volume %s: %w", s.paladinConfigVolume(), err)
	}

	if err := registerPaladinNode(ctx, s.besuRPCURL, paladinNodeRegistration{
		registry:     common.HexToAddress(s.registryAddr),
		nodeName:     bankNodeName(s.spokeID, s.bankID),
		grpcHostname: paladinDialHost(s.advertisedHost, bankGrpcHostname(s.spokeID, s.bankID)),
		certPEM:      certPEM,
		provider:     s.keyProvider,
		signerKeyID:  kp.LocalOperatorKeyID,
	}); err != nil {
		return err
	}
	sum := sha256.Sum256(certPEM)
	if err := os.WriteFile(s.paladinTLSFingerprintFile(), []byte(hex.EncodeToString(sum[:])), 0o644); err != nil {
		return fmt.Errorf("persist paladin TLS fingerprint: %w", err)
	}
	return nil
}
