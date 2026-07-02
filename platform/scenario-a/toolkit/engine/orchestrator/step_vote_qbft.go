// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
)

// voteQBFTStep promotes the joining node to a QBFT validator by casting
// qbft_proposeValidatorVote(joinerAddr, true) on each existing validator listed
// in the bundle, then waits for the joiner to appear in the active validator set.
//
// Quorum is computed over the bundle's validator list, which is the validator
// set the founding central bank published at bundle-emission time. For the
// supported found→join flow the founding CB is the spoke's sole validator
// (quorum = 1). If a spoke has grown to several validators, the operator must
// re-emit the bundle with the current validator set (BundleInput.Validators) so
// quorum reflects the live set; a stale list would under-count the real quorum.
type voteQBFTStep struct {
	spokeID    string
	joinerRPC  string
	validators []bundle.ValidatorSpec
	timeout    time.Duration
	interval   time.Duration
	httpClient *http.Client
	logw       io.Writer
}

func newVoteQBFTStep(spokeID, joinerRPC string, validators []bundle.ValidatorSpec, timeout, interval time.Duration, logw io.Writer) Step {
	return &voteQBFTStep{
		spokeID:    spokeID,
		joinerRPC:  joinerRPC,
		validators: validators,
		timeout:    timeout,
		interval:   interval,
		httpClient: sharedJoinHTTPClient,
		logw:       logw,
	}
}

func (s *voteQBFTStep) Name() string { return StepVoteQBFT }

// Check returns true if the joiner is already in the active validator set.
func (s *voteQBFTStep) Check(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	joinerAddr, err := resolveJoinerAddress(ctx, s.httpClient, s.joinerRPC)
	if err != nil {
		return false, nil // node not reachable yet → not done
	}
	set, err := qbftGetValidators(ctx, s.httpClient, s.joinerRPC, "latest")
	if err != nil {
		return false, nil
	}
	return containsAddressFold(set, joinerAddr), nil
}

func (s *voteQBFTStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	n := len(s.validators)
	if n == 0 {
		return fmt.Errorf("vote-qbft: no validators in bundle — cannot reach quorum")
	}
	quorum := n/2 + 1

	// Resolve the joiner's QBFT validator address from its own node info.
	joinerAddr, err := resolveJoinerAddress(ctx, s.httpClient, s.joinerRPC)
	if err != nil {
		return fmt.Errorf("vote-qbft: resolve joiner validator address: %w", err)
	}

	// Cast a vote on every existing validator; count successes.
	votes := 0
	for _, v := range s.validators {
		if err := qbftProposeValidatorVote(ctx, s.httpClient, v.RPCURL, joinerAddr, true); err != nil {
			logDetail(s.logw, s.spokeID, s.Name(), fmt.Sprintf("validator %s vote failed: %v", v.Address, err))
			continue
		}
		votes++
		logDetail(s.logw, s.spokeID, s.Name(), fmt.Sprintf("validator %s voted to add %s (%d/%d)", v.Address, joinerAddr, votes, quorum))
	}
	if votes < quorum {
		return fmt.Errorf("vote-qbft: collected %d/%d votes — quorum not reached", votes, quorum)
	}

	// Poll for activation at the next epoch boundary.
	for {
		set, err := qbftGetValidators(ctx, s.httpClient, s.joinerRPC, "latest")
		if err == nil && containsAddressFold(set, joinerAddr) {
			logDetail(s.logw, s.spokeID, s.Name(), fmt.Sprintf("joiner %s activated in validator set (size=%d)", joinerAddr, len(set)))
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("vote-qbft: timed out after %s waiting for joiner activation", s.timeout)
		case <-time.After(s.interval):
		}
	}
}
