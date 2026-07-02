// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// registerNodesStep registers the central bank's Paladin node identity on-chain
// (mode:found). It uses native Go logic parametrized by the spoke id — it does
// NOT invoke the reference go-test scripts, which hardcode the spoke-a/spoke-b
// topology and would register the wrong nodes for an arbitrary spoke.
//
// found is CB-only: exactly one node (<spoke-id>-cb) is registered here.
// Commercial-bank Paladin nodes are registered dynamically at join time (TK-9 /
// feature 033 US2).
type registerNodesStep struct {
	spokeID     string
	dataDir     string
	besuRPCURL  string
	keyProvider kp.KeyProvider
	timeout     time.Duration
}

func newRegisterNodesStep(spokeID, dataDir, besuRPCURL string, keyProvider kp.KeyProvider, timeout time.Duration) Step {
	return &registerNodesStep{spokeID: spokeID, dataDir: dataDir, besuRPCURL: besuRPCURL, keyProvider: keyProvider, timeout: timeout}
}

func (s *registerNodesStep) Name() string { return StepRegisterNodes }

// Check uses the provisioning state file (no idempotent on-chain query is exposed
// for the Paladin node registry).
func (s *registerNodesStep) Check(_ context.Context) (bool, error) {
	state, err := LoadState(s.dataDir)
	if err != nil {
		return false, err
	}
	return statusFor(state, StepRegisterNodes) == "done", nil
}

func (s *registerNodesStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Registry address produced by deploy-contracts.
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return fmt.Errorf("read deployed-addrs: %w", err)
	}
	if addrs.RegistryContractAddress == "" {
		return fmt.Errorf("REGISTRY_CONTRACT_ADDRESS missing in .deployed-addrs.env")
	}

	// CB Paladin node TLS cert written by gen-tls.
	certPath := filepath.Join(s.dataDir, "paladin", "central-bank", "tls.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("read CB Paladin cert %s: %w", certPath, err)
	}

	return registerPaladinNode(ctx, s.besuRPCURL, paladinNodeRegistration{
		registry:     common.HexToAddress(addrs.RegistryContractAddress),
		nodeName:     cbNodeName(s.spokeID),
		grpcHostname: cbGrpcHostname(s.spokeID),
		certPEM:      certPEM,
		provider:     s.keyProvider,
		signerKeyID:  kp.LocalOperatorKeyID,
	})
}
