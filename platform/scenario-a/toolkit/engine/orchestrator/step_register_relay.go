// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"path/filepath"
	"time"
)

type registerRelayStep struct {
	spokeID    string
	dataDir    string
	besuRPCURL string
	registrar  RelayRegistrar
	timeout    time.Duration
}

func newRegisterRelayStep(spokeID, dataDir, besuRPCURL string, registrar RelayRegistrar, timeout time.Duration) Step {
	return &registerRelayStep{
		spokeID:    spokeID,
		dataDir:    dataDir,
		besuRPCURL: besuRPCURL,
		registrar:  registrar,
		timeout:    timeout,
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

	info := SpokeInfo{
		SpokeID:     s.spokeID,
		BesuRPCURL:  s.besuRPCURL,
		HTLCAddress: addrs.RegistryContractAddress, // placeholder until HTLC address is tracked separately in DeployedAddrs
	}
	return s.registrar.Register(ctx, info)
}
