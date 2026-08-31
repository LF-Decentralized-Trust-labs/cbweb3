// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// waitSyncStep polls until the joining node is demonstrably following the spoke:
// it must have at least one connected P2P peer (net_peerCount >= 1) AND a block
// height >= target. The peer check rules out a misconfigured singleton node that
// produces its own isolated chain and would otherwise satisfy a height-only
// check. Idempotency is driven by persisted state, so Check always returns false
// (the orchestrator skips it when state is "done").
type waitSyncStep struct {
	besuRPCURL string
	target     uint64 // minimum block height that signals "synced"; 1 means "any block > 0"
	timeout    time.Duration
	interval   time.Duration
	httpClient *http.Client
	logw       io.Writer
}

func newWaitSyncStep(besuRPCURL string, target uint64, timeout, interval time.Duration, logw io.Writer) Step {
	if target == 0 {
		target = 1
	}
	return &waitSyncStep{
		besuRPCURL: besuRPCURL,
		target:     target,
		timeout:    timeout,
		interval:   interval,
		httpClient: sharedJoinHTTPClient,
		logw:       logw,
	}
}

func (s *waitSyncStep) Name() string { return StepWaitSync }

// Check is intentionally not idempotent by side-effect inspection; the
// orchestrator's persisted state ("done") short-circuits re-runs.
func (s *waitSyncStep) Check(_ context.Context) (bool, error) {
	return false, nil
}

func (s *waitSyncStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	for {
		peers, perr := netPeerCount(ctx, s.httpClient, s.besuRPCURL)
		height, herr := ethBlockNumber(ctx, s.httpClient, s.besuRPCURL)
		if perr == nil && herr == nil && peers >= 1 && height >= s.target {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait-sync: timed out after %s waiting for peers>=1 and block height>=%d (last: peers=%d, height=%d)", s.timeout, s.target, peers, height)
		case <-time.After(s.interval):
		}
	}
}
