// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// registerRelayStep registers the spoke with the Cacti relay so the relay can (a) watch the
// spoke's Besu HTLC events and (b) poll the spoke coordinator (central bank) for the aggregate
// of its banks' on-chain FX agreements, and drive the destination leg via the CB
// payment-orchestrator. It emits the full six-field SpokeInfo; the relay creates/reconciles a
// watcher from the registry dynamically (feature 035).
type registerRelayStep struct {
	spokeID        string
	dataDir        string
	besuRPCURL     string
	besuWSURL      string
	grpcEndpoint   string
	internalApiURL string
	registrar      RelayRegistrar
	timeout        time.Duration
}

func newRegisterRelayStep(spokeID, dataDir, besuRPCURL, besuWSURL, grpcEndpoint, internalApiURL string, registrar RelayRegistrar, timeout time.Duration) Step {
	return &registerRelayStep{
		spokeID:        spokeID,
		dataDir:        dataDir,
		besuRPCURL:     besuRPCURL,
		besuWSURL:      besuWSURL,
		grpcEndpoint:   grpcEndpoint,
		internalApiURL: internalApiURL,
		registrar:      registrar,
		timeout:        timeout,
	}
}

func (s *registerRelayStep) Name() string { return StepRegisterRelay }

func (s *registerRelayStep) Check(ctx context.Context) (bool, error) {
	ok, err := s.registrar.IsRegistered(ctx, s.spokeID)
	if err != nil {
		// Relay unavailable → treat as not registered (will retry).
		return false, nil
	}
	return ok, nil
}

func (s *registerRelayStep) Run(ctx context.Context) error {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return err
	}
	if addrs.HTLCAddress == "" {
		return fmt.Errorf("HTLC_ADDRESS missing — deploy-htlc must run before register-relay")
	}

	info := SpokeInfo{
		SpokeID:        s.spokeID,
		BesuRPCURL:     s.besuRPCURL,
		BesuWSURL:      s.besuWSURL,
		HTLCAddress:    addrs.HTLCAddress,
		GRPCEndpoint:   s.grpcEndpoint,
		InternalApiURL: s.internalApiURL,
	}
	return s.registrar.Register(ctx, info)
}
