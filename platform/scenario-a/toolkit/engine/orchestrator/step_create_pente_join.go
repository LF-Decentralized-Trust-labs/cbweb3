// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// createPenteJoinStep creates the bilateral Pente privacy group between the
// central bank and the joining commercial bank (mode:join US3). Members are the
// two nodes' Paladin identities — parametrized, no hardcoded bank list.
type createPenteJoinStep struct {
	spokeID    string
	bankID     string
	dataDir    string
	paladinURL string // the bank's Paladin RPC (it knows the CB node on-chain)
	timeout    time.Duration
}

func newCreatePenteJoinStep(spokeID, bankID, dataDir, paladinURL string, timeout time.Duration) Step {
	return &createPenteJoinStep{spokeID: spokeID, bankID: bankID, dataDir: dataDir, paladinURL: paladinURL, timeout: timeout}
}

func (s *createPenteJoinStep) Name() string { return StepCreatePenteJoin }

func (s *createPenteJoinStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.PenteContextGroupID != "", nil
}

func (s *createPenteJoinStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	members := []string{
		paladinIdentity(cbNodeName(s.spokeID)),
		paladinIdentity(bankNodeName(s.spokeID, s.bankID)),
	}
	name := fmt.Sprintf("fx-%s-%s", s.spokeID, s.bankID)

	groupID, err := createPenteGroup(ctx, s.paladinURL, name, members)
	if err != nil {
		return fmt.Errorf("create pente group: %w", err)
	}
	if err := addrsAppend(filepath.Join(s.dataDir, ".deployed-addrs.env"), "PENTE_CONTEXT_GROUP_ID", groupID); err != nil {
		return fmt.Errorf("persist PENTE_CONTEXT_GROUP_ID: %w", err)
	}
	return nil
}
